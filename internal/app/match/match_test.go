package match_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/internal/app/match"
	"github.com/andrewshostak/result-service/internal/app/match/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/testutils"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
)

func TestMatchService_Create(t *testing.T) {
	pollingInterval := 15 * time.Minute
	pollingFirstAttemptDelay := 115 * time.Minute

	ctx := context.Background()
	errUnexpected := errors.New("unexpected error")

	aliasHomeName, aliasAwayName := gofakeit.Word(), gofakeit.Word()
	aliasHome := testutils.FakeAlias(func(r *models.Alias) {
		r.Alias = aliasHomeName
	})
	aliasAway := testutils.FakeAlias(func(r *models.Alias) {
		r.Alias = aliasAwayName
	})

	startsAt := time.Date(time.Now().Year()+1, 12, 11, 16, 30, 0, 0, time.UTC)
	createMatchRequest := models.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: aliasHomeName,
		AliasAway: aliasAwayName,
	}

	matchID := uint(gofakeit.Uint8())
	matchResultScheduled := testutils.FakeMatch(func(r *models.Match) {
		r.ID = matchID
		r.ResultStatus = models.Scheduled
	})
	matchResultReceived := testutils.FakeMatch(func(r *models.Match) {
		r.ID = matchID
		r.ResultStatus = models.Received
	})

	externalMatchID := uint(gofakeit.Uint32())
	externalMatch := testutils.FakeExternalAPIMatch(func(r *models.ExternalAPIMatch) {
		r.ID = int(externalMatchID)
		r.Time = startsAt.UTC()
		r.Home = testutils.FakeExternalAPITeam(func(r *models.ExternalAPITeam) {
			r.ID = int(aliasHome.ExternalTeam.ID)
			r.Score = 0
		})
		r.Away = testutils.FakeExternalAPITeam(func(r *models.ExternalAPITeam) {
			r.ID = int(aliasAway.ExternalTeam.ID)
			r.Score = 0
		})
		r.Status = models.StatusMatchNotStarted
	})

	externalMatchSaved := testutils.FakeExternalMatch(func(r *models.ExternalMatch) {
		r.ID = externalMatchID
		r.MatchID = matchID
		r.HomeScore = externalMatch.Home.Score
		r.AwayScore = externalMatch.Away.Score
		r.Status = models.StatusMatchNotStarted
	})

	createdTaskName := gofakeit.Word()
	clientTask := testutils.FakeClientTask(func(t *models.ClientTask) {
		t.Name = createdTaskName
		t.ExecuteAt = startsAt.Add(pollingFirstAttemptDelay)
	})

	tests := []struct {
		name                      string
		input                     models.CreateMatchRequest
		aliasRepository           func(t *testing.T) *mocks.AliasRepository
		matchRepository           func(t *testing.T) *mocks.MatchRepository
		externalAPIClient         func(t *testing.T) *mocks.ExternalAPIClient
		externalMatchRepository   func(t *testing.T) *mocks.ExternalMatchRepository
		checkResultTaskRepository func(t *testing.T) *mocks.CheckResultTaskRepository
		taskClient                func(t *testing.T) *mocks.TaskClient
		result                    uint
		expectedErr               error
	}{
		{
			name:        "it returns an error when match starting date is not valid",
			input:       models.CreateMatchRequest{StartsAt: time.Now().Add(-1 * time.Hour)},
			expectedErr: errors.New("match starting time must be in the future"),
		},
		{
			name:  "it returns an error when home team alias finding fails",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find home team alias: %w", fmt.Errorf("failed to find team alias: %w", errUnexpected)),
		},
		{
			name:  "it returns an error when away team alias finding fails",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find away team alias: %w", fmt.Errorf("failed to find team alias: %w", errUnexpected)),
		},
		{
			name:  "it returns an error when alias relation with external team does not exist",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&models.Alias{TeamID: 1, Alias: aliasHomeName}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find home team alias: %w", errors.New(fmt.Sprintf("alias %s doesn't have external team relation", aliasHomeName))),
		},
		{
			name:  "it returns an error when match retrieval returns unexpected error",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find match: %w", errUnexpected),
		},
		{
			name:  "success - it returns id of existing and scheduled match",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(&matchResultScheduled, nil).Once()
				return m
			},
			result: matchID,
		},
		{
			name:  "it returns an error if match exists but its status doesn't allow to proceed",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(&matchResultReceived, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("match already exists with result status: %s", matchResultReceived.ResultStatus),
		},
		{
			name:  "it returns an error when matches retrieval from external api fails",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, errs.NewResourceNotFoundError(errors.New("not found"))).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get matches from external api: %w", errUnexpected),
		},
		{
			name:  "it returns an error when external api result doesn't contain expected match",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{
						Matches: []models.ExternalAPIMatch{testutils.FakeExternalAPIMatch(func(r *models.ExternalAPIMatch) {
							r.Home = testutils.FakeExternalAPITeam(func(r *models.ExternalAPITeam) { r.Name = "unexisting name" })
						},
						)}},
				}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("external match with home team id %d and away team id %d is not found: %w", aliasHome.ExternalTeam.ID, aliasAway.ExternalTeam.ID, errors.New("match not found")),
		},
		{
			name:  "it returns an error when external api match has unexpected status",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{Matches: []models.ExternalAPIMatch{
						testutils.FakeExternalAPIMatch(func(r *models.ExternalAPIMatch) {
							r.ID = int(externalMatchID)
							r.Time = startsAt.UTC()
							r.Home = testutils.FakeExternalAPITeam(func(t *models.ExternalAPITeam) {
								t.ID = int(aliasHome.ExternalTeam.ID)
								t.Score = 0
							})
							r.Away = testutils.FakeExternalAPITeam(func(t *models.ExternalAPITeam) {
								t.ID = int(aliasAway.ExternalTeam.ID)
								t.Score = 0
							})
							r.Status = models.StatusMatchUnknown
						}),
					}},
				}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("result check scheduling is not allowed for this match, external match status is %s", models.StatusMatchUnknown),
		},
		{
			name:  "it returns an error when match saving fails",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeam:     &models.Team{ID: aliasHome.TeamID},
					AwayTeam:     &models.Team{ID: aliasAway.TeamID},
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(nil, errUnexpected).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{Matches: []models.ExternalAPIMatch{externalMatch}},
				}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to save match with team ids %d and %d starting at %s: %w", aliasHome.TeamID, aliasAway.TeamID, startsAt.UTC(), errUnexpected),
		},
		{
			name:  "it returns an error when external match saving fails",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				savedMatch := testutils.FakeMatch(func(r *models.Match) {
					r.ID = matchID
					r.HomeTeam = &models.Team{ID: aliasHome.TeamID}
					r.AwayTeam = &models.Team{ID: aliasAway.TeamID}
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeam:     &models.Team{ID: aliasHome.TeamID},
					AwayTeam:     &models.Team{ID: aliasAway.TeamID},
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{Matches: []models.ExternalAPIMatch{externalMatch}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, models.ExternalMatch{
					ID:        externalMatchID,
					MatchID:   matchID,
					HomeScore: externalMatch.Home.Score,
					AwayScore: externalMatch.Away.Score,
					Status:    models.StatusMatchNotStarted,
				}).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to save external match with id %d and match id %d: %w", externalMatchID, matchID, errUnexpected),
		},
		{
			name:  "it returns an error when task scheduling results in unexpected error",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				savedMatch := testutils.FakeMatch(func(r *models.Match) {
					r.ID = matchID
					r.HomeTeam = &models.Team{ID: aliasHome.TeamID}
					r.AwayTeam = &models.Team{ID: aliasAway.TeamID}
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeam:     &models.Team{ID: aliasHome.TeamID},
					AwayTeam:     &models.Team{ID: aliasAway.TeamID},
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{Matches: []models.ExternalAPIMatch{externalMatch}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, models.ExternalMatch{
					ID:        externalMatchID,
					MatchID:   matchID,
					HomeScore: externalMatch.Home.Score,
					AwayScore: externalMatch.Away.Score,
					Status:    models.StatusMatchNotStarted,
				}).Return(&models.ExternalMatch{ID: externalMatchID}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to schedule result check task: %w", errUnexpected),
		},
		{
			name:  "it returns an error when task scheduling results in error indicating that task already exists and then task retrieval fails",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				savedMatch := testutils.FakeMatch(func(r *models.Match) {
					r.ID = matchID
					r.HomeTeam = &models.Team{ID: aliasHome.TeamID}
					r.AwayTeam = &models.Team{ID: aliasAway.TeamID}
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeam:     &models.Team{ID: aliasHome.TeamID},
					AwayTeam:     &models.Team{ID: aliasAway.TeamID},
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{Matches: []models.ExternalAPIMatch{externalMatch}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, models.ExternalMatch{
					ID:        externalMatchID,
					MatchID:   matchID,
					HomeScore: externalMatch.Home.Score,
					AwayScore: externalMatch.Away.Score,
					Status:    models.StatusMatchNotStarted,
				}).Return(&models.ExternalMatch{ID: externalMatchID}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(nil, fmt.Errorf("already exists: %w", errs.NewResourceAlreadyExistsError(errors.New("task exists")))).Once()
				m.On("GetResultCheckTask", ctx, matchID, uint(1)).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get result check task: %w", errUnexpected),
		},
		{
			name:  "it returns an error when check result task saving fails",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				savedMatch := testutils.FakeMatch(func(r *models.Match) {
					r.ID = matchID
					r.HomeTeam = &models.Team{ID: aliasHome.TeamID}
					r.AwayTeam = &models.Team{ID: aliasAway.TeamID}
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeam:     &models.Team{ID: aliasHome.TeamID},
					AwayTeam:     &models.Team{ID: aliasAway.TeamID},
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{Matches: []models.ExternalAPIMatch{externalMatch}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, externalMatchSaved).Return(&models.ExternalMatch{ID: externalMatchID}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(&clientTask, nil).Once()
				return m
			},
			checkResultTaskRepository: func(t *testing.T) *mocks.CheckResultTaskRepository {
				t.Helper()
				m := mocks.NewCheckResultTaskRepository(t)
				m.On("Save", ctx, models.CheckResultTask{
					MatchID:       matchID,
					Name:          createdTaskName,
					AttemptNumber: 1,
					ExecuteAt:     startsAt.Add(pollingFirstAttemptDelay),
				}).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to save result-check task: %w", errUnexpected),
		},
		{
			name:  "it returns an error when match update fails",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				savedMatch := testutils.FakeMatch(func(r *models.Match) {
					r.ID = matchID
					r.HomeTeam = &models.Team{ID: aliasHome.TeamID}
					r.AwayTeam = &models.Team{ID: aliasAway.TeamID}
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeam:     &models.Team{ID: aliasHome.TeamID},
					AwayTeam:     &models.Team{ID: aliasAway.TeamID},
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Scheduled).Return(nil, errUnexpected).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{Matches: []models.ExternalAPIMatch{externalMatch}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, externalMatchSaved).Return(&models.ExternalMatch{ID: externalMatchID}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(&clientTask, nil).Once()
				return m
			},
			checkResultTaskRepository: func(t *testing.T) *mocks.CheckResultTaskRepository {
				t.Helper()
				m := mocks.NewCheckResultTaskRepository(t)
				m.On("Save", ctx, models.CheckResultTask{
					MatchID:       matchID,
					Name:          createdTaskName,
					AttemptNumber: 1,
					ExecuteAt:     startsAt.Add(pollingFirstAttemptDelay),
				}).Return(&models.CheckResultTask{}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to update match status to %s: %w", models.Scheduled, errUnexpected),
		},
		{
			name:  "success - it creates match and schedules result check",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				savedMatch := testutils.FakeMatch(func(r *models.Match) {
					r.ID = matchID
					r.HomeTeam = &models.Team{ID: aliasHome.TeamID}
					r.AwayTeam = &models.Team{ID: aliasAway.TeamID}
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				updatedMatch := savedMatch
				updatedMatch.ResultStatus = models.Scheduled
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeam:     &models.Team{ID: aliasHome.TeamID},
					AwayTeam:     &models.Team{ID: aliasAway.TeamID},
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Scheduled).Return(&updatedMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{Matches: []models.ExternalAPIMatch{externalMatch, testutils.FakeExternalAPIMatch()}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, externalMatchSaved).Return(&models.ExternalMatch{ID: externalMatchID}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(&clientTask, nil).Once()
				return m
			},
			checkResultTaskRepository: func(t *testing.T) *mocks.CheckResultTaskRepository {
				t.Helper()
				m := mocks.NewCheckResultTaskRepository(t)
				m.On("Save", ctx, models.CheckResultTask{
					MatchID:       matchID,
					Name:          createdTaskName,
					AttemptNumber: 1,
					ExecuteAt:     startsAt.Add(pollingFirstAttemptDelay),
				}).Return(&models.CheckResultTask{}, nil).Once()
				return m
			},
			result: matchID,
		},
		{
			name:  "success - it creates match and schedules result check when task already exists",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&aliasHome, nil).Once()
				m.On("Find", ctx, aliasAwayName).Return(&aliasAway, nil).Once()
				return m
			},
			matchRepository: func(t *testing.T) *mocks.MatchRepository {
				t.Helper()
				m := mocks.NewMatchRepository(t)
				savedMatch := testutils.FakeMatch(func(r *models.Match) {
					r.ID = matchID
					r.HomeTeam = &models.Team{ID: aliasHome.TeamID}
					r.AwayTeam = &models.Team{ID: aliasAway.TeamID}
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				updatedMatch := savedMatch
				updatedMatch.ResultStatus = models.Scheduled
				m.On("One", ctx, models.Match{
					HomeTeam: &models.Team{ID: aliasHome.TeamID},
					AwayTeam: &models.Team{ID: aliasAway.TeamID},
					StartsAt: startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeam:     &models.Team{ID: aliasHome.TeamID},
					AwayTeam:     &models.Team{ID: aliasAway.TeamID},
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Scheduled).Return(&updatedMatch, nil).Once()
				return m
			},
			externalAPIClient: func(t *testing.T) *mocks.ExternalAPIClient {
				t.Helper()
				m := mocks.NewExternalAPIClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return([]models.ExternalAPILeague{
					{Matches: []models.ExternalAPIMatch{externalMatch, testutils.FakeExternalAPIMatch()}},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, externalMatchSaved).Return(&models.ExternalMatch{ID: externalMatchID}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(nil, fmt.Errorf("already exists: %w", errs.NewResourceAlreadyExistsError(errors.New("task exists")))).Once()
				m.On("GetResultCheckTask", ctx, matchID, uint(1)).Return(&clientTask, nil).Once()
				return m
			},
			checkResultTaskRepository: func(t *testing.T) *mocks.CheckResultTaskRepository {
				t.Helper()
				m := mocks.NewCheckResultTaskRepository(t)
				m.On("Save", ctx, models.CheckResultTask{
					MatchID:       matchID,
					Name:          createdTaskName,
					AttemptNumber: 1,
					ExecuteAt:     startsAt.Add(pollingFirstAttemptDelay),
				}).Return(&models.CheckResultTask{}, nil).Once()
				return m
			},
			result: matchID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var aliasRepository *mocks.AliasRepository
			if tt.aliasRepository != nil {
				aliasRepository = tt.aliasRepository(t)
			}

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

			logger := loggerinternal.SetupLogger()

			ms := match.NewMatchService(
				config.ResultCheck{
					MaxRetries:        0,
					Interval:          pollingInterval,
					FirstAttemptDelay: pollingFirstAttemptDelay,
				},
				aliasRepository,
				matchRepository,
				externalMatchRepository,
				checkResultTaskRepository,
				externalAPIClient,
				taskClient,
				logger,
			)

			actual, err := ms.Create(ctx, tt.input)
			assert.Equal(t, tt.result, actual)
			if tt.expectedErr != nil {
				assert.ErrorContains(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
