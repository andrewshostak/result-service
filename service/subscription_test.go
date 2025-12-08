package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/andrewshostak/result-service/repository"
	"github.com/andrewshostak/result-service/service"
	"github.com/andrewshostak/result-service/service/mocks"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
)

func TestSubscriptionService_Create(t *testing.T) {
	ctx := context.Background()
	matchID := uint(gofakeit.Uint8())
	url := gofakeit.URL()
	secretKey := gofakeit.UUID()
	request := service.CreateSubscriptionRequest{
		MatchID:   matchID,
		URL:       url,
		SecretKey: secretKey,
	}

	tests := []struct {
		name                   string
		input                  service.CreateSubscriptionRequest
		matchRepository        func(t *testing.T) *mocks.MatchRepository
		subscriptionRepository func(t *testing.T) *mocks.SubscriptionRepository
		expectedErr            error
	}{
		{
			name:  "it returns an error when match does not exist",
			input: request,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(nil, errors.New("not found")).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get a match: %w", errors.New("not found")),
		},
		{
			name:  "it returns an error when match result status is not scheduled",
			input: request,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&repository.Match{
					ID:           matchID,
					ResultStatus: string(service.Received),
				}, nil).Once()
				return m
			},
			expectedErr: errors.New("match result status doesn't allow to create a subscription"),
		},
		{
			name:  "it returns an error when subscription creation fails",
			input: request,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&repository.Match{
					ID:           matchID,
					ResultStatus: string(service.Scheduled),
				}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Create", ctx, repository.Subscription{
					MatchID: matchID,
					Key:     secretKey,
					Url:     url,
				}).Return(nil, errors.New("database error")).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to create subscription: %w", errors.New("database error")),
		},
		{
			name:  "it creates subscription successfully",
			input: request,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&repository.Match{
					ID:           matchID,
					ResultStatus: string(service.Scheduled),
				}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Create", ctx, repository.Subscription{
					MatchID: matchID,
					Key:     secretKey,
					Url:     url,
				}).Return(&repository.Subscription{
					ID:      uint(gofakeit.Uint8()),
					MatchID: matchID,
					Key:     secretKey,
					Url:     url,
				}, nil).Once()
				return m
			},
			expectedErr: nil,
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

			logger := mocks.NewLogger(t)
			aliasRepository := mocks.NewAliasRepository(t)
			taskClient := mocks.NewTaskClient(t)

			ss := service.NewSubscriptionService(subscriptionRepository, matchRepository, aliasRepository, taskClient, logger)

			err := ss.Create(ctx, tt.input)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
