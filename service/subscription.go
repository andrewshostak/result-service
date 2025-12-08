package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/repository"
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

func (s *SubscriptionService) Create(ctx context.Context, request CreateSubscriptionRequest) error {
	m, err := s.matchRepository.One(ctx, repository.Match{ID: request.MatchID})
	if err != nil {
		return fmt.Errorf("failed to get a match: %w", err)
	}
	match, err := fromRepositoryMatch(*m)
	if err != nil {
		return fmt.Errorf("failed to map from repository match: %w", err)
	}

	if !s.isMatchResultScheduled(*match) {
		return errors.New("match result status doesn't allow to create a subscription")
	}

	_, err = s.subscriptionRepository.Create(ctx, repository.Subscription{
		MatchID: request.MatchID,
		Key:     request.SecretKey,
		Url:     request.URL,
	})

	if err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	return nil
}

func (s *SubscriptionService) Delete(ctx context.Context, request DeleteSubscriptionRequest) error {
	aliasHome, err := s.aliasRepository.Find(ctx, request.AliasHome)
	if err != nil {
		return fmt.Errorf("failed to find home team alias: %w", err)
	}

	aliasAway, err := s.aliasRepository.Find(ctx, request.AliasAway)
	if err != nil {
		return fmt.Errorf("failed to find away team alias: %w", err)
	}

	m, err := s.matchRepository.One(ctx, repository.Match{
		StartsAt:   request.StartsAt.UTC(),
		HomeTeamID: aliasHome.TeamID,
		AwayTeamID: aliasAway.TeamID,
	})
	if err != nil {
		return fmt.Errorf("failed to find a match: %w", err)
	}
	match, err := fromRepositoryMatch(*m)
	if err != nil {
		return fmt.Errorf("failed to map from repository match: %w", err)
	}

	found, err := s.subscriptionRepository.One(ctx, match.ID, request.SecretKey, request.BaseURL)
	if err != nil {
		return fmt.Errorf("failed to find a subscription: %w", err)
	}

	subscription, err := fromRepositorySubscription(*found)
	if err != nil {
		return fmt.Errorf("failed to map from repository subscription: %w", err)
	}

	if subscription.Status != "pending" {
		return errs.SubscriptionNotFoundError{Message: fmt.Sprintf("subscription %d has status %s instead of %s", subscription.ID, subscription.Status, "pending")}
	}

	err = s.subscriptionRepository.Delete(ctx, subscription.ID)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	s.logger.Info().Uint("subscription_id", subscription.ID).Msg("subscription deleted")

	otherSubscriptions, errList := s.subscriptionRepository.List(ctx, match.ID)
	if errList != nil {
		s.logger.Error().Err(err).Uint("match_id", match.ID).Msg("failed to check other subscriptions presence")
		return nil
	}

	if len(otherSubscriptions) > 0 {
		s.logger.Info().Uint("match_id", match.ID).Msg("there are other subscriptions for the match. no need to cancel result acquiring task")
		return nil
	}

	errDelete := s.matchRepository.Delete(ctx, match.ID)
	if errDelete != nil {
		s.logger.Error().Err(errDelete).Uint("match_id", match.ID).Msg("failed to delete match")
		return nil
	}

	s.logger.Info().Uint("match_id", match.ID).Msg("match deleted")

	if match.CheckResultTask == nil {
		s.logger.Error().Uint("match_id", match.ID).Msg("failed to cancel scheduled task: match relation check result task is not found")
		return nil
	}

	if err := s.taskClient.DeleteResultCheckTask(ctx, match.CheckResultTask.Name); err != nil {
		s.logger.Error().Err(err).Uint("match_id", match.ID).Msg("failed to delete result-check task")
		return nil
	}

	s.logger.Info().Uint("match_id", match.ID).Msg("result check task deleted")

	return nil
}

func (s *SubscriptionService) isMatchResultScheduled(match Match) bool {
	return match.ResultStatus == Scheduled
}
