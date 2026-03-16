package testutils

import (
	"testing"
	"time"

	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func SetupTeamsWithRelations(t *testing.T, db *sqlx.DB) []TeamSeed {
	t.Helper()

	aliases := []string{"Arsenal", "Barcelona", "Juventus"}

	teamSeeds := make([]TeamSeed, 0, len(aliases))
	for i := range aliases {
		teamID, externalTeam := SetupTeamWithRelations(t, db, aliases[i], i+1)

		teamSeeds = append(teamSeeds, TeamSeed{
			TeamID:         teamID,
			ExternalTeamID: externalTeam.ID,
			Alias:          aliases[i],
		})
	}

	return teamSeeds
}

func SetupTeamWithRelations(t *testing.T, db *sqlx.DB, alias string, externalTeamID int) (uint, repository.ExternalTeam) {
	t.Helper()

	teamID := CreateTeam(t, db)
	CreateAlias(t, db, alias, teamID)
	externalTeam := CreateExternalTeam(t, db, teamID, externalTeamID)
	return teamID, externalTeam
}

func CreateAlias(t *testing.T, db *sqlx.DB, alias string, teamID uint) {
	t.Helper()

	aliasCreationQuery := "INSERT INTO aliases (team_id, alias) VALUES ($1, $2)"

	_, err := db.Exec(aliasCreationQuery, teamID, alias)
	require.NoError(t, err)
}

func CreateExternalTeam(t *testing.T, db *sqlx.DB, teamID uint, externalTeamID int) repository.ExternalTeam {
	t.Helper()

	var created repository.ExternalTeam
	externalTeamCreationQuery := "INSERT INTO external_teams (id, team_id) VALUES ($1, $2) RETURNING *"

	err := db.Get(&created, externalTeamCreationQuery, externalTeamID, teamID)
	require.NoError(t, err)

	return created
}

func CreateExternalMatch(t *testing.T, db *sqlx.DB, externalMatch repository.ExternalMatch) repository.ExternalMatch {
	t.Helper()

	var created repository.ExternalMatch
	externalMatchCreationQuery := "INSERT INTO external_matches (match_id, status, home_score, away_score) VALUES ($1, $2, $3, $4) RETURNING *"

	err := db.Get(&created, externalMatchCreationQuery, externalMatch.MatchID, externalMatch.Status, externalMatch.HomeScore, externalMatch.AwayScore)
	require.NoError(t, err)

	return created
}

func CreateCheckResultTask(t *testing.T, db *sqlx.DB, matchID uint, name string, executeAt time.Time) repository.CheckResultTask {
	t.Helper()

	var created repository.CheckResultTask
	query := "INSERT INTO check_result_tasks (match_id, name, execute_at) VALUES ($1, $2, $3) RETURNING *"

	err := db.Get(&created, query, matchID, name, executeAt)
	require.NoError(t, err)

	return created
}

func CreateMatch(t *testing.T, db *sqlx.DB, match repository.Match) repository.Match {
	t.Helper()

	var created repository.Match
	query := "INSERT INTO matches (home_team_id, away_team_id, starts_at, result_status) VALUES ($1, $2, $3, $4) RETURNING *"

	err := db.Get(&created, query, match.HomeTeamID, match.AwayTeamID, match.StartsAt, match.ResultStatus)
	require.NoError(t, err)

	return created
}

func CreateSubscription(t *testing.T, db *sqlx.DB, subscription repository.Subscription) repository.Subscription {
	t.Helper()

	var created repository.Subscription
	query := "INSERT INTO subscriptions (match_id, url, key, status) VALUES ($1, $2, $3, $4) RETURNING *"

	err := db.Get(&created, query, subscription.MatchID, subscription.Url, subscription.Key, subscription.Status)
	require.NoError(t, err)

	return created
}

func CreateTeam(t *testing.T, db *sqlx.DB) uint {
	t.Helper()

	var teamID []uint
	teamsCreationQuery := "INSERT INTO teams (id) VALUES (default) RETURNING id"

	err := db.Select(&teamID, teamsCreationQuery)
	require.NoError(t, err)

	return teamID[0]
}

func ListCheckResultTasks(t *testing.T, db *sqlx.DB) []repository.CheckResultTask {
	t.Helper()

	var checkResultTasks []repository.CheckResultTask

	err := db.Select(&checkResultTasks, "SELECT * FROM check_result_tasks")
	require.NoError(t, err)

	return checkResultTasks
}

func ListExternalMatches(t *testing.T, db *sqlx.DB) []repository.ExternalMatch {
	t.Helper()

	var externalMatches []repository.ExternalMatch

	err := db.Select(&externalMatches, "SELECT * FROM external_matches")
	require.NoError(t, err)

	return externalMatches
}

func ListMatches(t *testing.T, db *sqlx.DB) []repository.Match {
	t.Helper()

	var matches []repository.Match

	err := db.Select(&matches, "SELECT * FROM matches")
	require.NoError(t, err)

	return matches
}

func ListSubscriptionsByMatch(t *testing.T, db *sqlx.DB, matchID uint) []repository.Subscription {
	t.Helper()

	var subs []repository.Subscription

	err := db.Select(&subs, "SELECT * FROM subscriptions WHERE match_id = $1", matchID)
	require.NoError(t, err)

	return subs
}

type TeamSeed struct {
	TeamID         uint
	ExternalTeamID uint
	Alias          string
}
