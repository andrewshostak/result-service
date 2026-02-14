package fotmob

import (
	"fmt"
	"slices"
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
	ID       int               `json:"id"`
	Home     Team              `json:"home"`
	Away     Team              `json:"away"`
	StatusID fotmobMatchStatus `json:"statusId"`
	Status   Status            `json:"status"`
}

type Status struct {
	UTCTime string `json:"utcTime"`
	Reason  Reason `json:"reason"`
}

type Reason struct {
	Short    string `json:"short"`
	ShortKey string `json:"shortKey"`
	Long     string `json:"long"`
	LongKey  string `json:"longKey"`
}

type Team struct {
	ID       int    `json:"id"`
	Score    int    `json:"score"`
	Name     string `json:"name"`
	LongName string `json:"longName"`
}

type fotmobMatchStatus int

const (
	notStarted           fotmobMatchStatus = 1
	postponed            fotmobMatchStatus = 5
	abandoned            fotmobMatchStatus = 17
	live1stHalf          fotmobMatchStatus = 2
	live2ndHalf          fotmobMatchStatus = 3
	liveExtraTime1stHalf fotmobMatchStatus = 8
	liveExtraTime2ndHalf fotmobMatchStatus = 9
	halfTime             fotmobMatchStatus = 10
	interruptedShort     fotmobMatchStatus = 12
	waitingForExtraTime  fotmobMatchStatus = 14
	pauseExtraTime       fotmobMatchStatus = 231
	fullTime             fotmobMatchStatus = 6
	afterExtraTime       fotmobMatchStatus = 11
	afterPenalties       fotmobMatchStatus = 13
)

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
				Status:    ToDomainExternalAPIMatchStatus(match.ID, match.StatusID),
			})

			if isUnknownStatus(match.StatusID) {
				fmt.Printf(
					"match with id %d has status %d. reason - short %s, short key %s, long %s, long key %s",
					match.ID,
					match.StatusID,
					match.Status.Reason.Short,
					match.Status.Reason.ShortKey,
					match.Status.Reason.Long,
					match.Status.Reason.LongKey)
			}
		}

		matches = append(matches, leagueMatches...)
	}

	return matches, nil
}

func ToDomainExternalAPIMatchStatus(matchID int, statusID fotmobMatchStatus) models.ExternalMatchStatus {
	switch statusID {
	case notStarted:
		return models.StatusMatchNotStarted
	// 106 - [NOT CLEAR]
	case postponed, abandoned, 106:
		return models.StatusMatchCancelled
	// 4 - [NOT CLEAR] received from a match 4935228 which has penalties without extra time
	// 20 - [NOT CLEAR] received from a match 4935225 which has penalties without extra time
	case live1stHalf, live2ndHalf, 4, liveExtraTime1stHalf, liveExtraTime2ndHalf, halfTime, interruptedShort, waitingForExtraTime, 20, pauseExtraTime:
		return models.StatusMatchInProgress
	case fullTime, afterExtraTime, afterPenalties:
		return models.StatusMatchFinished
	// 93 - [NOT CLEAR] Will be continued after interruption ?
	// 7, 9, 15, 16 - [NOT CLEAR]
	default:
		fmt.Printf("match with id %d has unknown status: %d\n", matchID, statusID)
		return models.StatusMatchUnknown
	}
}

func isUnknownStatus(statusID fotmobMatchStatus) bool {
	return !slices.Contains([]fotmobMatchStatus{
		notStarted,
		postponed,
		abandoned,
		live1stHalf,
		live2ndHalf,
		liveExtraTime1stHalf,
		liveExtraTime2ndHalf,
		halfTime,
		interruptedShort,
		waitingForExtraTime,
		pauseExtraTime,
		fullTime,
		afterExtraTime,
		afterPenalties,
	}, statusID)
}
