package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/config"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/repository"
	"github.com/andrewshostak/result-service/service"
	"github.com/andrewshostak/result-service/service/mocks"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
)

func TestResultCheckerService_CheckResult(t *testing.T) {
	pollingInterval := 15 * time.Minute
	pollingFirstAttemptDelay := 115 * time.Minute

	ctx := context.Background()
	unexpectedErr := errors.New("unexpected error")
	matchID := uint(gofakeit.Uint8())

	tests := []struct {
		name                      string
		input                     uint
		expectedErr               error
		matchRepository           func(t *testing.T) *mocks.MatchRepository
		externalMatchRepository   func(t *testing.T) *mocks.ExternalMatchRepository
		checkResultTaskRepository func(t *testing.T) *mocks.CheckResultTaskRepository
		subscriptionRepository    func(t *testing.T) *mocks.SubscriptionRepository
		fotmobClient              func(t *testing.T) *mocks.FotmobClient
		taskClient                func(t *testing.T) *mocks.TaskClient
	}{
		{
			name:  "it returns error when match is not found",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get match by id: %w", unexpectedErr),
		},
		{
			name:  "it returns nil when match is not scheduled",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				notScheduledMatch := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.ResultStatus = string(service.NotScheduled)
				})
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&notScheduledMatch, nil).Once()
				return m
			},
		},
		{
			name:  "it returns error when ExternalMatch relation is nil",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				scheduledMatch := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.ResultStatus = string(service.Scheduled)
					r.ExternalMatch = nil
				})
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			expectedErr: errors.New("match doesn't have external match"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var matchRepository *mocks.MatchRepository
			if tt.matchRepository != nil {
				matchRepository = tt.matchRepository(t)
			}

			var fotmobClient *mocks.FotmobClient
			if tt.fotmobClient != nil {
				fotmobClient = tt.fotmobClient(t)
			}

			var externalMatchRepository *mocks.ExternalMatchRepository
			if tt.externalMatchRepository != nil {
				externalMatchRepository = tt.externalMatchRepository(t)
			}

			var checkResultTaskRepository *mocks.CheckResultTaskRepository
			if tt.checkResultTaskRepository != nil {
				checkResultTaskRepository = tt.checkResultTaskRepository(t)
			}

			var taskClient *mocks.TaskClient
			if tt.taskClient != nil {
				taskClient = tt.taskClient(t)
			}

			var subscriptionRepository *mocks.SubscriptionRepository
			if tt.subscriptionRepository != nil {
				subscriptionRepository = tt.subscriptionRepository(t)
			}

			logger := loggerinternal.SetupLogger()

			cfg := config.ResultCheck{
				MaxRetries:        0,
				Interval:          pollingInterval,
				FirstAttemptDelay: pollingFirstAttemptDelay,
			}

			rcs := service.NewResultCheckerService(
				cfg,
				matchRepository,
				externalMatchRepository,
				subscriptionRepository,
				checkResultTaskRepository,
				taskClient,
				fotmobClient,
				logger,
			)

			err := rcs.CheckResult(ctx, tt.input)
			if tt.expectedErr != nil {
				assert.ErrorContains(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
