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
	matchRepository := mocks.NewMatchRepository(t)
	footballAPIFixtureRepository := mocks.NewFootballAPIFixtureRepository(t)
	checkResultTaskRepository := mocks.NewCheckResultTaskRepository(t)
	footballAPIClient := mocks.NewFootballAPIClient(t)
	logger := mocks.NewLogger(t)
	taskClient := mocks.NewTaskClient(t)

	pollingInterval := 15 * time.Minute
	pollingFirstAttemptDelay := 115 * time.Minute

	ctx := context.Background()
	aliasHome, aliasAway := gofakeit.Name(), gofakeit.Name()

	tests := []struct {
		name            string
		input           service.CreateMatchRequest
		aliasRepository func(t *testing.T) *mocks.AliasRepository
		result          uint
		expectedErr     error
	}{
		{
			name:        "it returns an error when match starting date is in the past",
			input:       service.CreateMatchRequest{StartsAt: time.Now().Add(-1 * time.Hour)},
			expectedErr: errors.New("match starting time must be in the future"),
		},
		{
			name: "it returns an error when home team alias does not exist",
			input: service.CreateMatchRequest{
				StartsAt:  time.Now().Add(24 * time.Hour),
				AliasHome: aliasHome,
			},
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHome).Return(nil, errors.New("not found")).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find home team alias: %w", fmt.Errorf("failed to find team alias: %w", errors.New("not found"))),
		},
		{
			name: "it returns an error when away team alias does not exist",
			input: service.CreateMatchRequest{
				StartsAt:  time.Now().Add(24 * time.Hour),
				AliasHome: aliasHome,
				AliasAway: aliasAway,
			},
			aliasRepository: func(t *testing.T) *mocks.AliasRepository {
				t.Helper()
				m := mocks.NewAliasRepository(t)
				m.On("Find", ctx, aliasHome).Return(&repository.Alias{ID: 1, TeamID: 1, Alias: aliasHome, FootballApiTeam: &repository.FootballApiTeam{}}, nil).Once()
				m.On("Find", ctx, aliasAway).Return(nil, errors.New("not found")).Once()
				return m
			},
			expectedErr: fmt.Errorf("failed to find away team alias: %w", fmt.Errorf("failed to find team alias: %w", errors.New("not found"))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var aliasRepository *mocks.AliasRepository
			if tt.aliasRepository != nil {
				aliasRepository = tt.aliasRepository(t)
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
			assert.EqualError(t, err, tt.expectedErr.Error())
		})
	}
}

func fakeRepositoryMatch(teams bool, fixtures bool) repository.Match {
	matchID := uint(gofakeit.Uint8())
	homeTeamID := uint(gofakeit.Uint8())
	awayTeamID := uint(gofakeit.Uint8())

	var homeTeam *repository.Team
	var awayTeam *repository.Team
	var apiFixtures []repository.FootballApiFixture

	if teams {
		fakeTeam1, fakeTeam2 := fakeRepositoryTeam(homeTeamID, true), fakeRepositoryTeam(awayTeamID, true)
		homeTeam = &fakeTeam1
		awayTeam = &fakeTeam2
	}

	if fixtures {
		fakeFixture := fakeFootballAPIRepositoryFixture(matchID)
		apiFixtures = []repository.FootballApiFixture{fakeFixture}
	}

	return repository.Match{
		ID:                  matchID,
		HomeTeamID:          homeTeamID,
		AwayTeamID:          awayTeamID,
		StartsAt:            gofakeit.Date(),
		ResultStatus:        string(service.Scheduled),
		FootballApiFixtures: apiFixtures,
		HomeTeam:            homeTeam,
		AwayTeam:            awayTeam,
	}
}

func fakeRepositoryTeam(teamID uint, aliases bool) repository.Team {
	var a []repository.Alias

	if aliases {
		fakeAlias := fakeRepositoryAlias(teamID)
		a = []repository.Alias{fakeAlias}
	}

	return repository.Team{
		ID:      teamID,
		Aliases: a,
	}
}

func fakeRepositoryAlias(teamID uint) repository.Alias {
	return repository.Alias{
		ID:              uint(gofakeit.Uint8()),
		TeamID:          teamID,
		Alias:           gofakeit.Name(),
		FootballApiTeam: nil,
	}
}

func fakeFootballAPIRepositoryFixture(matchID uint) repository.FootballApiFixture {
	data := pgtype.JSONB{}
	_ = data.UnmarshalJSON([]byte(footballAPIFixtureRaw(4, 2)))

	return repository.FootballApiFixture{
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

	if repositoryMatch.FootballApiFixtures != nil {
		fixtures = make([]service.FootballAPIFixture, 0, len(repositoryMatch.FootballApiFixtures))
		for i := range repositoryMatch.FootballApiFixtures {
			fixtures = append(fixtures, service.FootballAPIFixture{
				ID:   repositoryMatch.FootballApiFixtures[i].ID,
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

func footballAPIFixtureRaw(home, away uint) string {
	return fmt.Sprintf(`{"goals": {"away": %d, "home": %d}, "teams": {"away": {"id": 35, "name": "Bournemouth"}, "home": {"id": 33, "name": "Manchester United"}}, "fixture": {"id": 1035330, "date": "2023-12-09T17:00:00+02:00", "status": {"long": "Match Finished", "short": "FT"}}}`, away, home)
}
