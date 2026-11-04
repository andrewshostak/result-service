package fotmob

import (
	"fmt"
	"slices"
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
	UTCTime string  `json:"utcTime"`
	Reason  *Reason `json:"reason"`
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
	// match is delayed (hasn't started). it is not yet clear whether it will be played today or not. known status change path delayed => postponed
	delayed fotmobMatchStatus = 25
	// it won't be played today, but not yet clear when it will be played
	postponed fotmobMatchStatus = 5
	// a real case: match was interrupted, scheduled to the next day, and haven't started the next day because of another unexpected reason, and new date is not clear yet
	abandoned fotmobMatchStatus = 17
	// canceled forever
	cancelled fotmobMatchStatus = 106
	// planned to continue after the interruption. a real case: match played and interrupted yesterday had this status the next day until it was resumed
	resumptionPlanned    fotmobMatchStatus = 93 // TODO: use in switch
	live1stHalf          fotmobMatchStatus = 2
	live2ndHalf          fotmobMatchStatus = 3
	liveExtraTime1stHalf fotmobMatchStatus = 8
	liveExtraTime2ndHalf fotmobMatchStatus = 9
	halfTime             fotmobMatchStatus = 10
	// match already started, but was interrupted. it is not clear when the match will resume. (could be postponed to tomorrow)
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
			leagueMatches = append(leagueMatches, models.ProviderAPIMatch{
				ID:             match.ID,
				HomeID:         strconv.Itoa(match.Home.ID),
				AwayID:         strconv.Itoa(match.Away.ID),
				HomeScore:      match.Home.Score,
				AwayScore:      match.Away.Score,
				Time:           startsAt,
				Status:         ToDomainProviderMatchStatus(match.ID, match.StatusID),
				ProviderStatus: &rawStatus,
			})

			if isUnknownStatus(match.StatusID) && match.Status.Reason != nil {
				// do not need two logs, here, and in ToDomainProviderMatchStatus default
				fmt.Printf(
					"match with id %d has status %d. reason - short %s, short key %s, long %s, long key %s \n",
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

func ToDomainProviderMatchStatus(matchID uint, statusID fotmobMatchStatus) models.ProviderMatchStatus {
	switch statusID {
	case notStarted, delayed, postponed:
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
		fmt.Printf("match with id %d has unknown status: %d\n", matchID, statusID)
		return models.StatusMatchUnknown
	}
}

func isUnknownStatus(statusID fotmobMatchStatus) bool {
	return !slices.Contains([]fotmobMatchStatus{
		notStarted,
		postponed,
		abandoned,
		cancelled,
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
