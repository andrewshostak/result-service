package testutils

import (
	"testing"

	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func SetupTeamWithRelations(t *testing.T, db *sqlx.DB, alias string, externalTeamID int) (uint, repository.ExternalTeam) {
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
	query := "INSERT INTO subscriptions (match_id, url, key) VALUES ($1, $2, $3) RETURNING *"

	err := db.Get(&created, query, subscription.MatchID, subscription.Url, subscription.Key)
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
