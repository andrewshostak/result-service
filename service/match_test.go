package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/repository"
	"github.com/andrewshostak/result-service/service"
	"github.com/andrewshostak/result-service/service/mocks"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestMatchService_Create(t *testing.T) {
	footballAPIFixtureRepository := mocks.NewExternalMatchRepository(t)
	checkResultTaskRepository := mocks.NewCheckResultTaskRepository(t)
	footballAPIClient := mocks.NewFootballAPIClient(t)
	logger := mocks.NewLogger(t)
	taskClient := mocks.NewTaskClient(t)

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

	startsAt := time.Now().Add(24 * time.Hour)
	createMatchRequest := service.CreateMatchRequest{
		StartsAt:  startsAt,
		AliasHome: aliasHomeName,
		AliasAway: aliasAwayName,
	}

	matchID := uint(gofakeit.Uint8())
	scheduledMatch := fakeRepositoryMatch(func(r *repository.Match) {
		r.ID = matchID
		r.ResultStatus = string(service.Scheduled)
	})

	tests := []struct {
		name            string
		input           service.CreateMatchRequest
		aliasRepository func(t *testing.T) *mocks.AliasRepository
		matchRepository func(t *testing.T) *mocks.MatchRepository
		result          uint
		expectedErr     error
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
			name:  "it returns an error when alias relation FootballApiTeam does not exist",
			input: createMatchRequest,
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHomeName).Return(&repository.Alias{ID: 1, TeamID: 1, Alias: aliasHomeName}, nil).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find home team alias: %w", errors.New(fmt.Sprintf("alias %s found, but there is no releated external team", aliasHomeName))),
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
				}).Return(&scheduledMatch, nil).Once()
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

			ms := service.NewMatchService(
				config.ResultCheck{
					MaxRetries:        0,
					Interval:          pollingInterval,
					FirstAttemptDelay: pollingFirstAttemptDelay,
				},
				aliasRepository,
				matchRepository,
				footballAPIFixtureRepository,
				checkResultTaskRepository,
				footballAPIClient,
				taskClient,
				logger,
			)

			actual, err := ms.Create(ctx, tt.input)
			assert.Equal(t, tt.result, actual)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
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

func fakeExternalMatchRepository(matchID uint) repository.ExternalMatch {
	data := pgtype.JSONB{}
	_ = data.UnmarshalJSON([]byte(externalMatchRaw(4, 2)))

	return repository.ExternalMatch{
		ID:      uint(gofakeit.Uint8()),
		MatchID: matchID,
		Data:    data,
	}
}

func expectedMatch(repositoryMatch repository.Match) service.Match {
	var fixtures []service.FootballAPIFixture
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

	if repositoryMatch.ExternalMatches != nil {
		fixtures = make([]service.FootballAPIFixture, 0, len(repositoryMatch.ExternalMatches))
		for i := range repositoryMatch.ExternalMatches {
			fixtures = append(fixtures, service.FootballAPIFixture{
				ID:   repositoryMatch.ExternalMatches[i].ID,
				Home: 4,
				Away: 2,
			})
		}
	}

	return service.Match{
		ID:                  repositoryMatch.ID,
		StartsAt:            repositoryMatch.StartsAt,
		FootballApiFixtures: fixtures,
		HomeTeam:            homeTeam,
		AwayTeam:            awayTeam,
	}
}

func externalMatchRaw(home, away uint) string {
	return fmt.Sprintf(`{"goals": {"away": %d, "home": %d}, "teams": {"away": {"id": 35, "name": "Bournemouth"}, "home": {"id": 33, "name": "Manchester United"}}, "fixture": {"id": 1035330, "date": "2023-12-09T17:00:00+02:00", "status": {"long": "Match Finished", "short": "FT"}}}`, away, home)
}

type Option[T any] func(*T)

func applyOptions[T any](item *T, updates ...Option[T]) {
	for _, update := range updates {
		update(item)
	}
}
