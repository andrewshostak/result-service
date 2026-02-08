package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/internal/app/models"
)

type SubscriptionService struct {
	subscriptionRepository SubscriptionRepository
	matchRepository        MatchRepository
	aliasRepository        AliasRepository
	taskClient             TaskClient
	logger                 Logger
}

func NewSubscriptionService(
	subscriptionRepository SubscriptionRepository,
	matchRepository MatchRepository,
	aliasRepository AliasRepository,
	taskClient TaskClient,
	logger Logger,
) *SubscriptionService {
	return &SubscriptionService{
		subscriptionRepository: subscriptionRepository,
		matchRepository:        matchRepository,
		aliasRepository:        aliasRepository,
		taskClient:             taskClient,
		logger:                 logger,
	}
}

func (s *SubscriptionService) Create(ctx context.Context, request models.CreateSubscriptionRequest) error {
	match, err := s.matchRepository.One(ctx, models.Match{ID: request.MatchID})
	if err != nil {
		return fmt.Errorf("failed to get a match: %w", err)
	}

	if !s.isMatchResultScheduled(*match) {
		return errs.NewUnprocessableContentError(errors.New("match result status doesn't allow to create a subscription"))
	}

	_, err = s.subscriptionRepository.Create(ctx, models.Subscription{
		MatchID: request.MatchID,
		Key:     request.SecretKey,
		Url:     request.URL,
	})

	if errors.As(err, &errs.ResourceAlreadyExistsError{}) {
		s.logger.Error().Uint("subscription_id", match.ID).Msg("subscription already exists")
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	s.logger.Debug().Uint("subscription_id", match.ID).Msg("subscription created")

	return nil
}

func (s *SubscriptionService) Delete(ctx context.Context, request models.DeleteSubscriptionRequest) error {
	aliasHome, err := s.aliasRepository.Find(ctx, request.AliasHome)
	if err != nil {
		return fmt.Errorf("failed to find home team alias: %w", err)
	}

	aliasAway, err := s.aliasRepository.Find(ctx, request.AliasAway)
	if err != nil {
		return fmt.Errorf("failed to find away team alias: %w", err)
	}

	match, err := s.matchRepository.One(ctx, models.Match{
		StartsAt: request.StartsAt.UTC(),
		HomeTeam: &models.Team{ID: aliasHome.TeamID},
		AwayTeam: &models.Team{ID: aliasAway.TeamID},
	})
	if err != nil {
		return fmt.Errorf("failed to find match: %w", err)
	}

	subscription, err := s.subscriptionRepository.One(ctx, match.ID, request.SecretKey, request.BaseURL)
	if err != nil {
		return fmt.Errorf("failed to find subscription: %w", err)
	}

	if s.isSubscriberNotified(*subscription) {
		return errs.NewUnprocessableContentError(errors.New("not allowed to delete successfully notified subscription"))
	}

	err = s.subscriptionRepository.Delete(ctx, subscription.ID)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	s.logger.Debug().Uint("subscription_id", subscription.ID).Msg("subscription deleted")

	otherSubscriptions, errList := s.subscriptionRepository.List(ctx, match.ID)
	if errList != nil {
		s.logger.Error().Err(err).Uint("match_id", match.ID).Msg("failed to check other subscriptions presence")
		return nil
	}

	if len(otherSubscriptions) > 0 {
		s.logger.Debug().Uint("match_id", match.ID).Msg("there are other subscriptions for the match")
		return nil
	}

	errDelete := s.matchRepository.Delete(ctx, match.ID)
	if errDelete != nil {
		s.logger.Error().Err(errDelete).Uint("match_id", match.ID).Msg("failed to delete match")
		return nil
	}

	if match.CheckResultTask == nil {
		s.logger.Error().Uint("match_id", match.ID).Msg("match relation check result task does not exist")
		return nil
	}

	if err := s.taskClient.DeleteResultCheckTask(ctx, match.CheckResultTask.Name); err != nil {
		s.logger.Error().Err(err).Uint("match_id", match.ID).Msg("failed to delete result-check task")
		return nil
	}

	s.logger.Debug().Uint("match_id", match.ID).Msg("match & result check task deleted")

	return nil
}

func (s *SubscriptionService) isMatchResultScheduled(match models.Match) bool {
	return match.ResultStatus == models.Scheduled
}

func (s *SubscriptionService) isSubscriberNotified(subscription models.Subscription) bool {
	return subscription.Status == models.SuccessfulSub
}
