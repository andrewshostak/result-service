package match

import (
	"context"
	"errors"
	"fmt"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/internal/app/models"
)

type ResultCheckerService struct {
	config                    config.ResultCheck
	matchRepository           MatchRepository
	externalMatchRepository   ExternalMatchRepository
	subscriptionRepository    SubscriptionRepository
	checkResultTaskRepository CheckResultTaskRepository
	externalAPIClient         ExternalAPIClient
	taskClient                TaskClient
	logger                    Logger
}

func NewResultCheckerService(
	config config.ResultCheck,
	matchRepository MatchRepository,
	externalMatchRepository ExternalMatchRepository,
	subscriptionRepository SubscriptionRepository,
	checkResultTaskRepository CheckResultTaskRepository,
	taskClient TaskClient,
	externalAPIClient ExternalAPIClient,
	logger Logger,
) *ResultCheckerService {
	return &ResultCheckerService{
		config:                    config,
		matchRepository:           matchRepository,
		externalMatchRepository:   externalMatchRepository,
		subscriptionRepository:    subscriptionRepository,
		checkResultTaskRepository: checkResultTaskRepository,
		taskClient:                taskClient,
		externalAPIClient:         externalAPIClient,
		logger:                    logger,
	}
}

func (s *ResultCheckerService) CheckResult(ctx context.Context, matchID uint) error {
	match, err := s.matchRepository.One(ctx, models.Match{ID: matchID})
	if err != nil {
		return fmt.Errorf("failed to get match by id: %w", err)
	}

	if !s.isScheduled(match) {
		s.logger.Error().Uint("match_id", matchID).Msg(fmt.Sprintf("expected result status to be %s, actual result status is %s", models.Scheduled, match.ResultStatus))
		return nil
	}

	if match.ExternalMatch == nil {
		s.logger.Error().Uint("match_id", matchID).Msg("match relation external match does not exist")
		return errors.New("match relation external match does not exist")
	}

	leagues, err := s.externalAPIClient.GetMatchesByDate(ctx, match.StartsAt)
	if err != nil {
		s.logger.Error().Uint("match_id", matchID).Err(err)
		if errUpdate := s.updateMatchResultStatus(ctx, match.ID, models.APIError); errUpdate != nil {
			s.logger.Error().Uint("match_id", matchID).Err(errUpdate)
		}

		return fmt.Errorf("failed to get matches from external api: %w", err)
	}

	externalAPIMatch, err := s.findExternalMatch(match.ExternalMatch.ID, leagues)
	if err != nil {
		return fmt.Errorf("external match with id %d is not found: %w", match.ExternalMatch.ID, err)
	}

	_, err = s.externalMatchRepository.Save(ctx, &match.ExternalMatch.ID, externalAPIMatch.ToExternalMatch(match.ID))
	if err != nil {
		return fmt.Errorf("failed to update external match: %w", err)
	}

	switch externalAPIMatch.Status {
	case models.StatusMatchInProgress:
		return s.handleInPlayMatch(ctx, *match)
	case models.StatusMatchFinished:
		return s.handleFinishedMatch(ctx, match.ID)
	// if we receive here any other status - that is not expected, we should cancel the result check.
	default:
		return s.handleMatchWithUnexpectedStatus(ctx, matchID, externalAPIMatch.Status)
	}
}

func (s *ResultCheckerService) findExternalMatch(externalID uint, leagues []models.ExternalAPILeague) (*models.ExternalAPIMatch, error) {
	for _, matches := range leagues {
		for _, match := range matches.Matches {
			if match.ID == int(externalID) {
				return &match, nil
			}
		}
	}

	return nil, errors.New("match not found")
}

func (s *ResultCheckerService) handleMatchWithUnexpectedStatus(ctx context.Context, matchID uint, externalMatchStatus models.ExternalMatchStatus) error {
	s.logger.Error().Uint("match_id", matchID).Msgf("result check cancelled: external match status is %s", externalMatchStatus)

	if err := s.updateMatchResultStatus(ctx, matchID, models.Cancelled); err != nil {
		return fmt.Errorf("failed to update result status of match: %w", err)
	}

	return nil
}

func (s *ResultCheckerService) handleInPlayMatch(ctx context.Context, match models.Match) error {
	s.logger.Debug().Uint("match_id", match.ID).Msg("match is in play, re-scheduling result check task")

	if match.CheckResultTask == nil {
		return errors.New("match relation result check task doesn't exist")
	}

	scheduleAt := match.StartsAt.Add(s.config.FirstAttemptDelay)
	for i := uint(0); i < match.CheckResultTask.AttemptNumber; i++ {
		scheduleAt = scheduleAt.Add(s.config.Interval)
	}

	attemptNumber := match.CheckResultTask.AttemptNumber + 1

	task, err := s.taskClient.ScheduleResultCheck(ctx, match.ID, attemptNumber, scheduleAt)
	if err != nil {
		s.logger.Error().Uint("match_id", match.ID).Uint("attempt_number", attemptNumber).Time("schedule_at", scheduleAt).Err(err).Msg("failed to re-schedule check result task")
		if errUpdate := s.updateMatchResultStatus(ctx, match.ID, models.SchedulingError); errUpdate != nil {
			s.logger.Error().Uint("match_id", match.ID).Err(errUpdate)
		}

		return fmt.Errorf("failed to re-schedule result check task: %w", err)
	}

	if _, err := s.checkResultTaskRepository.Save(ctx, models.CheckResultTask{
		MatchID:       match.ID,
		Name:          task.Name,
		AttemptNumber: attemptNumber,
		ExecuteAt:     task.ExecuteAt,
	}); err != nil {
		return fmt.Errorf("failed to update result check task: %w", err)
	}

	return nil
}

func (s *ResultCheckerService) handleFinishedMatch(ctx context.Context, matchID uint) error {
	s.logger.Debug().Uint("match_id", matchID).Msg("match is finished, scheduling subscribers notifications")

	subscriptions, err := s.subscriptionRepository.ListByMatchAndStatus(ctx, matchID, models.PendingSub)
	if err != nil {
		return fmt.Errorf("failed to get subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		s.logger.Error().Uint("match_id", matchID).Msg("no pending subscriptions found for the match")
	}

	for _, subscription := range subscriptions {
		err := s.taskClient.ScheduleSubscriberNotification(ctx, subscription.ID)
		if err != nil && !errors.As(err, &errs.ResourceAlreadyExistsError{}) {
			s.logger.Error().Uint("subscription_id", subscription.ID).Err(err).Msg("failed to schedule subscriber notification task")
			errUpdate := s.subscriptionRepository.Update(ctx, subscription.ID, models.Subscription{Status: models.SchedulingErrorSub, SubscriberError: nil})
			if errUpdate != nil {
				s.logger.Error().Err(errUpdate).Uint("subscription_id", subscription.ID).Msg(fmt.Sprintf("failed to update subscription status to: %s", string(models.SchedulingErrorSub)))
			}

			return fmt.Errorf("failed to schedule subscriber notification: %w", err)
		}
	}

	if err := s.updateMatchResultStatus(ctx, matchID, models.Received); err != nil {
		return fmt.Errorf("failed to handle finished match: %w", err)
	}

	return nil
}

func (s *ResultCheckerService) updateMatchResultStatus(ctx context.Context, matchID uint, status models.ResultStatus) error {
	if _, errUpdate := s.matchRepository.Update(ctx, matchID, status); errUpdate != nil {
		return fmt.Errorf("failed to update result status to %s: %w", status, errUpdate)
	}

	return nil
}

func (s *ResultCheckerService) isScheduled(match *models.Match) bool {
	return match != nil && match.ResultStatus == models.Scheduled
}
