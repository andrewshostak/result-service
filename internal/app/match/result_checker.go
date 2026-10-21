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
	externalMatchRepository   ExternalMatchRepository
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
	externalMatchRepository ExternalMatchRepository,
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
		externalMatchRepository:   externalMatchRepository,
		providerMatchRepository:   providerMatchRepository,
		subscriptionRepository:    subscriptionRepository,
		checkResultTaskRepository: checkResultTaskRepository,
		taskClient:                taskClient,
		providers:                 providers,
		logger:                    logger,
	}
}

func (s *ResultCheckerService) CheckResult(ctx context.Context, matchID uint) error {
	match, err := s.matchRepository.One(ctx, models.Match{ID: matchID})
	if err != nil {
		return fmt.Errorf("failed to get match by id: %w", err)
	}

	if !s.canCheckResult(match) {
		s.logger.Error().Uint("match_id", matchID).Msg(fmt.Sprintf("result check skipped: unexpected result status %s", match.ResultStatus))
		return nil
	}

	// TODO: change the logic to get all existing provider matches (by match id) and iterate through all of them
	if match.ExternalMatch == nil {
		s.logger.Error().Uint("match_id", matchID).Msg("match relation external match doesn't exist")
		return errors.New("match relation external match doesn't exist")
	}

	matches, err := s.getMatchesFromProviders(ctx, match.StartsAt)
	if err != nil {
		s.logger.Error().Uint("match_id", matchID).Err(err)
		if _, errUpdate := s.matchRepository.Update(ctx, match.ID, models.Match{ResultStatus: models.APIError}); errUpdate != nil {
			s.logger.Error().Uint("match_id", matchID).Err(errUpdate)
		}

		return fmt.Errorf("failed to get matches from external api: %w", err)
	}

	externalAPIMatch, providerName := s.findExternalMatchByID(match.ExternalMatch.ID, matches)
	if externalAPIMatch == nil {
		s.logger.Info().Uint("match_id", matchID).Msgf("external match with id %d is not found", match.ExternalMatch.ID)

		return s.handleNotFoundMatch(ctx, *match.ExternalMatch, match.StartsAt)
	}

	externalMatch, err := s.externalMatchRepository.Save(ctx, &match.ExternalMatch.ID, externalAPIMatch.ToExternalMatch(match.ID))
	if err != nil {
		return fmt.Errorf("failed to update external match: %w", err)
	}
	_, err = s.providerMatchRepository.Save(ctx, externalAPIMatch.ToProviderMatch(match.ID, providerName))
	if err != nil {
		return fmt.Errorf("failed to save provider match with id %d and match id %d: %w", externalAPIMatch.ID, match.ID, err)
	}

	switch externalAPIMatch.Status {
	// TODO: add a case when match was paused after some time and its status is 'not_started'
	case models.StatusMatchInProgress:
		return s.handleInPlayMatch(ctx, *match)
	case models.StatusMatchFinished:
		return s.handleFinishedMatch(ctx, match.ID, *externalMatch)
	// if we receive here any other status - that is not expected, we should cancel the result check.
	default:
		return s.handleMatchWithUnexpectedStatus(ctx, matchID, externalAPIMatch.Status)
	}
}

func (s *ResultCheckerService) findExternalMatchByID(externalID uint, matches map[string][]models.ExternalAPIMatch) (*models.ExternalAPIMatch, string) {
	for providerName, providerMatches := range matches {
		for _, match := range providerMatches {
			if match.ID == externalID {
				return &match, providerName
			}
		}
	}

	return nil, ""
}

// TODO: update external match as well
func (s *ResultCheckerService) handleMatchWithUnexpectedStatus(ctx context.Context, matchID uint, externalMatchStatus models.ExternalMatchStatus) error {
	s.logger.Error().Uint("match_id", matchID).Msgf("result check cancelled: external match status is %s", externalMatchStatus)

	if _, err := s.matchRepository.Update(ctx, matchID, models.Match{ResultStatus: models.Cancelled}); err != nil {
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

	if attemptNumber > s.config.MaxRetries {
		s.logger.Error().Uint("match_id", match.ID).Msgf("maximum number of attempts %d reached", s.config.MaxRetries)

		if _, errUpdate := s.matchRepository.Update(ctx, match.ID, models.Match{ResultStatus: models.Cancelled}); errUpdate != nil {
			s.logger.Error().Uint("match_id", match.ID).Err(errUpdate)
			return errUpdate
		}

		return nil
	}

	task, err := s.taskClient.ScheduleResultCheck(ctx, match.ID, attemptNumber, scheduleAt)
	if err != nil {
		s.logger.Error().Uint("match_id", match.ID).Uint("attempt_number", attemptNumber).Time("schedule_at", scheduleAt).Err(err).Msg("failed to re-schedule check result task")
		if _, errUpdate := s.matchRepository.Update(ctx, match.ID, models.Match{ResultStatus: models.SchedulingError}); errUpdate != nil {
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

func (s *ResultCheckerService) handleFinishedMatch(ctx context.Context, matchID uint, externalMatch models.ExternalMatch) error {
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

	if _, err := s.matchRepository.Update(ctx, matchID, models.Match{ResultStatus: models.Received, HomeScore: &externalMatch.HomeScore, AwayScore: &externalMatch.AwayScore}); err != nil {
		return fmt.Errorf("failed to handle finished match: %w", err)
	}

	return nil
}

// handleNotFoundMatch updates statuses of match and external match.
// When a match is postponed to another date - it is removed from original date matches. In that case retrying doesn't make sense, so returning nil.
func (s *ResultCheckerService) handleNotFoundMatch(ctx context.Context, externalMatch models.ExternalMatch, startsAt time.Time) error {
	externalMatch.Status = models.StatusMatchUnknown
	if _, err := s.externalMatchRepository.Save(ctx, &externalMatch.ID, externalMatch); err != nil {
		return fmt.Errorf("failed to update external match: %w", err)
	}

	if _, err := s.providerMatchRepository.Save(ctx, models.ProviderMatch{
		MatchID:         externalMatch.MatchID,
		Provider:        fotmobProvider,
		ProviderMatchID: fmt.Sprintf("%d", externalMatch.ID),
		StartsAt:        startsAt,
		HomeScore:       externalMatch.HomeScore,
		AwayScore:       externalMatch.AwayScore,
		Status:          models.StatusMatchUnknown,
	}); err != nil {
		return fmt.Errorf("failed to save provider match: %w", err)
	}

	if _, err := s.matchRepository.Update(ctx, externalMatch.MatchID, models.Match{ResultStatus: models.Cancelled}); err != nil {
		return fmt.Errorf("failed to update match: %w", err)
	}

	return nil
}

func (s *ResultCheckerService) getMatchesFromProviders(ctx context.Context, startsAt time.Time) (map[string][]models.ExternalAPIMatch, error) {
	var matchesErrors []error
	matches := make(map[string][]models.ExternalAPIMatch)
	for _, provider := range s.providers {
		name := provider.Name()
		// parallelize in case of multiple providers
		providerMatches, err := provider.GetMatches(ctx, startsAt)
		if err != nil {
			matchesErrors = append(matchesErrors, fmt.Errorf("failed to get matches from provider %s: %w", name, err))
			continue
		}

		matches[name] = providerMatches
	}

	if len(matches) != 0 {
		return matches, nil
	}

	return nil, fmt.Errorf("failed to get matches from providers: %w", errors.Join(matchesErrors...))
}

func (s *ResultCheckerService) canCheckResult(match *models.Match) bool {
	return match != nil && (match.ResultStatus == models.Scheduled || match.ResultStatus == models.APIError)
}
