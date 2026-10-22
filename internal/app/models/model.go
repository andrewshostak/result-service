package models

import (
	"fmt"
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

type Provider struct {
	ID   uint
	Name string
}

type ProviderTeam struct {
	ID             uint
	TeamID         uint
	Provider       string
	ProviderTeamID string
}

type ProviderMatch struct {
	ID              uint
	MatchID         uint
	Provider        string
	ProviderMatchID string
	StartsAt        time.Time
	HomeScore       int
	AwayScore       int
	Status          ExternalMatchStatus
	ProviderStatus  *string
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
	HomeScore    *int
	AwayScore    *int

	ExternalMatch   *ExternalMatch
	CheckResultTask *CheckResultTask
}

type Alias struct {
	Alias  string
	TeamID uint
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

type League struct {
	CountryCode string
	Name        string
}

type ExternalAPITeam struct {
	ID          int
	Name        string
	LeagueNames []string
	CountryCode string
}

type ExternalAPIMatch struct {
	ID             uint
	HomeID         string
	AwayID         string
	HomeScore      int
	AwayScore      int
	Time           time.Time
	Status         ExternalMatchStatus
	ProviderStatus *string
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
		ID:        m.ID,
		MatchID:   matchID,
		HomeScore: m.HomeScore,
		AwayScore: m.AwayScore,
		Status:    m.Status,
	}
}

func (m *ExternalAPIMatch) ToProviderMatch(matchID uint, provider string) ProviderMatch {
	return ProviderMatch{
		MatchID:         matchID,
		Provider:        provider,
		ProviderMatchID: fmt.Sprintf("%d", m.ID),
		StartsAt:        m.Time,
		HomeScore:       m.HomeScore,
		AwayScore:       m.AwayScore,
		Status:          m.Status,
		ProviderStatus:  m.ProviderStatus,
	}
}
