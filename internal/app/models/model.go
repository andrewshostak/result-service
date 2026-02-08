package models

import (
	"time"
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
	HomeTeamID   uint
	AwayTeamID   uint
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
	ID              uint
	Url             string
	MatchID         uint
	Key             string
	CreatedAt       time.Time
	Status          SubscriptionStatus
	NotifiedAt      *time.Time
	SubscriberError *string

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
	ExecuteAt     time.Time
}

// TODO: match domain model doesn't care about leagues
// TODO: backfill aliases domain model needs leagues for filtering
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
	ID    int
	Score int
	Name  string
}

type Task struct {
	Name      string
	ExecuteAt time.Time
}

type SubscriberNotification struct {
	Url  string
	Key  string
	Home uint
	Away uint
}

func (m *ExternalAPIMatch) ToExternalMatch(matchID uint) ExternalMatch {
	return ExternalMatch{
		ID:        uint(m.ID),
		MatchID:   matchID,
		HomeScore: m.Home.Score,
		AwayScore: m.Away.Score,
		Status:    m.Status,
	}
}
