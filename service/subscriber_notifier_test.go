package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/client"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/repository"
	"github.com/andrewshostak/result-service/service"
	"github.com/andrewshostak/result-service/service/mocks"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSubscriberNotifierService_NotifySubscriber(t *testing.T) {
	ctx := context.Background()
	subscriptionID, matchID := uint(gofakeit.Uint8()), uint(gofakeit.Uint8())

	errorMessage := "unexpected error"
	unexpectedErr := errors.New(errorMessage)

	subscription := fakeRepositorySubscription(func(s *repository.Subscription) {
		s.ID = subscriptionID
		s.MatchID = matchID
		s.Status = "pending"
	})

	externalMatch := fakeExternalMatchRepository(func(m *repository.ExternalMatch) {
		m.MatchID = matchID
	})

	match := fakeRepositoryMatch(func(m *repository.Match) {
		m.ID = matchID
		m.ExternalMatch = &externalMatch
	})

	notification := fakeClientNotification(func(n *client.Notification) {
		n.Home = uint(match.ExternalMatch.HomeScore)
		n.Away = uint(match.ExternalMatch.AwayScore)
		n.Url = subscription.Url
		n.Key = subscription.Key
	})

	tests := []struct {
		name                   string
		input                  uint
		matchRepository        func(t *testing.T) *mocks.MatchRepository
		notifierClient         func(t *testing.T) *mocks.NotifierClient
		subscriptionRepository func(t *testing.T) *mocks.SubscriptionRepository
		expectedErr            error
	}{
		{
			name:  "success - it returns nil when processing unnotified subscription",
			input: subscriptionID,
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Get", ctx, subscriptionID).Return(&subscription, nil).Once()
				m.On("Update", ctx, subscriptionID, mock.MatchedBy(subscriptionMatchedFunc)).Return(nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&match, nil).Once()
				return m
			},
			notifierClient: func(t *testing.T) *mocks.NotifierClient {
				t.Helper()
				m := mocks.NewNotifierClient(t)
				m.On("Notify", ctx, notification).Return(nil).Once()
				return m
			},
			expectedErr: nil,
		},
		{
			name:  "it returns an error when subscription retrieval fails",
			input: subscriptionID,
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Get", ctx, subscriptionID).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get subscription by id: %w", unexpectedErr),
		},
		{
			name:  "success - it returns nil when subscription is already notified",
			input: subscriptionID,
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Get", ctx, subscriptionID).Return(&repository.Subscription{
					ID:     subscriptionID,
					Status: "successful",
				}, nil).Once()
				return m
			},
			expectedErr: nil,
		},
		{
			name:  "it returns an error when match retrieval fails",
			input: subscriptionID,
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Get", ctx, subscriptionID).Return(&repository.Subscription{
					ID:      subscriptionID,
					Status:  "pending",
					MatchID: matchID,
				}, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get match: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when match relation external match doesn't exist",
			input: subscriptionID,
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Get", ctx, subscriptionID).Return(&repository.Subscription{
					ID:      subscriptionID,
					Status:  "pending",
					MatchID: matchID,
				}, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&repository.Match{ID: matchID}, nil).Once()
				return m
			},
			expectedErr: errors.New("no external match found for the match"),
		},
		{
			name:  "it returns an error when notifier fails and subscription update fails",
			input: subscriptionID,
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Get", ctx, subscriptionID).Return(&subscription, nil).Once()
				m.On("Update", ctx, subscriptionID, repository.Subscription{
					Status: string(service.SubscriberErrorSub),
					Error:  &errorMessage,
				}).Return(unexpectedErr).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&match, nil).Once()
				return m
			},
			notifierClient: func(t *testing.T) *mocks.NotifierClient {
				t.Helper()
				m := mocks.NewNotifierClient(t)
				m.On("Notify", ctx, notification).Return(unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to notify subscriber: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when notifier fails and subscription update succeeds",
			input: subscriptionID,
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Get", ctx, subscriptionID).Return(&subscription, nil).Once()
				m.On("Update", ctx, subscriptionID, repository.Subscription{
					Status: string(service.SubscriberErrorSub),
					Error:  &errorMessage,
				}).Return(nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&match, nil).Once()
				return m
			},
			notifierClient: func(t *testing.T) *mocks.NotifierClient {
				t.Helper()
				m := mocks.NewNotifierClient(t)
				m.On("Notify", ctx, notification).Return(unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to notify subscriber: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when notifier succeeds but subscription update fails",
			input: subscriptionID,
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Get", ctx, subscriptionID).Return(&subscription, nil).Once()
				m.On("Update", ctx, subscriptionID, mock.MatchedBy(subscriptionMatchedFunc)).Return(unexpectedErr).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&match, nil).Once()
				return m
			},
			notifierClient: func(t *testing.T) *mocks.NotifierClient {
				t.Helper()
				m := mocks.NewNotifierClient(t)
				m.On("Notify", ctx, notification).Return(nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to update subscription status to %s: %w", string(service.SuccessfulSub), unexpectedErr),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var matchRepository *mocks.MatchRepository
			if tt.matchRepository != nil {
				matchRepository = tt.matchRepository(t)
			}

			var subscriptionRepository *mocks.SubscriptionRepository
			if tt.subscriptionRepository != nil {
				subscriptionRepository = tt.subscriptionRepository(t)
			}

			var notifierClient *mocks.NotifierClient
			if tt.notifierClient != nil {
				notifierClient = tt.notifierClient(t)
			}

			logger := loggerinternal.SetupLogger()

			sns := service.NewSubscriberNotifierService(subscriptionRepository, matchRepository, notifierClient, logger)

			err := sns.NotifySubscriber(ctx, tt.input)
			if tt.expectedErr != nil {
				assert.ErrorContains(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func fakeClientNotification(options ...Option[client.Notification]) client.Notification {
	notification := client.Notification{
		Url:  gofakeit.URL(),
		Key:  gofakeit.UUID(),
		Home: uint(gofakeit.Uint8()),
		Away: uint(gofakeit.Uint8()),
	}

	applyOptions(&notification, options...)

	return notification
}

func subscriptionMatchedFunc(actual repository.Subscription) bool {
	if actual.Error != nil {
		return false
	}

	if actual.Status != string(service.SuccessfulSub) {
		return false
	}

	if actual.NotifiedAt.After(time.Now()) {
		return false
	}

	return true
}
