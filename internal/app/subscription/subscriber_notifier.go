package subscription

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
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
	sub, err := s.subscriptionRepository.Get(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to get subscription by id: %w", err)
	}

	if s.isNotified(*sub) {
		s.logger.Error().Uint("subscription_id", sub.ID).Msg("subscription is already notified")
		return nil
	}

	m, err := s.matchRepository.One(ctx, models.Match{ID: sub.MatchID})
	if err != nil {
		return fmt.Errorf("failed to get match: %w", err)
	}

	if m.ExternalMatch == nil {
		return fmt.Errorf("match relation external match doesn't exist")
	}

	err = s.notifierClient.Notify(ctx, models.SubscriberNotification{
		Url:  sub.Url,
		Key:  sub.Key,
		Home: uint(m.ExternalMatch.HomeScore),
		Away: uint(m.ExternalMatch.AwayScore),
	})
	if err != nil {
		s.logger.Error().Err(err).Uint("subscription_id", sub.ID).Msg("failed to notify subscriber")

		errMessage := err.Error()
		errUpdate := s.subscriptionRepository.Update(ctx, sub.ID, models.Subscription{
			Status:          models.SubscriberErrorSub,
			SubscriberError: &errMessage,
		})
		if errUpdate != nil {
			s.logger.Error().Err(errUpdate).Uint("subscription_id", sub.ID).Msg(fmt.Sprintf("failed to update subscription status to: %s", string(models.SubscriberErrorSub)))
		}

		return fmt.Errorf("failed to notify subscriber: %w", err)
	}

	notifiedAt := time.Now()
	errUpdate := s.subscriptionRepository.Update(ctx, sub.ID, models.Subscription{
		Status:     models.SuccessfulSub,
		NotifiedAt: &notifiedAt,
	})
	if errUpdate != nil {
		s.logger.Error().Err(errUpdate).Uint("subscription_id", sub.ID).Msg(fmt.Sprintf("failed to update subscription status to: %s", string(models.SuccessfulSub)))
		return fmt.Errorf("failed to update subscription status to %s: %w", string(models.SuccessfulSub), errUpdate)
	}

	s.logger.Debug().Uint("subscription_id", sub.ID).Msg("subscriber notified")

	return nil
}

func (s *SubscriberNotifierService) isNotified(subscription models.Subscription) bool {
	return subscription.Status == models.SuccessfulSub
}
