package match_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/app/match"
	"github.com/andrewshostak/result-service/internal/app/match/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/testutils"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestResultCheckerService_CheckResult(t *testing.T) {
	pollingInterval := 15 * time.Minute
	pollingFirstAttemptDelay := 115 * time.Minute

	ctx := context.Background()
	unexpectedErr := errors.New("unexpected error")
	matchID := uint(gofakeit.Uint8())
	startsAt := time.Now().Add(2 * time.Hour)

	externalMatchID := uint(gofakeit.Uint32())
	scheduledMatch := testutils.FakeMatch(func(r *models.Match) {
		r.ID = matchID
		r.ResultStatus = models.Scheduled
		r.StartsAt = startsAt
		r.ExternalMatch = &models.ExternalMatch{
			ID:      externalMatchID,
			MatchID: matchID,
		}
		r.CheckResultTask = &models.CheckResultTask{
			AttemptNumber: 1,
		}
	})

	externalMatchClient := testutils.FakeExternalAPIMatch(func(r *models.ExternalAPIMatch) {
		r.ID = int(externalMatchID)
	})

	externalMatchClientFinished := externalMatchClient
	externalMatchClientFinished.Status = models.StatusMatchFinished

	externalMatchClientNotStarted := externalMatchClient
	externalMatchClientNotStarted.Status = models.StatusMatchNotStarted

	externalMatchClientCancelled := externalMatchClient
	externalMatchClientCancelled.Status = models.StatusMatchCancelled

	externalMatchClientInProgress := externalMatchClient
	externalMatchClientInProgress.Status = models.StatusMatchInProgress

	clientTask := testutils.FakeTask()
	repositorySubscription := testutils.FakeSubscription()

	expectedRepositoryMatch := models.ExternalMatch{
		ID:        externalMatchID,
		MatchID:   matchID,
		HomeScore: externalMatchClient.HomeScore,
		AwayScore: externalMatchClient.AwayScore,
		Status:    models.StatusMatchFinished,
	}

	expectedRepositoryMatchFinished := expectedRepositoryMatch
	expectedRepositoryMatchFinished.Status = models.StatusMatchFinished

	expectedRepositoryMatchNotStarted := expectedRepositoryMatch
	expectedRepositoryMatchNotStarted.Status = models.StatusMatchNotStarted

	expectedRepositoryMatchCancelled := expectedRepositoryMatch
	expectedRepositoryMatchCancelled.Status = models.StatusMatchCancelled

	expectedRepositoryMatchInProgress := expectedRepositoryMatch
	expectedRepositoryMatchInProgress.Status = models.StatusMatchInProgress

	tests := []struct {
		name                      string
		input                     uint
		expectedErr               error
		matchRepository           func(t *testing.T) *mocks.MatchRepository
		externalMatchRepository   func(t *testing.T) *mocks.ExternalMatchRepository
		checkResultTaskRepository func(t *testing.T) *mocks.CheckResultTaskRepository
		subscriptionRepository    func(t *testing.T) *mocks.SubscriptionRepository
		externalAPIClient         func(t *testing.T) *mocks.ExternalAPIClient
		taskClient                func(t *testing.T) *mocks.TaskClient
	}{
		{
			name:  "it returns an error when match retrieval fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get match by id: %w", unexpectedErr),
		},
		{
			name:  "success - it doesn't proceed when match status is not scheduled",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				notScheduledMatch := testutils.FakeMatch(func(r *models.Match) {
					r.ID = matchID
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{ID: matchID}).Return(&notScheduledMatch, nil).Once()
				return m
			},
		},
		{
			name:  "it returns an error when match relation with external match does not exist",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				match := testutils.FakeMatch(func(r *models.Match) {
					r.ID = matchID
					r.ResultStatus = models.Scheduled
					r.ExternalMatch = nil
				})
				m.On("One", ctx, models.Match{ID: matchID}).Return(&match, nil).Once()
				return m
			},
			expectedErr: errors.New("match relation external match does not exist"),
		},
		{
			name:  "it returns an error when matches retrieval from external api fails and match update fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, models.APIError).Return(nil, unexpectedErr).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get matches from external api: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when matches retrieval from external api fails and match update succeeds",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				updatedMatch := scheduledMatch
				updatedMatch.ResultStatus = models.APIError
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, models.APIError).Return(&updatedMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get matches from external api: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when external api result doesn't contain expected match",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, mock.Anything).Return([]models.ExternalAPIMatch{
					testutils.FakeExternalAPIMatch(func(r *models.ExternalAPIMatch) {
						r.ID = int(gofakeit.Uint32()) // Different ID
					}),
				}, nil).Once()
				return m
			},
			expectedErr: errors.New("is not found: match not found"),
		},
		{
			name:  "it returns an error when external match saving fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientFinished}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to update external match: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when external match status is cancelled and update fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Cancelled).Return(nil, unexpectedErr).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientCancelled}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchCancelled).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to update result status of match: %w", fmt.Errorf("failed to update result status to %s: %w", "cancelled", unexpectedErr)),
		},
		{
			name:  "it returns nil when external match status is cancelled and update succeeds",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				cancelledMatch := scheduledMatch
				cancelledMatch.ResultStatus = models.Cancelled
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Cancelled).Return(&cancelledMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientCancelled}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchCancelled).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns nil when external match status is not started and update succeeds",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				cancelledMatch := scheduledMatch
				cancelledMatch.ResultStatus = models.Cancelled
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Cancelled).Return(&cancelledMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientNotStarted}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchNotStarted).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns an error when external match status is in progress and match relation with check result task does not exist",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				noCheckResultMatch := scheduledMatch
				noCheckResultMatch.CheckResultTask = nil
				m.On("One", ctx, models.Match{ID: matchID}).Return(&noCheckResultMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientInProgress}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			expectedErr: errors.New("match relation result check task doesn't exist"),
		},
		{
			name:  "it returns an error when external match status is in progress and task re-scheduling fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, models.SchedulingError).Return(&models.Match{}, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientInProgress}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, scheduledMatch.CheckResultTask.AttemptNumber+1, scheduledMatch.StartsAt.Add(pollingFirstAttemptDelay).Add(pollingInterval)).
					Return(nil, unexpectedErr).
					Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to re-schedule result check task: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when external match status is in progress and task re-scheduling fails and match status update fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, models.SchedulingError).Return(nil, unexpectedErr).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientInProgress}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, scheduledMatch.CheckResultTask.AttemptNumber+1, scheduledMatch.StartsAt.Add(pollingFirstAttemptDelay).Add(pollingInterval)).
					Return(nil, unexpectedErr).
					Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to re-schedule result check task: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when external match status is in progress and rescheduled task saving fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientInProgress}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, scheduledMatch.CheckResultTask.AttemptNumber+1, scheduledMatch.StartsAt.Add(pollingFirstAttemptDelay).Add(pollingInterval)).
					Return(&clientTask, nil).
					Once()
				return m
			},
			checkResultTaskRepository: func(t *testing.T) *mocks.CheckResultTaskRepository {
				t.Helper()
				m := mocks.NewCheckResultTaskRepository(t)
				m.On("Save", ctx, models.CheckResultTask{
					MatchID:       matchID,
					Name:          clientTask.Name,
					AttemptNumber: scheduledMatch.CheckResultTask.AttemptNumber + 1,
					ExecuteAt:     clientTask.ExecuteAt,
				}).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to update result check task: %w", unexpectedErr),
		},
		{
			name:  "success - it returns nil when processing external match with status in progress",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientInProgress}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, scheduledMatch.CheckResultTask.AttemptNumber+1, scheduledMatch.StartsAt.Add(pollingFirstAttemptDelay).Add(pollingInterval)).
					Return(&clientTask, nil).
					Once()
				return m
			},
			checkResultTaskRepository: func(t *testing.T) *mocks.CheckResultTaskRepository {
				t.Helper()
				m := mocks.NewCheckResultTaskRepository(t)
				m.On("Save", ctx, models.CheckResultTask{
					MatchID:       matchID,
					Name:          clientTask.Name,
					AttemptNumber: scheduledMatch.CheckResultTask.AttemptNumber + 1,
					ExecuteAt:     clientTask.ExecuteAt,
				}).Return(&models.CheckResultTask{}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns an error when external match status is finished and subscriptions retrieval fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientFinished}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, models.PendingSub).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get subscriptions: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when external match status is finished and match update fails after no subscriptions found",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Received).Return(nil, unexpectedErr).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientFinished}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, models.PendingSub).Return([]models.Subscription{}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to handle finished match: %w", fmt.Errorf("failed to update result status to %s: %w", "received", unexpectedErr)),
		},
		{
			name:  "it returns an error when external match status is finished and creation of subscriber notifier task fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientFinished}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, models.PendingSub).Return([]models.Subscription{repositorySubscription}, nil).Once()
				m.On("Update", ctx, repositorySubscription.ID, models.Subscription{Status: models.SchedulingErrorSub}).Return(nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleSubscriberNotification", ctx, repositorySubscription.ID).Return(unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to schedule subscriber notification: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when external match status is finished and creation of subscriber notifier task fails and subscription update fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientFinished}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, models.PendingSub).Return([]models.Subscription{repositorySubscription}, nil).Once()
				m.On("Update", ctx, repositorySubscription.ID, models.Subscription{Status: models.SchedulingErrorSub}).Return(unexpectedErr).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleSubscriberNotification", ctx, repositorySubscription.ID).Return(unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to schedule subscriber notification: %w", unexpectedErr),
		},
		{
			name:  "success - it returns nil when processing finished external match",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Received).Return(&models.Match{}, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatches", ctx, startsAt).Return([]models.ExternalAPIMatch{externalMatchClientFinished}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&models.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, models.PendingSub).Return([]models.Subscription{repositorySubscription}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleSubscriberNotification", ctx, repositorySubscription.ID).Return(nil).Once()
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

			var externalAPIClient *mocks.ExternalAPIClient
			if tt.externalAPIClient != nil {
				externalAPIClient = tt.externalAPIClient(t)
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

			rcs := match.NewResultCheckerService(
				cfg,
				matchRepository,
				externalMatchRepository,
				subscriptionRepository,
				checkResultTaskRepository,
				taskClient,
				externalAPIClient,
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
