package testutils

import (
	"testing"

	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func CreateTeams(t *testing.T, db *sqlx.DB) []int {
	t.Helper()

	var teamIDs []int
	teamsCreationQuery := "INSERT INTO teams (id) VALUES (default), (default), (default) RETURNING id"

	err := db.Select(&teamIDs, teamsCreationQuery)
	require.NoError(t, err)
	require.Equal(t, 3, len(teamIDs))

	return teamIDs
}

func CreateExternalTeams(t *testing.T, db *sqlx.DB, teamID int, externalTeamID int) repository.ExternalTeam {
	t.Helper()

	var created repository.ExternalTeam
	externalTeamCreationQuery := "INSERT INTO external_teams (id, team_id) VALUES ($1, $2) RETURNING *"

	err := db.Get(&created, externalTeamCreationQuery, externalTeamID, teamID)
	require.NoError(t, err)

	return created
}

func CreateAlias(t *testing.T, db *sqlx.DB, alias string, teamID int) {
	t.Helper()

	aliasCreationQuery := "INSERT INTO aliases (team_id, alias) VALUES ($1, $2)"

	_, err := db.Exec(aliasCreationQuery, teamID, alias)
	require.NoError(t, err)
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

func ListSubscriptionsByMatch(t *testing.T, db *sqlx.DB, matchID uint) []repository.Subscription {
	t.Helper()

	var subs []repository.Subscription

	err := db.Select(&subs, "SELECT * FROM subscriptions WHERE match_id = $1", matchID)
	require.NoError(t, err)

	return subs
}
