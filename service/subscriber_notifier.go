package service

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/client"
	"github.com/andrewshostak/result-service/repository"
)

type SubscriberNotifierService struct {
	subscriptionRepository SubscriptionRepository
	matchRepository        MatchRepository
	notifierClient         NotifierClient
	logger                 Logger
}

func NewSubscriberNotifierService(
	subscriptionRepository SubscriptionRepository,
	matchRepository MatchRepository,
	notifierClient NotifierClient,
	logger Logger,
) *SubscriberNotifierService {
	return &SubscriberNotifierService{
		subscriptionRepository: subscriptionRepository,
		matchRepository:        matchRepository,
		notifierClient:         notifierClient,
		logger:                 logger,
	}
}

func (s *SubscriberNotifierService) NotifySubscriber(ctx context.Context, subscriptionID uint) error {
	subscription, err := s.subscriptionRepository.Get(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to get subscription by id: %w", err)
	}

	match, err := s.matchRepository.One(ctx, repository.Match{ID: subscription.MatchID})
	if err != nil {
		return fmt.Errorf("failed to get match: %w", err)
	}

	m := fromRepositoryMatch(*match)

	if m.ExternalMatch == nil {
		return fmt.Errorf("no external match found for the match")
	}

	err = s.notifierClient.Notify(ctx, client.Notification{
		Url:  subscription.Url,
		Key:  subscription.Key,
		Home: uint(m.ExternalMatch.HomeScore),
		Away: uint(m.ExternalMatch.AwayScore),
	})
	if err != nil {
		errUpdate := s.subscriptionRepository.Update(ctx, subscription.ID, repository.Subscription{Status: string(SubscriberErrorSub)})
		if errUpdate != nil {
			s.logger.Error().Err(errUpdate).Uint("subscription_id", subscription.ID).Msg(fmt.Sprintf("failed to update subscription status to: %s", string(SubscriberErrorSub)))
		}

		return fmt.Errorf("failed to notify subscriber: %w", err)
	}

	errUpdate := s.subscriptionRepository.Update(ctx, subscription.ID, repository.Subscription{Status: string(SuccessfulSub)})
	if errUpdate != nil {
		s.logger.Error().Err(errUpdate).Uint("subscription_id", subscription.ID).Msg(fmt.Sprintf("failed to update subscription status to: %s", string(SuccessfulSub)))
	}

	return nil
}
