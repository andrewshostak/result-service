package fotmob

import (
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
)

type MatchesResponse struct {
	Leagues []League `json:"leagues"`
}

type League struct {
	Ccode            string  `json:"ccode"`
	Name             string  `json:"name"`
	ParentLeagueName string  `json:"parentLeagueName"`
	Matches          []Match `json:"matches"`
}

type Match struct {
	ID       int    `json:"id"`
	Home     Team   `json:"home"`
	Away     Team   `json:"away"`
	StatusID int    `json:"statusId"`
	Status   Status `json:"status"`
}

type Status struct {
	UTCTime string `json:"utcTime"`
}

type Team struct {
	ID       int    `json:"id"`
	Score    int    `json:"score"`
	Name     string `json:"name"`
	LongName string `json:"longName"`
}

func toDomainExternalAPITeams(response MatchesResponse) []models.ExternalAPITeam {
	teams := make([]models.ExternalAPITeam, 0, len(response.Leagues)*2) // each league has at least one match with two teams
	for _, league := range response.Leagues {
		leagueTeams := make([]models.ExternalAPITeam, 0, len(league.Matches)*2)
		for _, match := range league.Matches {
			homeTeam := models.ExternalAPITeam{
				ID:          match.Home.ID,
				Name:        match.Home.Name,
				LeagueNames: []string{league.Name, league.ParentLeagueName},
				CountryCode: league.Ccode,
			}

			awayTeam := models.ExternalAPITeam{
				ID:          match.Away.ID,
				Name:        match.Away.Name,
				LeagueNames: []string{league.Name, league.ParentLeagueName},
				CountryCode: league.Ccode,
			}

			leagueTeams = append(leagueTeams, homeTeam, awayTeam)
		}

		teams = append(teams, leagueTeams...)
	}

	return teams
}

func toDomainExternalAPIMatches(response MatchesResponse) ([]models.ExternalAPIMatch, error) {
	matches := make([]models.ExternalAPIMatch, 0, len(response.Leagues)) // each league has at least one match
	for _, league := range response.Leagues {
		leagueMatches := make([]models.ExternalAPIMatch, 0, len(league.Matches))
		for _, match := range league.Matches {
			startsAt, err := time.Parse(time.RFC3339, match.Status.UTCTime)
			if err != nil {
				return nil, fmt.Errorf("unable to parse match starting time %s: %w", match.Status.UTCTime, err)
			}

			leagueMatches = append(leagueMatches, models.ExternalAPIMatch{
				ID:        match.ID,
				HomeID:    match.Home.ID,
				AwayID:    match.Away.ID,
				HomeScore: match.Home.Score,
				AwayScore: match.Away.Score,
				Time:      startsAt,
				Status:    toDomainExternalAPIMatchStatus(match.ID, match.StatusID),
			})
		}

		matches = append(matches, leagueMatches...)
	}

	return matches, nil
}

func toDomainExternalAPIMatchStatus(matchID int, statusID int) models.ExternalMatchStatus {
	switch statusID {
	// 1 - Not started
	case 1:
		return models.StatusMatchNotStarted
	// 5 - Postponed
	// 17 - Abandoned
	case 5, 17, 106:
		return models.StatusMatchCancelled
	// 2 - Live 1st half
	// 3 - Live 2nd half
	// 8 - Live extra time 1st half
	// 9 - Live extra time 2nd half
	// 10 - Half-Time
	// 12 - Interrupted (Short)
	// 14 - Waiting for extra time
	// 231 - Pause extra time
	case 2, 3, 8, 9, 10, 12, 14, 231:
		return models.StatusMatchInProgress
	// 6 - Full-Time
	// 11 - After extra time
	// 13 - Finished after Penalties
	case 6, 11, 13:
		return models.StatusMatchFinished
	// 93 - Will be continued after interruption
	// 4, 7, 9, 14, 15, 16
	default:
		fmt.Printf("match with id %d has unknown status status: %d\n", matchID, statusID)
		return models.StatusMatchUnknown
	}
}
