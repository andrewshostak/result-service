package fotmob

import (
	"fmt"
	"strconv"
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
	ID       uint              `json:"id"`
	Home     Team              `json:"home"`
	Away     Team              `json:"away"`
	StatusID fotmobMatchStatus `json:"statusId"`
	Status   Status            `json:"status"`
}

type Status struct {
	UTCTime   string  `json:"utcTime"`
	Started   bool    `json:"started"`
	Finished  bool    `json:"finished"`
	Cancelled bool    `json:"cancelled"`
	Awarded   bool    `json:"awarded"`
	Reason    *Reason `json:"reason"`
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
	notStarted fotmobMatchStatus = 1
	// Match is delayed (hasn't started). It is not yet clear whether it will be played today. Known status change path: delayed => postponed.
	delayed fotmobMatchStatus = 25
	// It won't be played today, but it's not yet clear when it will be played.
	postponed fotmobMatchStatus = 5
	// A real case: match was interrupted, rescheduled to the next day, but didn't start the next day because of another unexpected reason, and the new date is not yet clear.
	abandoned fotmobMatchStatus = 17
	// Canceled forever.
	cancelled fotmobMatchStatus = 106
	// Planned to continue after the interruption. A real case: a match played and interrupted yesterday had this status the next day until it was resumed.
	resumptionPlanned    fotmobMatchStatus = 93
	live1stHalf          fotmobMatchStatus = 2
	live2ndHalf          fotmobMatchStatus = 3
	liveExtraTime1stHalf fotmobMatchStatus = 8
	liveExtraTime2ndHalf fotmobMatchStatus = 9
	halfTime             fotmobMatchStatus = 10
	// Match has already started but was interrupted. It is not clear when the match will resume (it could be postponed to tomorrow).
	interruptedShort    fotmobMatchStatus = 12
	waitingForExtraTime fotmobMatchStatus = 14
	pauseExtraTime      fotmobMatchStatus = 231
	fullTime            fotmobMatchStatus = 6
	afterExtraTime      fotmobMatchStatus = 11
	afterPenalties      fotmobMatchStatus = 13
)

func toDomainProviderAPITeams(response MatchesResponse) []models.ProviderAPITeam {
	teams := make([]models.ProviderAPITeam, 0, len(response.Leagues)*2) // each league has at least one match with two teams
	for _, league := range response.Leagues {
		leagueTeams := make([]models.ProviderAPITeam, 0, len(league.Matches)*2)
		for _, match := range league.Matches {
			homeTeam := models.ProviderAPITeam{
				ID:          match.Home.ID,
				Name:        match.Home.Name,
				LeagueNames: []string{league.Name, league.ParentLeagueName},
				CountryCode: league.Ccode,
			}

			awayTeam := models.ProviderAPITeam{
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

func toDomainProviderAPIMatches(response MatchesResponse) ([]models.ProviderAPIMatch, error) {
	matches := make([]models.ProviderAPIMatch, 0, len(response.Leagues)) // each league has at least one match
	for _, league := range response.Leagues {
		leagueMatches := make([]models.ProviderAPIMatch, 0, len(league.Matches))
		for _, match := range league.Matches {
			startsAt, err := time.Parse(time.RFC3339, match.Status.UTCTime)
			if err != nil {
				return nil, fmt.Errorf("unable to parse match starting time %s: %w", match.Status.UTCTime, err)
			}

			rawStatus := fmt.Sprintf("%d", match.StatusID)
			statusDetails := fmt.Sprintf("started=%t finished=%t cancelled=%t awarded=%t", match.Status.Started, match.Status.Finished, match.Status.Cancelled, match.Status.Awarded)
			if match.Status.Reason != nil {
				statusDetails += fmt.Sprintf(" reason_short=%s reason_short_key=%s reason_long=%s reason_long_key=%s", match.Status.Reason.Short, match.Status.Reason.ShortKey, match.Status.Reason.Long, match.Status.Reason.LongKey)
			}

			leagueMatches = append(leagueMatches, models.ProviderAPIMatch{
				ID:                    match.ID,
				HomeID:                strconv.Itoa(match.Home.ID),
				AwayID:                strconv.Itoa(match.Away.ID),
				HomeScore:             match.Home.Score,
				AwayScore:             match.Away.Score,
				Time:                  startsAt,
				Status:                ToDomainProviderMatchStatus(match.StatusID),
				ProviderStatus:        &rawStatus,
				ProviderStatusDetails: &statusDetails,
			})
		}

		matches = append(matches, leagueMatches...)
	}

	return matches, nil
}

func ToDomainProviderMatchStatus(statusID fotmobMatchStatus) models.ProviderMatchStatus {
	switch statusID {
	case notStarted, delayed, postponed, resumptionPlanned:
		return models.StatusMatchNotStarted
	case abandoned, cancelled:
		return models.StatusMatchCancelled
	// 4 - [NOT CLEAR] received from a match 4935228 which has penalties without extra time
	// 20 - [NOT CLEAR] received from a match 4935225 which has penalties without extra time
	case live1stHalf, live2ndHalf, 4, liveExtraTime1stHalf, liveExtraTime2ndHalf, halfTime, interruptedShort, waitingForExtraTime, 20, pauseExtraTime:
		return models.StatusMatchInProgress
	case fullTime, afterExtraTime, afterPenalties:
		return models.StatusMatchFinished
	// 7, 9, 15, 16, 234 - [NOT CLEAR]
	default:
		return models.StatusMatchUnknown
	}
}
