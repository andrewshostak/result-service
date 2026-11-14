package match

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/app/models"
)

const fotmobProvider = "fotmob"

type ResultCheckerService struct {
	config                    config.ResultCheck
	matchRepository           MatchRepository
	providerMatchRepository   ProviderMatchRepository
	subscriptionRepository    SubscriptionRepository
	checkResultTaskRepository CheckResultTaskRepository
	providers                 []ResultProvider
	taskClient                TaskClient
	logger                    Logger
}

func NewResultCheckerService(
	config config.ResultCheck,
	matchRepository MatchRepository,
	providerMatchRepository ProviderMatchRepository,
	subscriptionRepository SubscriptionRepository,
	checkResultTaskRepository CheckResultTaskRepository,
	taskClient TaskClient,
	providers []ResultProvider,
	logger Logger,
) *ResultCheckerService {
	return &ResultCheckerService{
		config:                    config,
		matchRepository:           matchRepository,
		providerMatchRepository:   providerMatchRepository,
		subscriptionRepository:    subscriptionRepository,
		checkResultTaskRepository: checkResultTaskRepository,
		taskClient:                taskClient,
		providers:                 providers,
		logger:                    logger,
	}
}

func (s *ResultCheckerService) CheckProviderMatchResult(ctx context.Context, providerMatchID uint) error {
	providerMatch, err := s.providerMatchRepository.FindByID(ctx, providerMatchID)
	if err != nil {
		return fmt.Errorf("failed to get provider match by id: %w", err)
	}

	match, err := s.matchRepository.One(ctx, models.Match{ID: providerMatch.MatchID})
	if err != nil {
		return fmt.Errorf("failed to get match by id: %w", err)
	}

	if !s.canCheckResult(match) {
		s.logger.Error().Uint("match_id", match.ID).Uint("provider_match_id", providerMatchID).Msg(fmt.Sprintf("result check skipped: unexpected result status %s", match.ResultStatus))
		return nil
	}

	matches, err := s.getMatchesFromProvider(ctx, providerMatch.Provider, providerMatch.StartsAt)
	if err != nil {
		s.logger.Error().Uint("match_id", match.ID).Uint("provider_match_id", providerMatchID).Err(err).Msg("failed to get matches from provider api")
		if _, errUpdate := s.matchRepository.Update(ctx, match.ID, models.Match{ResultStatus: models.APIError}); errUpdate != nil {
			s.logger.Error().Uint("match_id", match.ID).Err(errUpdate).Msg("failed to update match status to api_error")
		}

		return fmt.Errorf("failed to get matches from provider api: %w", err)
	}

	providerAPIMatch := s.findProviderAPIMatch(*providerMatch, matches)
	if providerAPIMatch == nil {
		s.logger.Info().Uint("match_id", match.ID).Str("provider", providerMatch.Provider).Msgf("provider match with id %s is not found", providerMatch.ProviderMatchID)
		return s.handleNotFoundProviderMatch(ctx, *match, *providerMatch)
	}

	if _, err := s.providerMatchRepository.Save(ctx, providerAPIMatch.ToProviderMatch(match.ID, providerMatch.Provider)); err != nil {
		return fmt.Errorf("failed to save provider match with id %d and match id %d: %w", providerAPIMatch.ID, match.ID, err)
	}

	switch providerAPIMatch.Status {
	// TODO: add a case for handling not-started matches
	case models.StatusMatchInProgress:
		return s.handleInPlayProviderMatch(ctx, *match, *providerMatch, providerAPIMatch.Time)
	case models.StatusMatchFinished:
		return s.handleFinishedMatch(ctx, match.ID, providerAPIMatch.HomeScore, providerAPIMatch.AwayScore)
	// if we receive here any other status - that is not expected, we should cancel the result check.
	default:
		return s.handleMatchWithUnexpectedStatus(ctx, match.ID, providerAPIMatch.Status, providerAPIMatch.ProviderStatus, providerAPIMatch.ProviderStatusDetails)
	}
}

func (s *ResultCheckerService) findProviderAPIMatch(providerMatch models.ProviderMatch, apiMatches []models.ProviderAPIMatch) *models.ProviderAPIMatch {
	for _, apiMatch := range apiMatches {
		if fmt.Sprintf("%d", apiMatch.ID) == providerMatch.ProviderMatchID {
			return &apiMatch
		}
	}

	return nil
}

// TODO: update provider match as well
func (s *ResultCheckerService) handleMatchWithUnexpectedStatus(ctx context.Context, matchID uint, providerMatchStatus models.ProviderMatchStatus, providerStatus, providerStatusDetails *string) error {
	event := s.logger.Error().Uint("match_id", matchID)
	if providerStatus != nil {
		event = event.Str("provider_status", *providerStatus)
	}
	if providerStatusDetails != nil {
		event = event.Str("provider_status_details", *providerStatusDetails)
	}
	event.Msgf("result check cancelled: provider match status is %s", providerMatchStatus)

	if _, err := s.matchRepository.Update(ctx, matchID, models.Match{ResultStatus: models.Cancelled}); err != nil {
		return fmt.Errorf("failed to update result status of match: %w", err)
	}

	return nil
}

func (s *ResultCheckerService) handleFinishedMatch(ctx context.Context, matchID uint, homeScore, awayScore int) error {
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
		if err != nil && !errors.As(err, &models.ResourceAlreadyExistsError{}) {
			s.logger.Error().Uint("subscription_id", subscription.ID).Err(err).Msg("failed to schedule subscriber notification task")
			errUpdate := s.subscriptionRepository.Update(ctx, subscription.ID, models.Subscription{Status: models.SchedulingErrorSub, SubscriberError: nil})
			if errUpdate != nil {
				s.logger.Error().Err(errUpdate).Uint("subscription_id", subscription.ID).Msg(fmt.Sprintf("failed to update subscription status to: %s", string(models.SchedulingErrorSub)))
			}

			return fmt.Errorf("failed to schedule subscriber notification: %w", err)
		}
	}

	if _, err := s.matchRepository.Update(ctx, matchID, models.Match{ResultStatus: models.Received, HomeScore: &homeScore, AwayScore: &awayScore}); err != nil {
		return fmt.Errorf("failed to handle finished match: %w", err)
	}

	return nil
}

// handleNotFoundMatch marks a single provider's match as unknown.
// When a match is postponed to another date - it is removed from original date matches. In that case retrying doesn't make sense for that provider.
func (s *ResultCheckerService) handleNotFoundMatch(ctx context.Context, providerMatch models.ProviderMatch) error {
	providerMatch.Status = models.StatusMatchUnknown
	if _, err := s.providerMatchRepository.Save(ctx, providerMatch); err != nil {
		return fmt.Errorf("failed to update provider match: %w", err)
	}

	return nil
}

func (s *ResultCheckerService) canCheckResult(match *models.Match) bool {
	return match != nil && (match.ResultStatus == models.Scheduled || match.ResultStatus == models.APIError)
}

func (s *ResultCheckerService) getMatchesFromProvider(ctx context.Context, providerName string, startsAt time.Time) ([]models.ProviderAPIMatch, error) {
	for _, provider := range s.providers {
		if provider.Name() == providerName {
			matches, err := provider.GetMatches(ctx, startsAt)
			if err != nil {
				return nil, fmt.Errorf("failed to get matches from provider %s: %w", providerName, err)
			}

			return matches, nil
		}
	}

	return nil, fmt.Errorf("no registered provider named %s", providerName)
}

func (s *ResultCheckerService) handleInPlayProviderMatch(ctx context.Context, match models.Match, providerMatch models.ProviderMatch, providerStartsAt time.Time) error {
	s.logger.Debug().Uint("match_id", match.ID).Uint("provider_match_id", providerMatch.ID).Msg("match is in play, re-scheduling result check task")

	checkResultTask := providerCheckResultTask(match.CheckResultTasks, providerMatch.ID)
	if checkResultTask == nil {
		return errors.New("match relation result check task doesn't exist")
	}

	scheduleAt := providerStartsAt.Add(s.config.FirstAttemptDelay)
	for i := uint(0); i < checkResultTask.AttemptNumber; i++ {
		scheduleAt = scheduleAt.Add(s.config.Interval)
	}

	attemptNumber := checkResultTask.AttemptNumber + 1

	if attemptNumber > s.config.MaxRetries {
		s.logger.Error().Uint("match_id", match.ID).Msgf("maximum number of attempts %d reached", s.config.MaxRetries)

		if _, errUpdate := s.matchRepository.Update(ctx, match.ID, models.Match{ResultStatus: models.Cancelled}); errUpdate != nil {
			s.logger.Error().Uint("match_id", match.ID).Err(errUpdate).Msg("failed to update match status to cancelled")
			return errUpdate
		}

		return nil
	}

	providerMatchID := providerMatch.ID
	task, err := s.taskClient.ScheduleProviderResultCheck(ctx, providerMatchID, attemptNumber, scheduleAt)
	if err != nil {
		s.logger.Error().Uint("match_id", match.ID).Uint("provider_match_id", providerMatchID).Uint("attempt_number", attemptNumber).Time("schedule_at", scheduleAt).Err(err).Msg("failed to re-schedule check result task")
		if _, errUpdate := s.matchRepository.Update(ctx, match.ID, models.Match{ResultStatus: models.SchedulingError}); errUpdate != nil {
			s.logger.Error().Uint("match_id", match.ID).Err(errUpdate).Msg("failed to update match status to scheduling_error")
		}

		return fmt.Errorf("failed to re-schedule result check task: %w", err)
	}

	if _, err := s.checkResultTaskRepository.Save(ctx, models.CheckResultTask{
		MatchID:         match.ID,
		ProviderMatchID: &providerMatchID,
		Name:            task.Name,
		AttemptNumber:   attemptNumber,
		ExecuteAt:       task.ExecuteAt,
	}); err != nil {
		return fmt.Errorf("failed to update result check task: %w", err)
	}

	return nil
}

func (s *ResultCheckerService) handleNotFoundProviderMatch(ctx context.Context, match models.Match, providerMatch models.ProviderMatch) error {
	if err := s.handleNotFoundMatch(ctx, providerMatch); err != nil {
		return err
	}

	s.logger.Error().Uint("match_id", match.ID).Msg("result check cancelled: provider match not found")

	if _, err := s.matchRepository.Update(ctx, match.ID, models.Match{ResultStatus: models.Cancelled}); err != nil {
		return fmt.Errorf("failed to update match: %w", err)
	}

	return nil
}

func providerCheckResultTask(tasks []models.CheckResultTask, providerMatchID uint) *models.CheckResultTask {
	for i := range tasks {
		if tasks[i].ProviderMatchID != nil && *tasks[i].ProviderMatchID == providerMatchID {
			return &tasks[i]
		}
	}

	return nil
}
