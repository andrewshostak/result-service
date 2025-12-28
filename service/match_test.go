package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/client"
	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/errs"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/repository"
	"github.com/andrewshostak/result-service/service"
	"github.com/andrewshostak/result-service/service/mocks"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
)

func TestMatchService_Create(t *testing.T) {
	pollingInterval := 15 * time.Minute
	pollingFirstAttemptDelay := 115 * time.Minute

	ctx := context.Background()
	errUnexpected := errors.New("unexpected error")

	aliasHomeName, aliasAwayName := gofakeit.Word(), gofakeit.Word()
	aliasHome := fakeRepositoryAlias(func(r *repository.Alias) {
		r.Alias = aliasHomeName
	})
	aliasAway := fakeRepositoryAlias(func(r *repository.Alias) {
		r.Alias = aliasAwayName
	})

	startsAt := time.Date(time.Now().Year()+1, 12, 11, 16, 30, 0, 0, time.UTC)
	createMatchRequest := service.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: aliasHomeName,
		AliasAway: aliasAwayName,
	}

	matchID := uint(gofakeit.Uint8())
	matchResultScheduled := fakeRepositoryMatch(func(r *repository.Match) {
		r.ID = matchID
		r.ResultStatus = string(service.Scheduled)
	})
	matchResultReceived := fakeRepositoryMatch(func(r *repository.Match) {
		r.ID = matchID
		r.ResultStatus = string(service.Received)
	})

	externalMatchID := uint(gofakeit.Uint32())
	externalMatch := fakeClientMatch(func(r *client.MatchFotmob) {
		r.ID = int(externalMatchID)
		r.Status.UTCTime = startsAt.UTC().Format(time.RFC3339)
		r.Home = fakeClientTeam(func(r *client.TeamFotmob) {
			r.ID = int(aliasHome.ExternalTeam.ID)
			r.Score = 0
		})
		r.Away = fakeClientTeam(func(r *client.TeamFotmob) {
			r.ID = int(aliasAway.ExternalTeam.ID)
			r.Score = 0
		})
	})

	externalMatchSaved := fakeExternalMatchRepository(func(r *repository.ExternalMatch) {
		r.ID = externalMatchID
		r.MatchID = matchID
		r.HomeScore = externalMatch.Home.Score
		r.AwayScore = externalMatch.Away.Score
		r.Status = string(service.StatusMatchNotStarted)
	})

	createdTaskName := gofakeit.Word()
	clientTask := fakeClientTask(func(t *client.Task) {
		t.Name = createdTaskName
		t.ExecuteAt = startsAt.Add(pollingFirstAttemptDelay)
	})

	tests := []struct {
		name                      string
		input                     service.CreateMatchRequest
		aliasRepository           func(t *testing.T) *mocks.AliasRepository
		matchRepository           func(t *testing.T) *mocks.MatchRepository
		fotmobClient              func(t *testing.T) *mocks.FotmobClient
		externalMatchRepository   func(t *testing.T) *mocks.ExternalMatchRepository
		checkResultTaskRepository func(t *testing.T) *mocks.CheckResultTaskRepository
		taskClient                func(t *testing.T) *mocks.TaskClient
		result                    uint
		expectedErr               error
	}{
		{
			name:        "it returns an error when match starting date is in the past",
			input:       service.CreateMatchRequest{StartsAt: time.Now().Add(-1 * time.Hour)},
			expectedErr: errors.New("match starting time must be in the future"),
		},
		{
			name:  "it returns an error when home team alias does not exist",
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
			name:  "it returns an error when away team alias does not exist",
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
			name:  "it returns an error when alias relation ExternalTeam does not exist",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&repository.Alias{ID: 1, TeamID: 1, Alias: aliasHomeName}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find home team alias: %w", errors.New(fmt.Sprintf("alias %s doesn't have external team relation", aliasHomeName))),
		},
		{
			name:  "it returns error when unexpected error from match repository method returned",
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
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("unexpected error when getting a match: %w", errUnexpected),
		},
		{
			name:  "it returns id if match already exists and is scheduled",
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
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(&matchResultScheduled, nil).Once()
				return m
			},
			result: matchID,
		},
		{
			name:  "it returns error if match already exists and has not not_scheduled status",
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
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(&matchResultReceived, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("match already exists with result status: %s", matchResultReceived.ResultStatus),
		},
		{
			name:  "it returns error when external api client method returns unexpected error",
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
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get matches from external api: %w", errUnexpected),
		},
		{
			name:  "it returns an error when fails to map external api client result",
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
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{
							Matches: []client.MatchFotmob{fakeClientMatch(func(r *client.MatchFotmob) { r.Status = client.StatusFotmob{UTCTime: "invalid time"} })},
						},
					},
				}, nil).Once()
				return m
			},
			expectedErr: errors.New("failed to map from external api matches: failed to map from client match: unable to parse match starting time invalid time"),
		},
		{
			name:  "it returns an error when there is no match in the response from external api",
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
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{fakeClientMatch(func(r *client.MatchFotmob) {
							r.Home = fakeClientTeam(func(r *client.TeamFotmob) { r.Name = "unexisting name" })
						})}},
					},
				}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("external match with home team id %d and away team id %d is not found: %w", aliasHome.ExternalTeam.ID, aliasAway.ExternalTeam.ID, errors.New("match not found")),
		},
		{
			name:  "it returns error when result scheduling not allowed",
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
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{
							fakeClientMatch(func(r *client.MatchFotmob) {
								r.ID = int(externalMatchID)
								r.Status.UTCTime = startsAt.UTC().Format(time.RFC3339)
								r.Home = fakeClientTeam(func(t *client.TeamFotmob) {
									t.ID = int(aliasHome.ExternalTeam.ID)
									t.Score = 0
								})
								r.Away = fakeClientTeam(func(t *client.TeamFotmob) {
									t.ID = int(aliasAway.ExternalTeam.ID)
									t.Score = 0
								})
								r.StatusID = 111
							}),
						}},
					},
				}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("result check scheduling is not allowed for this match, external match status is %s", service.StatusMatchUnknown),
		},
		{
			name:  "it returns error when match repository save fails",
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
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				m.On("Save", ctx, (*uint)(nil), repository.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: string(service.NotScheduled),
				}).Return(nil, errUnexpected).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{externalMatch}},
					},
				}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to save match with team ids %d and %d starting at %s: %w", aliasHome.TeamID, aliasAway.TeamID, startsAt.UTC(), errUnexpected),
		},
		{
			name:  "it returns error when external match repository save fails",
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
				savedMatch := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = string(service.NotScheduled)
				})
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				m.On("Save", ctx, (*uint)(nil), repository.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: string(service.NotScheduled),
				}).Return(&savedMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{externalMatch}},
					},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, repository.ExternalMatch{
					ID:        externalMatchID,
					MatchID:   matchID,
					HomeScore: externalMatch.Home.Score,
					AwayScore: externalMatch.Away.Score,
					Status:    string(service.StatusMatchNotStarted),
				}).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to save external match with id %d and match id %d: %w", externalMatchID, matchID, errUnexpected),
		},
		{
			name:  "it returns error when task client ScheduleResultCheck returns unexpected error",
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
				savedMatch := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = string(service.NotScheduled)
				})
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				m.On("Save", ctx, (*uint)(nil), repository.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: string(service.NotScheduled),
				}).Return(&savedMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{externalMatch}},
					},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, repository.ExternalMatch{
					ID:        externalMatchID,
					MatchID:   matchID,
					HomeScore: externalMatch.Home.Score,
					AwayScore: externalMatch.Away.Score,
					Status:    string(service.StatusMatchNotStarted),
				}).Return(&repository.ExternalMatch{ID: externalMatchID}, nil).Once()
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
			name:  "it returns error when task client ScheduleResultCheck returns error indicating task already exists and GetResultCheckTask returns error",
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
				savedMatch := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = string(service.NotScheduled)
				})
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				m.On("Save", ctx, (*uint)(nil), repository.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: string(service.NotScheduled),
				}).Return(&savedMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{externalMatch}},
					},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, repository.ExternalMatch{
					ID:        externalMatchID,
					MatchID:   matchID,
					HomeScore: externalMatch.Home.Score,
					AwayScore: externalMatch.Away.Score,
					Status:    string(service.StatusMatchNotStarted),
				}).Return(&repository.ExternalMatch{ID: externalMatchID}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(nil, fmt.Errorf("already exists: %w", errs.ClientTaskAlreadyExistsError{Message: "task exists"})).Once()
				m.On("GetResultCheckTask", ctx, matchID, uint(1)).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get result check task: %w", errUnexpected),
		},
		{
			name:  "it returns error when check result task repository Save returns unexpected error",
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
				savedMatch := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = string(service.NotScheduled)
				})
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				m.On("Save", ctx, (*uint)(nil), repository.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: string(service.NotScheduled),
				}).Return(&savedMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{externalMatch}},
					},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, externalMatchSaved).Return(&repository.ExternalMatch{ID: externalMatchID}, nil).Once()
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
				m.On("Save", ctx, repository.CheckResultTask{
					MatchID:       matchID,
					Name:          createdTaskName,
					ExecuteAt:     startsAt.Add(pollingFirstAttemptDelay),
					AttemptNumber: 1,
				}).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to save result-check task: %w", errUnexpected),
		},
		{
			name:  "it returns error when match repository Update returns error",
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
				savedMatch := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = string(service.NotScheduled)
				})
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				m.On("Save", ctx, (*uint)(nil), repository.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: string(service.NotScheduled),
				}).Return(&savedMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.Scheduled)).Return(nil, errUnexpected).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{externalMatch}},
					},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, externalMatchSaved).Return(&repository.ExternalMatch{ID: externalMatchID}, nil).Once()
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
				m.On("Save", ctx, repository.CheckResultTask{
					MatchID:       matchID,
					Name:          createdTaskName,
					ExecuteAt:     startsAt.Add(pollingFirstAttemptDelay),
					AttemptNumber: 1,
				}).Return(&repository.CheckResultTask{}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to set match status to %s: %w", service.Scheduled, errUnexpected),
		},
		{
			name:  "it successfully creates match and schedules result check",
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
				savedMatch := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = string(service.NotScheduled)
				})
				updatedMatch := savedMatch
				updatedMatch.ResultStatus = string(service.Scheduled)
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				m.On("Save", ctx, (*uint)(nil), repository.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: string(service.NotScheduled),
				}).Return(&savedMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.Scheduled)).Return(&updatedMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{externalMatch, fakeClientMatch()}},
					},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, externalMatchSaved).Return(&repository.ExternalMatch{ID: externalMatchID}, nil).Once()
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
				m.On("Save", ctx, repository.CheckResultTask{
					MatchID:       matchID,
					Name:          createdTaskName,
					ExecuteAt:     startsAt.Add(pollingFirstAttemptDelay),
					AttemptNumber: 1,
				}).Return(&repository.CheckResultTask{}, nil).Once()
				return m
			},
			result: matchID,
		},
		{
			name:  "it successfully creates match and schedules result check - when task already exists",
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
				savedMatch := fakeRepositoryMatch(func(r *repository.Match) {
					r.ID = matchID
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = string(service.NotScheduled)
				})
				updatedMatch := savedMatch
				updatedMatch.ResultStatus = string(service.Scheduled)
				m.On("One", ctx, repository.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", errs.MatchNotFoundError{Message: "not found"})).Once()
				m.On("Save", ctx, (*uint)(nil), repository.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: string(service.NotScheduled),
				}).Return(&savedMatch, nil).Once()
				m.On("Update", ctx, matchID, string(service.Scheduled)).Return(&updatedMatch, nil).Once()
				return m
			},
			fotmobClient: func(t *testing.T) *mocks.FotmobClient {
				t.Helper()
				m := mocks.NewFotmobClient(t)
				m.On("GetMatchesByDate", ctx, startsAt.UTC()).Return(&client.MatchesResponse{
					Leagues: []client.LeagueFotmob{
						{Matches: []client.MatchFotmob{externalMatch, fakeClientMatch()}},
					},
				}, nil).Once()
				return m
			},
			externalMatchRepository: func(t *testing.T) *mocks.ExternalMatchRepository {
				t.Helper()
				m := mocks.NewExternalMatchRepository(t)
				m.On("Save", ctx, &externalMatchID, externalMatchSaved).Return(&repository.ExternalMatch{ID: externalMatchID}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(nil, fmt.Errorf("already exists: %w", errs.ClientTaskAlreadyExistsError{Message: "task exists"})).Once()
				m.On("GetResultCheckTask", ctx, matchID, uint(1)).Return(&clientTask, nil).Once()
				return m
			},
			checkResultTaskRepository: func(t *testing.T) *mocks.CheckResultTaskRepository {
				t.Helper()
				m := mocks.NewCheckResultTaskRepository(t)
				m.On("Save", ctx, repository.CheckResultTask{
					MatchID:       matchID,
					Name:          createdTaskName,
					ExecuteAt:     startsAt.Add(pollingFirstAttemptDelay),
					AttemptNumber: 1,
				}).Return(&repository.CheckResultTask{}, nil).Once()
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

			logger := loggerinternal.SetupLogger()

			ms := service.NewMatchService(
				config.ResultCheck{
					MaxRetries:        0,
					Interval:          pollingInterval,
					FirstAttemptDelay: pollingFirstAttemptDelay,
				},
				aliasRepository,
				matchRepository,
				externalMatchRepository,
				checkResultTaskRepository,
				fotmobClient,
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

func fakeRepositoryMatch(options ...Option[repository.Match]) repository.Match {
	statuses := []service.ResultStatus{
		service.NotScheduled,
		service.Scheduled,
		service.SchedulingError,
		service.Received,
		service.APIError,
		service.Cancelled,
	}

	m := repository.Match{
		ID:           uint(gofakeit.Uint8()),
		HomeTeamID:   uint(gofakeit.Uint8()),
		AwayTeamID:   uint(gofakeit.Uint8()),
		StartsAt:     gofakeit.Date(),
		ResultStatus: string(statuses[gofakeit.IntRange(0, len(statuses)-1)]),
	}

	applyOptions(&m, options...)

	return m
}

func fakeRepositoryTeam(teamID uint, aliases bool) repository.Team {
	var a []repository.Alias

	if aliases {
		fakeAlias := fakeRepositoryAlias(func(r *repository.Alias) {
			r.TeamID = teamID
		})
		a = []repository.Alias{fakeAlias}
	}

	return repository.Team{
		ID:      teamID,
		Aliases: a,
	}
}

func fakeRepositoryAlias(options ...Option[repository.Alias]) repository.Alias {
	alias := repository.Alias{
		ID:           uint(gofakeit.Uint8()),
		TeamID:       uint(gofakeit.Uint8()),
		Alias:        gofakeit.Name(),
		ExternalTeam: &repository.ExternalTeam{},
	}

	applyOptions(&alias, options...)

	return alias
}

func fakeClientMatch(options ...Option[client.MatchFotmob]) client.MatchFotmob {
	match := client.MatchFotmob{
		ID:       int(gofakeit.Int8()),
		Home:     fakeClientTeam(),
		Away:     fakeClientTeam(),
		StatusID: 1,
		Status: client.StatusFotmob{
			UTCTime: gofakeit.Date().Format(time.RFC3339),
		},
	}

	applyOptions(&match, options...)

	return match
}

func fakeClientTeam(options ...Option[client.TeamFotmob]) client.TeamFotmob {
	team := client.TeamFotmob{
		ID:       int(gofakeit.Int8()),
		Score:    gofakeit.IntRange(0, 9),
		Name:     gofakeit.Name(),
		LongName: gofakeit.Name(),
	}

	applyOptions(&team, options...)

	return team
}

func fakeClientTask(options ...Option[client.Task]) client.Task {
	task := client.Task{
		Name:      gofakeit.Name(),
		ExecuteAt: time.Now().Add(time.Duration(gofakeit.RandomInt([]int{1, 2, 4, 8})) * time.Hour),
	}

	applyOptions(&task, options...)

	return task
}

func fakeExternalMatchRepository(options ...Option[repository.ExternalMatch]) repository.ExternalMatch {
	externalMatch := repository.ExternalMatch{
		ID:        uint(gofakeit.Uint8()),
		MatchID:   uint(gofakeit.Uint8()),
		HomeScore: gofakeit.IntRange(0, 9),
		AwayScore: gofakeit.IntRange(0, 9),
		Status:    gofakeit.RandomString([]string{"not_started", "cancelled", "in_progress", "finished", "unknown"}),
	}

	applyOptions(&externalMatch, options...)

	return externalMatch
}

func expectedMatch(repositoryMatch repository.Match) service.Match {
	var externalMatch *service.ExternalMatchDB
	var homeTeam *service.Team
	var awayTeam *service.Team

	if repositoryMatch.HomeTeam != nil {
		var aliases []service.Alias

		if repositoryMatch.HomeTeam.Aliases != nil {
			aliases = make([]service.Alias, 0, len(repositoryMatch.HomeTeam.Aliases))
			for i := range repositoryMatch.HomeTeam.Aliases {
				aliases = append(aliases, service.Alias{
					Alias:  repositoryMatch.HomeTeam.Aliases[i].Alias,
					TeamID: repositoryMatch.HomeTeam.Aliases[i].TeamID,
				})
			}
		}

		homeTeam = &service.Team{
			ID:      repositoryMatch.HomeTeam.ID,
			Aliases: aliases,
		}
	}

	if repositoryMatch.AwayTeam != nil {
		var aliases []service.Alias

		if repositoryMatch.AwayTeam.Aliases != nil {
			aliases = make([]service.Alias, 0, len(repositoryMatch.AwayTeam.Aliases))
			for i := range repositoryMatch.AwayTeam.Aliases {
				aliases = append(aliases, service.Alias{
					Alias:  repositoryMatch.AwayTeam.Aliases[i].Alias,
					TeamID: repositoryMatch.AwayTeam.Aliases[i].TeamID,
				})
			}
		}

		awayTeam = &service.Team{
			ID:      repositoryMatch.AwayTeam.ID,
			Aliases: aliases,
		}
	}

	if repositoryMatch.ExternalMatch != nil {
		externalMatch = &service.ExternalMatchDB{
			ID:        repositoryMatch.ExternalMatch.ID,
			MatchID:   repositoryMatch.ExternalMatch.MatchID,
			HomeScore: repositoryMatch.ExternalMatch.HomeScore,
			AwayScore: repositoryMatch.ExternalMatch.AwayScore,
			Status:    service.ExternalMatchStatus(repositoryMatch.ExternalMatch.Status),
		}
	}

	return service.Match{
		ID:            repositoryMatch.ID,
		StartsAt:      repositoryMatch.StartsAt,
		ExternalMatch: externalMatch,
		HomeTeam:      homeTeam,
		AwayTeam:      awayTeam,
	}
}

type Option[T any] func(*T)

func applyOptions[T any](item *T, updates ...Option[T]) {
	for _, update := range updates {
		update(item)
	}
}
