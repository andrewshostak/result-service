package subscription_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/andrewshostak/result-service/internal/app/subscription"
	sub "github.com/andrewshostak/result-service/internal/app/subscription"
	"github.com/andrewshostak/result-service/internal/app/subscription/mocks"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/testutils"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
)

func TestSubscriptionService_Create(t *testing.T) {
	ctx := context.Background()
	matchID := uint(gofakeit.Uint8())
	url := gofakeit.URL()
	secretKey := gofakeit.UUID()
	request := models.CreateSubscriptionRequest{
		MatchID:   matchID,
		URL:       url,
		SecretKey: secretKey,
	}

	tests := []struct {
		name                   string
		input                  models.CreateSubscriptionRequest
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
				m.On("One", ctx, models.Match{ID: matchID}).Return(nil, errors.New("not found")).Once()
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
				m.On("One", ctx, models.Match{ID: matchID}).Return(&models.Match{
					ID:           matchID,
					ResultStatus: models.Received,
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
				m.On("One", ctx, models.Match{ID: matchID}).Return(&models.Match{
					ID:           matchID,
					ResultStatus: models.Scheduled,
				}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Create", ctx, models.Subscription{
					MatchID: matchID,
					Key:     secretKey,
					Url:     url,
				}).Return(nil, errors.New("database error")).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to create subscription: %w", errors.New("database error")),
		},
		{
			name:  "it returns nil when subscription creation fails because subscription already exists",
			input: request,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&models.Match{
					ID:           matchID,
					ResultStatus: models.Scheduled,
				}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Create", ctx, models.Subscription{
					MatchID: matchID,
					Key:     secretKey,
					Url:     url,
				}).Return(nil, errs.NewResourceAlreadyExistsError(errors.New("already exists"))).Once()
				return m
			},
		},
		{
			name:  "success - it creates subscription",
			input: request,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&models.Match{
					ID:           matchID,
					ResultStatus: models.Scheduled,
				}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("Create", ctx, models.Subscription{
					MatchID: matchID,
					Key:     secretKey,
					Url:     url,
				}).Return(&models.Subscription{
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

			logger := loggerinternal.SetupLogger()
			aliasRepository := mocks.NewAliasRepository(t)
			taskClient := mocks.NewTaskClient(t)

			ss := subscription.NewSubscriptionService(subscriptionRepository, matchRepository, aliasRepository, taskClient, logger)

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

	input := models.DeleteSubscriptionRequest{
		StartsAt:  time.Time{},
		AliasHome: gofakeit.Word(),
		AliasAway: gofakeit.Word(),
		BaseURL:   gofakeit.URL(),
		SecretKey: gofakeit.Password(true, true, true, false, false, 10),
	}

	unexpectedErr := errors.New("unexpected error")

	aliasHome := testutils.FakeAlias(func(a *models.Alias) {
		a.Alias = input.AliasHome
	})
	aliasAway := testutils.FakeAlias(func(a *models.Alias) {
		a.Alias = input.AliasAway
	})
	match := testutils.FakeMatch(func(m *models.Match) {
		m.StartsAt = input.StartsAt.UTC()
		m.HomeTeam = &models.Team{ID: aliasHome.TeamID}
		m.AwayTeam = &models.Team{ID: aliasAway.TeamID}
	})
	subscription := testutils.FakeSubscription(func(s *models.Subscription) {
		s.MatchID = match.ID
		s.Key = input.SecretKey
		s.Url = fmt.Sprintf("%s/asd/zxc", input.BaseURL)
		s.Status = models.PendingSub
	})
	checkResultTask := testutils.FakeCheckResultTask(func(t *models.CheckResultTask) {
		t.MatchID = match.ID
	})

	tests := []struct {
		name                   string
		input                  models.DeleteSubscriptionRequest
		matchRepository        func(t *testing.T) *mocks.MatchRepository
		subscriptionRepository func(t *testing.T) *mocks.SubscriptionRepository
		aliasRepository        func(t *testing.T) *mocks.AliasRepository
		taskClient             func(t *testing.T) *mocks.TaskClient
		expectedErr            error
	}{
		{
			name:  "it returns an error when home alias finding fails",
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
			name:  "it returns an error when away alias finding fails",
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
			name:  "it returns an error when match search fails",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find match: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when subscription search fails",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find subscription: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when subscription deletion is not allowed",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&models.Subscription{
					Status: models.SuccessfulSub,
				}, nil).Once()
				return m
			},
			expectedErr: errs.NewUnprocessableContentError(errors.New("not allowed to delete successfully notified subscription")),
		},
		{
			name:  "it returns an error when subscription deletion fails",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
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
			name:  "it returns nil when subscription deleted and other subscriptions listing fails",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
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
			name:  "success - it returns nil when subscription deleted and subscriptions list contains at least one item",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]models.Subscription{{}}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns nil when subscription deleted and match repository deletion fails",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				m.On("Delete", ctx, match.ID).Return(unexpectedErr).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]models.Subscription{}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns nil when subscription deleted and match relation check result task doesn't exist",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&match, nil).Once()
				m.On("Delete", ctx, match.ID).Return(nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]models.Subscription{}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns nil when subscription deleted and check result task deletion fails",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&matchWithTask, nil).Once()
				m.On("Delete", ctx, match.ID).Return(nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]models.Subscription{}, nil).Once()
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
			name:  "success - it deletes subscription and match and check result task",
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
				m.On("One", ctx, models.Match{StartsAt: input.StartsAt.UTC(), HomeTeamID: aliasHome.TeamID, AwayTeamID: aliasAway.TeamID}).Return(&matchWithTask, nil).Once()
				m.On("Delete", ctx, match.ID).Return(nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("One", ctx, match.ID, input.SecretKey, input.BaseURL).Return(&subscription, nil).Once()
				m.On("Delete", ctx, subscription.ID).Return(nil).Once()
				m.On("List", ctx, match.ID).Return([]models.Subscription{}, nil).Once()
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

			ss := sub.NewSubscriptionService(subscriptionRepository, matchRepository, aliasRepository, taskClient, logger)

			err := ss.Delete(ctx, tt.input)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
