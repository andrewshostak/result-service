package service

import (
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/client"
	"github.com/andrewshostak/result-service/repository"
)

type CreateMatchRequest struct {
	StartsAt  time.Time
	AliasHome string
	AliasAway string
}

type CreateSubscriptionRequest struct {
	MatchID   uint
	URL       string
	SecretKey string
}

type DeleteSubscriptionRequest struct {
	StartsAt  time.Time
	AliasHome string
	AliasAway string
	BaseURL   string
	SecretKey string
}

type Team struct {
	ID uint

	Aliases []Alias
}

type ExternalTeam struct {
	ID     uint
	TeamID uint
}

type ResultStatus string

const (
	NotScheduled    ResultStatus = "not_scheduled"
	Scheduled       ResultStatus = "scheduled"
	SchedulingError ResultStatus = "scheduling_error"
	Received        ResultStatus = "received"
	APIError        ResultStatus = "api_error"
	Cancelled       ResultStatus = "cancelled"
)

type Match struct {
	ID           uint
	StartsAt     time.Time
	ResultStatus ResultStatus

	ExternalMatch   *ExternalMatch
	CheckResultTask *CheckResultTask
	HomeTeam        *Team
	AwayTeam        *Team
}

type Alias struct {
	Alias  string
	TeamID uint

	ExternalTeam *ExternalTeam
}

type SubscriptionStatus string

const (
	PendingSub         SubscriptionStatus = "pending"
	SchedulingErrorSub SubscriptionStatus = "scheduling_error"
	SuccessfulSub      SubscriptionStatus = "successful"
	SubscriberErrorSub SubscriptionStatus = "subscriber_error"
)

type Subscription struct {
	ID         uint
	Url        string
	MatchID    uint
	Key        string
	CreatedAt  time.Time
	Status     SubscriptionStatus
	NotifiedAt *time.Time

	Match *Match
}

type ExternalMatchStatus string

const (
	StatusMatchNotStarted ExternalMatchStatus = "not_started"
	StatusMatchCancelled  ExternalMatchStatus = "cancelled"
	StatusMatchInProgress ExternalMatchStatus = "in_progress"
	StatusMatchFinished   ExternalMatchStatus = "finished"
	StatusMatchUnknown    ExternalMatchStatus = "unknown"
)

type ExternalMatch struct {
	ID        uint
	MatchID   uint
	HomeScore int
	AwayScore int
	Status    ExternalMatchStatus
}

type CheckResultTask struct {
	ID            uint
	MatchID       uint
	Name          string
	AttemptNumber uint
}

type ExternalMatchesResponse struct {
	Leagues []ExternalAPILeague
}

type ExternalAPILeague struct {
	CountryCode      string
	Name             string
	ParentLeagueName string
	Matches          []ExternalAPIMatch
}

type ExternalAPIMatch struct {
	ID     int
	Time   time.Time
	Home   ExternalAPITeam
	Away   ExternalAPITeam
	Status ExternalMatchStatus
}

type ExternalAPITeam struct {
	ID       int
	Score    int
	Name     string
	LongName string
}

type ClientTask struct {
	Name      string
	ExecuteAt time.Time
}

func fromRepositoryExternalMatch(f repository.ExternalMatch) *ExternalMatch {
	return &ExternalMatch{
		ID:        f.ID,
		MatchID:   f.MatchID,
		HomeScore: f.HomeScore,
		AwayScore: f.AwayScore,
		Status:    ExternalMatchStatus(f.Status),
	}
}

func fromClientFotmobLeagues(response client.MatchesResponse) ([]ExternalAPILeague, error) {
	leagues := make([]ExternalAPILeague, 0, len(response.Leagues))
	for _, v := range response.Leagues {
		matches := make([]ExternalAPIMatch, 0, len(v.Matches))

		for _, match := range v.Matches {
			m, err := fromClientFotmobMatch(match)
			if err != nil {
				return nil, fmt.Errorf("failed to map from client match: %w", err)
			}

			matches = append(matches, *m)
		}

		leagues = append(leagues, ExternalAPILeague{
			CountryCode:      v.Ccode,
			Name:             v.Name,
			ParentLeagueName: v.ParentLeagueName,
			Matches:          matches,
		})
	}

	return leagues, nil
}

func fromClientFotmobMatch(match client.MatchFotmob) (*ExternalAPIMatch, error) {
	startsAt, err := time.Parse(time.RFC3339, match.Status.UTCTime)
	if err != nil {
		return nil, fmt.Errorf("unable to parse match starting time %s: %w", match.Status.UTCTime, err)
	}

	return &ExternalAPIMatch{
		ID:     match.ID,
		Time:   startsAt,
		Home:   fromClientFotmobTeam(match.Home),
		Away:   fromClientFotmobTeam(match.Away),
		Status: fromClientMatchStatus(match.ID, match.StatusID),
	}, nil
}

func fromClientMatchStatus(matchID int, statusID int) ExternalMatchStatus {
	switch statusID {
	// 1 - Not started
	case 1:
		return StatusMatchNotStarted
	// 5 - Postponed
	// 17 - Abandoned
	case 5, 17, 106:
		return StatusMatchCancelled
	// 2 - Live 1st half
	// 3 - Live 2nd half
	// 10 - Half-Time
	case 2, 3, 10:
		return StatusMatchInProgress
	// 6 - Full-Time
	// 11 - After extra time
	// 13 - Finished after Penalties
	case 6, 11, 13:
		return StatusMatchFinished
	// 4, 7, 8, 9, 12, 14, 15, 16
	default:
		fmt.Printf("match with id %d has unknown status status: %d\n", matchID, statusID)
		return StatusMatchUnknown
	}
}

func fromClientFotmobTeam(team client.TeamFotmob) ExternalAPITeam {
	return ExternalAPITeam{
		ID:       team.ID,
		Score:    team.Score,
		Name:     team.Name,
		LongName: team.LongName,
	}
}

func fromClientTask(task client.Task) ClientTask {
	return ClientTask{
		Name:      task.Name,
		ExecuteAt: task.ExecuteAt,
	}
}

func fromRepositoryMatch(m repository.Match) *Match {
	var externalMatch *ExternalMatch
	if m.ExternalMatch != nil {
		externalMatch = fromRepositoryExternalMatch(*m.ExternalMatch)
	}

	var homeTeam *Team
	if m.HomeTeam != nil {
		aliases := make([]Alias, 0, len(m.HomeTeam.Aliases))
		for _, alias := range m.HomeTeam.Aliases {
			aliases = append(aliases, Alias{TeamID: alias.TeamID, Alias: alias.Alias})
		}

		homeTeam = &Team{ID: m.HomeTeam.ID, Aliases: aliases}
	}

	var awayTeam *Team
	if m.AwayTeam != nil {
		aliases := make([]Alias, 0, len(m.AwayTeam.Aliases))
		for _, alias := range m.AwayTeam.Aliases {
			aliases = append(aliases, Alias{TeamID: alias.TeamID, Alias: alias.Alias})
		}

		awayTeam = &Team{ID: m.AwayTeam.ID, Aliases: aliases}
	}

	match := Match{
		ID:            m.ID,
		StartsAt:      m.StartsAt,
		ResultStatus:  ResultStatus(m.ResultStatus),
		ExternalMatch: externalMatch,
		HomeTeam:      homeTeam,
		AwayTeam:      awayTeam,
	}

	if m.CheckResultTask != nil {
		checkResultTask := fromRepositoryCheckResultTask(*m.CheckResultTask)
		match.CheckResultTask = &checkResultTask
	}

	return &match
}

func fromRepositoryExternalTeam(t repository.ExternalTeam) ExternalTeam {
	return ExternalTeam{
		ID:     t.ID,
		TeamID: t.TeamID,
	}
}

func fromRepositoryAlias(a repository.Alias) Alias {
	var externalTeam *ExternalTeam

	if a.ExternalTeam != nil {
		mapped := fromRepositoryExternalTeam(*a.ExternalTeam)
		externalTeam = &mapped
	}

	return Alias{
		Alias:        a.Alias,
		TeamID:       a.TeamID,
		ExternalTeam: externalTeam,
	}
}

func fromRepositorySubscription(s repository.Subscription) Subscription {
	var match *Match

	if s.Match != nil {
		match = fromRepositoryMatch(*s.Match)
	}

	return Subscription{
		ID:         s.ID,
		Url:        s.Url,
		MatchID:    s.MatchID,
		Key:        s.Key,
		CreatedAt:  s.CreatedAt,
		Status:     SubscriptionStatus(s.Status),
		NotifiedAt: s.NotifiedAt,
		Match:      match,
	}
}

func fromRepositorySubscriptions(s []repository.Subscription) []Subscription {
	subscriptions := make([]Subscription, 0, len(s))
	for i := range s {
		subscriptions = append(subscriptions, fromRepositorySubscription(s[i]))
	}

	return subscriptions
}

func fromRepositoryCheckResultTask(t repository.CheckResultTask) CheckResultTask {
	return CheckResultTask{
		ID:            t.ID,
		MatchID:       t.MatchID,
		Name:          t.Name,
		AttemptNumber: t.AttemptNumber,
	}
}

func toRepositoryExternalMatch(matchID uint, externalMatch ExternalAPIMatch) repository.ExternalMatch {
	return repository.ExternalMatch{
		ID:        uint(externalMatch.ID),
		MatchID:   matchID,
		HomeScore: externalMatch.Home.Score,
		AwayScore: externalMatch.Away.Score,
		Status:    string(externalMatch.Status),
	}
}

func toRepositoryMatch(homeTeamID, awayTeamID uint, startsAt time.Time, resultStatus ResultStatus) repository.Match {
	return repository.Match{
		HomeTeamID:   homeTeamID,
		AwayTeamID:   awayTeamID,
		StartsAt:     startsAt,
		ResultStatus: string(resultStatus),
	}
}

func toRepositoryCheckResultTask(matchID uint, attemptNumber uint, task ClientTask) repository.CheckResultTask {
	return repository.CheckResultTask{
		MatchID:       matchID,
		Name:          task.Name,
		ExecuteAt:     task.ExecuteAt,
		AttemptNumber: attemptNumber,
	}
}
