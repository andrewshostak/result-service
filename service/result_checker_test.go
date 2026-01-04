package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/client"
	"github.com/andrewshostak/result-service/config"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/repository"
	"github.com/andrewshostak/result-service/service"
	"github.com/andrewshostak/result-service/service/mocks"
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
	scheduledMatch := fakeRepositoryMatch(func(r *repository.Match) {
		r.ID = matchID
		r.ResultStatus = string(service.Scheduled)
		r.StartsAt = startsAt
		r.ExternalMatch = &repository.ExternalMatch{
			ID:      externalMatchID,
			MatchID: matchID,
		}
		r.CheckResultTask = &repository.CheckResultTask{
			AttemptNumber: 1,
		}
	})

	externalMatchClient := fakeClientMatch(func(r *client.MatchFotmob) {
		r.ID = int(externalMatchID)
	})

	externalMatchClientFinished := externalMatchClient
	externalMatchClientFinished.StatusID = 6

	externalMatchClientNotStarted := externalMatchClient
	externalMatchClientNotStarted.StatusID = 1

	externalMatchClientCancelled := externalMatchClient
	externalMatchClientCancelled.StatusID = 5

	externalMatchClientInProgress := externalMatchClient
	externalMatchClientInProgress.StatusID = 2

	clientTask := fakeClientTask()
	repositorySubscription := fakeRepositorySubscription()

	expectedRepositoryMatch := repository.ExternalMatch{
		ID:        externalMatchID,
		MatchID:   matchID,
		HomeScore: externalMatchClient.Home.Score,
		AwayScore: externalMatchClient.Away.Score,
		Status:    string(service.StatusMatchFinished),
	}

	expectedRepositoryMatchFinished := expectedRepositoryMatch
	expectedRepositoryMatchFinished.Status = string(service.StatusMatchFinished)

	expectedRepositoryMatchNotStarted := expectedRepositoryMatch
	expectedRepositoryMatchNotStarted.Status = string(service.StatusMatchNotStarted)

	expectedRepositoryMatchCancelled := expectedRepositoryMatch
	expectedRepositoryMatchCancelled.Status = string(service.StatusMatchCancelled)

	expectedRepositoryMatchInProgress := expectedRepositoryMatch
	expectedRepositoryMatchInProgress.Status = string(service.StatusMatchInProgress)

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
			name:  "it returns an error when match retrieval fails",
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
			name:  "success - it doesn't proceed when match status is not scheduled",
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
			name:  "it returns an error when match relation with external match does not exist",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				match := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.ResultStatus = string(service.Scheduled)
					r.ExternalMatch = nil
				})
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&match, nil).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.APIError)).Return(nil, unexpectedErr).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(nil, unexpectedErr).Once()
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
				updatedMatch.ResultStatus = string(service.APIError)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.APIError)).Return(&updatedMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(nil, unexpectedErr).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get matches from external api: %w", unexpectedErr),
		},
		{
			name:  "it returns an error when mapping from external api result fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, mock.Anything).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{
							Matches: []client.MatchFotmob{
								fakeClientMatch(func(r *client.MatchFotmob) {
									r.Status = client.StatusFotmob{UTCTime: "invalid time"}
								}),
							},
						},
					},
				}, nil).Once()
				return m
			},
			expectedErr: errors.New("failed to map from external api matches: failed to map from client match: unable to parse match starting time invalid time"),
		},
		{
			name:  "it returns an error when external api result doesn't contain expected match",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, mock.Anything).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{
							Matches: []client.MatchFotmob{
								fakeClientMatch(func(r *client.MatchFotmob) {
									r.ID = int(gofakeit.Uint32()) // Different ID
								}),
							},
						},
					},
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientFinished}}},
				}, nil).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.Cancelled)).Return(nil, unexpectedErr).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientCancelled}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchCancelled).Return(&repository.ExternalMatch{}, nil).Once()
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
				cancelledMatch.ResultStatus = string(service.Cancelled)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.Cancelled)).Return(&cancelledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientCancelled}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchCancelled).Return(&repository.ExternalMatch{}, nil).Once()
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
				cancelledMatch.ResultStatus = string(service.Cancelled)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.Cancelled)).Return(&cancelledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientNotStarted}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchNotStarted).Return(&repository.ExternalMatch{}, nil).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&noCheckResultMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientInProgress}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&repository.ExternalMatch{}, nil).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.SchedulingError)).Return(&repository.Match{}, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientInProgress}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&repository.ExternalMatch{}, nil).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.SchedulingError)).Return(nil, unexpectedErr).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientInProgress}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&repository.ExternalMatch{}, nil).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientInProgress}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&repository.ExternalMatch{}, nil).Once()
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
				m.On("Save", ctx, repository.CheckResultTask{
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientInProgress}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchInProgress).Return(&repository.ExternalMatch{}, nil).Once()
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
				m.On("Save", ctx, repository.CheckResultTask{
					MatchID:       matchID,
					Name:          clientTask.Name,
					AttemptNumber: scheduledMatch.CheckResultTask.AttemptNumber + 1,
					ExecuteAt:     clientTask.ExecuteAt,
				}).Return(&repository.CheckResultTask{}, nil).Once()
				return m
			},
		},
		{
			name:  "it returns an error when external match status is finished and subscriptions retrieval fails",
			input: matchID,
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientFinished}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&repository.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, string(service.PendingSub)).Return(nil, unexpectedErr).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.Received)).Return(nil, unexpectedErr).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientFinished}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&repository.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, string(service.PendingSub)).Return([]repository.Subscription{}, nil).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientFinished}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&repository.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, string(service.PendingSub)).Return([]repository.Subscription{repositorySubscription}, nil).Once()
				m.On("Update", ctx, repositorySubscription.ID, repository.Subscription{Status: string(service.SchedulingErrorSub)}).Return(nil).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientFinished}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&repository.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, string(service.PendingSub)).Return([]repository.Subscription{repositorySubscription}, nil).Once()
				m.On("Update", ctx, repositorySubscription.ID, repository.Subscription{Status: string(service.SchedulingErrorSub)}).Return(unexpectedErr).Once()
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
				m.On("One", ctx, repository.Match{ID: matchID}).Return(&scheduledMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.Received)).Return(&repository.Match{}, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{{Matches: []client.MatchFotmob{externalMatchClientFinished}}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, expectedRepositoryMatchFinished).Return(&repository.ExternalMatch{}, nil).Once()
				return m
			},
			subscriptionRepository: func(t *testing.T) *mocks.SubscriptionRepository {
				t.Helper()
				m := mocks.NewSubscriptionRepository(t)
				m.On("ListByMatchAndStatus", ctx, matchID, string(service.PendingSub)).Return([]repository.Subscription{repositorySubscription}, nil).Once()
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

func fakeRepositorySubscription(options ...Option[repository.Subscription]) repository.Subscription {
	statuses := []service.SubscriptionStatus{
		service.PendingSub,
		service.SchedulingErrorSub,
		service.SuccessfulSub,
		service.SubscriberErrorSub,
	}

	notifiedAt := gofakeit.Date()

	sub := repository.Subscription{
		ID:         uint(gofakeit.Uint8()),
		Url:        gofakeit.URL(),
		MatchID:    uint(gofakeit.Uint8()),
		Key:        gofakeit.Password(true, true, true, false, false, 10),
		Status:     string(statuses[gofakeit.IntRange(0, len(statuses)-1)]),
		NotifiedAt: &notifiedAt,
	}

	applyOptions(&sub, options...)

	return sub
}
