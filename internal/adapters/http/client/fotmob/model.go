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

func toDomainExternalAPIResult(response MatchesResponse) ([]models.ExternalAPILeague, error) {
	leagues := make([]models.ExternalAPILeague, 0, len(response.Leagues))
	for _, v := range response.Leagues {
		matches := make([]models.ExternalAPIMatch, 0, len(v.Matches))

		for _, match := range v.Matches {
			m, err := toDomainExternalAPIMatch(match)
			if err != nil {
				return nil, fmt.Errorf("failed to map to domain match: %w", err)
			}

			matches = append(matches, *m)
		}

		leagues = append(leagues, models.ExternalAPILeague{
			CountryCode:      v.Ccode,
			Name:             v.Name,
			ParentLeagueName: v.ParentLeagueName,
			Matches:          matches,
		})
	}

	return leagues, nil
}

func toDomainExternalAPIMatch(match Match) (*models.ExternalAPIMatch, error) {
	startsAt, err := time.Parse(time.RFC3339, match.Status.UTCTime)
	if err != nil {
		return nil, fmt.Errorf("unable to parse match starting time %s: %w", match.Status.UTCTime, err)
	}

	return &models.ExternalAPIMatch{
		ID:     match.ID,
		Time:   startsAt,
		Home:   toDomainExternalAPITeam(match.Home),
		Away:   toDomainExternalAPITeam(match.Away),
		Status: toDomainExternalAPIMatchStatus(match.ID, match.StatusID),
	}, nil
}

func toDomainExternalAPITeam(team Team) models.ExternalAPITeam {
	return models.ExternalAPITeam{
		ID:    team.ID,
		Score: team.Score,
		Name:  team.Name,
	}
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
