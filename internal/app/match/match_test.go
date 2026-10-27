package match_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/app/match"
	"github.com/andrewshostak/result-service/internal/app/match/mocks"
	"github.com/andrewshostak/result-service/internal/app/models"
	loggerinternal "github.com/andrewshostak/result-service/internal/infra/logger"
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

	providerTeamHomeID := strconv.Itoa(int(gofakeit.Uint32()))
	providerTeamAwayID := strconv.Itoa(int(gofakeit.Uint32()))

	providerTeamHome := models.ProviderTeam{
		TeamID:         aliasHome.TeamID,
		Provider:       "fotmob",
		ProviderTeamID: providerTeamHomeID,
	}
	providerTeamAway := models.ProviderTeam{
		TeamID:         aliasAway.TeamID,
		Provider:       "fotmob",
		ProviderTeamID: providerTeamAwayID,
	}

	startsAt := time.Date(time.Now().Year()+1, 12, 11, 16, 30, 0, 0, time.UTC)
	createMatchRequest := models.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: aliasHomeName,
		AliasAway: aliasAwayName,
	}

	// earlier than startsAt (the provider-resolved time), used to verify the first result-check
	// is scheduled off the earlier of the admin-submitted and provider-reported kickoff time.
	adminStartsAt := startsAt.Add(-45 * time.Minute)

	matchID := uint(gofakeit.Uint8())
	matchResultScheduled := testutils.FakeMatch(func(r *models.Match) {
		r.ID = matchID
		r.ResultStatus = models.Scheduled
	})
	matchResultReceived := testutils.FakeMatch(func(r *models.Match) {
		r.ID = matchID
		r.ResultStatus = models.Received
	})

	providerAPIMatchID := uint(gofakeit.Uint32())
	providerAPIMatch := testutils.FakeProviderAPIMatch(func(r *models.ProviderAPIMatch) {
		r.ID = providerAPIMatchID
		r.Time = startsAt.UTC()
		r.HomeID = providerTeamHomeID
		r.HomeScore = 0
		r.AwayID = providerTeamAwayID
		r.AwayScore = 0
		r.Status = models.StatusMatchNotStarted
	})

	providerMatchSaved := providerAPIMatch.ToProviderMatch(matchID, "fotmob")

	createdTaskName := gofakeit.Word()
	clientTask := testutils.FakeTask(func(t *models.Task) {
		t.Name = createdTaskName
		t.ExecuteAt = startsAt.Add(pollingFirstAttemptDelay)
	})

	tests := []struct {
		name                      string
		input                     models.CreateMatchRequest
		aliasRepository           func(t *testing.T) *mocks.AliasRepository
		matchRepository           func(t *testing.T) *mocks.MatchRepository
		providerTeamRepository    func(t *testing.T) *mocks.ProviderTeamRepository
		resultProvider            func(t *testing.T) *mocks.ResultProvider
		providerMatchRepository   func(t *testing.T) *mocks.ProviderMatchRepository
		checkResultTaskRepository func(t *testing.T) *mocks.CheckResultTaskRepository
		taskClient                func(t *testing.T) *mocks.TaskClient
		result                    uint
		expectedErr               error
	}{
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
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
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
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
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
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(&matchResultReceived, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("match already exists with result status: %s", matchResultReceived.ResultStatus),
		},
		{
			name:  "it returns an error when home provider team is not found",
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
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, models.NewResourceNotFoundError(errors.New("not found"))).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(nil, models.NewResourceNotFoundError(errors.New("not found"))).Once()
				return m
			},
			expectedErr: fmt.Errorf("provider team for home team id %d not found", aliasHome.TeamID),
		},
		{
			name:  "it returns an error when away provider team is not found",
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
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, models.NewResourceNotFoundError(errors.New("not found"))).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(nil, models.NewResourceNotFoundError(errors.New("not found"))).Once()
				return m
			},
			expectedErr: fmt.Errorf("provider team for away team id %d not found", aliasAway.TeamID),
		},
		{
			name:  "it returns an error when matches retrieval from provider api fails",
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
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, models.NewResourceNotFoundError(errors.New("not found"))).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return(nil, errUnexpected).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to get matches from provider api: %w", fmt.Errorf("failed to get matches across providers: %w", fmt.Errorf("failed to get matches from provider fotmob: %w", errUnexpected))),
		},
		{
			name:  "it returns an error when provider api result doesn't contain expected match",
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
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return(
					[]models.ProviderAPIMatch{testutils.FakeProviderAPIMatch(func(r *models.ProviderAPIMatch) {
						r.HomeID = "123123123"
					}),
					}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			expectedErr: fmt.Errorf("provider match with home provider team id %s and away provider team id %s is not found: %w", providerTeamHomeID, providerTeamAwayID, errors.New("match not found")),
		},
		{
			name:  "it returns an error when provider api match has unexpected status",
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
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return(
					[]models.ProviderAPIMatch{
						testutils.FakeProviderAPIMatch(func(r *models.ProviderAPIMatch) {
							r.ID = providerAPIMatchID
							r.Time = startsAt.UTC()
							r.HomeID = providerTeamHomeID
							r.AwayID = providerTeamAwayID
							r.HomeScore = 0
							r.AwayScore = 0
							r.Status = models.StatusMatchUnknown
						}),
					}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			expectedErr: fmt.Errorf("result check scheduling is not allowed for this match, provider match status is %s", models.StatusMatchUnknown),
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
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(nil, errUnexpected).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return([]models.ProviderAPIMatch{providerAPIMatch}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to save match with team ids %d and %d starting at %s: %w", aliasHome.TeamID, aliasAway.TeamID, startsAt.UTC(), errUnexpected),
		},
		{
			name:  "it returns an error when provider match saving fails",
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
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return([]models.ProviderAPIMatch{providerAPIMatch}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			providerMatchRepository: func(t *testing.T) *mocks.ProviderMatchRepository {
				t.Helper()
				m := mocks.NewProviderMatchRepository(t)
				m.On("Save", ctx, providerMatchSaved).Return(nil, errUnexpected).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to save provider match with id %d and match id %d: %w", providerAPIMatchID, matchID, errUnexpected),
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
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return([]models.ProviderAPIMatch{providerAPIMatch}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			providerMatchRepository: func(t *testing.T) *mocks.ProviderMatchRepository {
				t.Helper()
				m := mocks.NewProviderMatchRepository(t)
				m.On("Save", ctx, providerMatchSaved).Return(&models.ProviderMatch{}, nil).Once()
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
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return([]models.ProviderAPIMatch{providerAPIMatch}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			providerMatchRepository: func(t *testing.T) *mocks.ProviderMatchRepository {
				t.Helper()
				m := mocks.NewProviderMatchRepository(t)
				m.On("Save", ctx, providerMatchSaved).Return(&models.ProviderMatch{}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(nil, fmt.Errorf("already exists: %w", models.NewResourceAlreadyExistsError(errors.New("task exists")))).Once()
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
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return([]models.ProviderAPIMatch{providerAPIMatch}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			providerMatchRepository: func(t *testing.T) *mocks.ProviderMatchRepository {
				t.Helper()
				m := mocks.NewProviderMatchRepository(t)
				m.On("Save", ctx, providerMatchSaved).Return(&models.ProviderMatch{}, nil).Once()
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
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				m.On("One", ctx, models.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Match{ResultStatus: models.Scheduled}).Return(nil, errUnexpected).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return([]models.ProviderAPIMatch{providerAPIMatch}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			providerMatchRepository: func(t *testing.T) *mocks.ProviderMatchRepository {
				t.Helper()
				m := mocks.NewProviderMatchRepository(t)
				m.On("Save", ctx, providerMatchSaved).Return(&models.ProviderMatch{}, nil).Once()
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
			name: "success - it creates match and schedules result check off the earlier of admin and provider time",
			input: models.CreateMatchRequest{
				StartsAt:  adminStartsAt,
				AliasHome: aliasHomeName,
				AliasAway: aliasAwayName,
			},
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
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				updatedMatch := savedMatch
				updatedMatch.ResultStatus = models.Scheduled
				m.On("One", ctx, models.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   adminStartsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Match{ResultStatus: models.Scheduled}).Return(&updatedMatch, nil).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, adminStartsAt.UTC()).Return([]models.ProviderAPIMatch{providerAPIMatch, testutils.FakeProviderAPIMatch()}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			providerMatchRepository: func(t *testing.T) *mocks.ProviderMatchRepository {
				t.Helper()
				m := mocks.NewProviderMatchRepository(t)
				m.On("Save", ctx, providerMatchSaved).Return(&models.ProviderMatch{}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				// providerAPIMatch.Time is startsAt (later); adminStartsAt is earlier, so the
				// scheduling call must use adminStartsAt, proving earliestOf picked it.
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), adminStartsAt.Add(pollingFirstAttemptDelay)).Return(&clientTask, nil).Once()
				return m
			},
			checkResultTaskRepository: func(t *testing.T) *mocks.CheckResultTaskRepository {
				t.Helper()
				m := mocks.NewCheckResultTaskRepository(t)
				m.On("Save", ctx, models.CheckResultTask{
					MatchID:       matchID,
					Name:          createdTaskName,
					AttemptNumber: 1,
					ExecuteAt:     clientTask.ExecuteAt,
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
					r.HomeTeamID = aliasHome.TeamID
					r.AwayTeamID = aliasAway.TeamID
					r.StartsAt = startsAt.UTC()
					r.ResultStatus = models.NotScheduled
				})
				updatedMatch := savedMatch
				updatedMatch.ResultStatus = models.Scheduled
				m.On("One", ctx, models.Match{
					HomeTeamID: aliasHome.TeamID,
					AwayTeamID: aliasAway.TeamID,
					StartsAt:   startsAt.UTC(),
				}).Return(nil, fmt.Errorf("match not found: %w", models.NewResourceNotFoundError(errors.New("not found")))).Once()
				m.On("Save", ctx, (*uint)(nil), models.Match{
					HomeTeamID:   aliasHome.TeamID,
					AwayTeamID:   aliasAway.TeamID,
					StartsAt:     startsAt.UTC(),
					ResultStatus: models.NotScheduled,
				}).Return(&savedMatch, nil).Once()
				m.On("Update", ctx, matchID, models.Match{ResultStatus: models.Scheduled}).Return(&updatedMatch, nil).Once()
				return m
			},
			providerTeamRepository: func(t *testing.T) *mocks.ProviderTeamRepository {
				t.Helper()
				m := mocks.NewProviderTeamRepository(t)
				m.On("FindByTeamAndProvider", ctx, aliasHome.TeamID, "fotmob").Return(&providerTeamHome, nil).Once()
				m.On("FindByTeamAndProvider", ctx, aliasAway.TeamID, "fotmob").Return(&providerTeamAway, nil).Once()
				return m
			},
			resultProvider: func(t *testing.T) *mocks.ResultProvider {
				t.Helper()
				m := mocks.NewResultProvider(t)
				m.On("GetMatches", ctx, startsAt.UTC()).Return([]models.ProviderAPIMatch{providerAPIMatch, testutils.FakeProviderAPIMatch()}, nil).Once()
				m.On("Name").Return("fotmob").Once()
				return m
			},
			providerMatchRepository: func(t *testing.T) *mocks.ProviderMatchRepository {
				t.Helper()
				m := mocks.NewProviderMatchRepository(t)
				m.On("Save", ctx, providerMatchSaved).Return(&models.ProviderMatch{}, nil).Once()
				return m
			},
			taskClient: func(t *testing.T) *mocks.TaskClient {
				t.Helper()
				m := mocks.NewTaskClient(t)
				m.On("ScheduleResultCheck", ctx, matchID, uint(1), startsAt.Add(pollingFirstAttemptDelay)).Return(nil, fmt.Errorf("already exists: %w", models.NewResourceAlreadyExistsError(errors.New("task exists")))).Once()
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

			var providerTeamRepository *mocks.ProviderTeamRepository
			if tt.providerTeamRepository != nil {
				providerTeamRepository = tt.providerTeamRepository(t)
			}

			var resultProvider *mocks.ResultProvider
			var providers []match.ResultProvider
			if tt.resultProvider != nil {
				resultProvider = tt.resultProvider(t)
				providers = []match.ResultProvider{resultProvider}
			}

			var providerMatchRepository *mocks.ProviderMatchRepository
			if tt.providerMatchRepository != nil {
				providerMatchRepository = tt.providerMatchRepository(t)
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
				providerTeamRepository,
				providerMatchRepository,
				checkResultTaskRepository,
				providers,
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
