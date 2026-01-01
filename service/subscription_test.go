package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/errs"
	loggerinternal "github.com/andrewshostak/result-service/logger"
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

func TestSubscriptionService_Delete(t *testing.T) {
	ctx := context.Background()

	input := service.DeleteSubscriptionRequest{
		StartsAt:  time.Time{},
		AliasHome: gofakeit.Word(),
		AliasAway: gofakeit.Word(),
		BaseURL:   gofakeit.URL(),
		SecretKey: gofakeit.Password(true, true, true, false, false, 10),
	}

	unexpectedErr := errors.New("unexpected error")

	aliasHome := fakeRepositoryAlias(func(a *repository.Alias) {
		a.Alias = input.AliasHome
	})
	aliasAway := fakeRepositoryAlias(func(a *repository.Alias) {
		a.Alias = input.AliasAway
	})
	match := fakeRepositoryMatch(func(m *repository.Match) {
		m.StartsAt = input.StartsAt.UTC()
		m.HomeTeamID = aliasHome.TeamID
		m.AwayTeamID = aliasAway.TeamID
	})
	subscription := fakeRepositorySubscription(func(s *repository.Subscription) {
		s.MatchID = match.ID
		s.Key = input.SecretKey
		s.Url = fmt.Sprintf("%s/asd/zxc", input.BaseURL)
		s.Status = string(service.PendingSub)
	})
	checkResultTask := fakeRepositoryCheckResultTask(func(t *repository.CheckResultTask) {
		t.MatchID = match.ID
	})

	tests := []struct {
		name                   string
		input                  service.DeleteSubscriptionRequest
		matchRepository        func(t *testing.T) *mocks.MatchRepository
		subscriptionRepository func(t *testing.T) *mocks.SubscriptionRepository
		aliasRepository        func(t *testing.T) *mocks.AliasRepository
		taskClient             func(t *testing.T) *mocks.TaskClient
		expectedErr            error
	}{
		{
			name:  "it returns error when home alias not found",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find home team alias: %w", unexpectedErr),
		},
		{
			name:  "it returns error when away alias not found",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find away team alias: %w", unexpectedErr),
		},
		{
			name:  "it returns error when match not found",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find a match: %w", unexpectedErr),
		},
		{
			name:  "it returns error when subscription not found",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find a subscription: %w", unexpectedErr),
		},
		{
			name:  "it returns error when subscription deletion is not allowed",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&repository.Subscription{
					Status: string(service.SuccessfulSub),
				}, nil).Once()
				return m
			},
			expectedErr: errs.SubscriptionDeleteNotAllowedError{Message: "not allowed to delete successfully notified subscription"},
		},
		{
			name:  "it returns error when subscription deletion fails",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to delete subscription: %w", unexpectedErr),
		},
		{
			name:  "it returns nil when subscription list returns error",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return(nil, unexpectedErr).Once()
				return m
			},
		},
		{
			name:  "it returns nil when subscription list contains at least one subscription",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]repository.Subscription{{}}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns nil when match repository delete returns error",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				m.On("Delete", ctx, match.ID).Return(unexpectedErr).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]repository.Subscription{}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns nil when match repository delete succeeds and match doesnt have check-result-task relation",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				m.On("Delete", ctx, match.ID).Return(nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]repository.Subscription{}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns nil when check result task client returns error",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				matchWithTask := match
				matchWithTask.CheckResultTask = &checkResultTask
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&matchWithTask, nil).Once()
				m.On("Delete", ctx, match.ID).Return(nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]repository.Subscription{}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("DeleteResultCheckTask", ctx, checkResultTask.Name).Return(unexpectedErr).Once()
				return m
			},
		},
		{
			name:  "it returns nil on successful flow",
			input: input,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, input.AliasHome).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, input.AliasAway).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				matchWithTask := match
				matchWithTask.CheckResultTask = &checkResultTask
				m.On("One", ctx, repository.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&matchWithTask, nil).Once()
				m.On("Delete", ctx, match.ID).Return(nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]repository.Subscription{}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("DeleteResultCheckTask", ctx, checkResultTask.Name).Return(nil).Once()
				return m
			},
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

			var aliasRepository *mocks.AliasRepository
			if tt.aliasRepository != nil {
				aliasRepository = tt.aliasRepository(t)
			}

			var taskClient *mocks.TaskClient
			if tt.taskClient != nil {
				taskClient = tt.taskClient(t)
			}

			logger := loggerinternal.SetupLogger()

			ss := service.NewSubscriptionService(subscriptionRepository, matchRepository, aliasRepository, taskClient, logger)

			err := ss.Delete(ctx, tt.input)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func fakeRepositoryCheckResultTask(options ...Option[repository.CheckResultTask]) repository.CheckResultTask {
	task := repository.CheckResultTask{
		ID:            uint(gofakeit.Uint8()),
		MatchID:       uint(gofakeit.Uint8()),
		Name:          gofakeit.Name(),
		AttemptNumber: 1,
		ExecuteAt:     time.Now(),
		CreatedAt:     time.Now(),
	}

	applyOptions(&task, options...)

	return task
}
