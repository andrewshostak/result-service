package repository

import (
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
)

type Alias struct {
	ID        uint      `gorm:"column:id;primaryKey"`
	TeamID    uint      `gorm:"column:team_id"`
	Alias     string    `gorm:"column:alias;unique"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

type Team struct {
	ID        uint      `gorm:"column:id;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`

	Aliases []Alias
}

type Provider struct {
	ID   uint   `gorm:"column:id;primaryKey"`
	Name string `gorm:"column:name"`
}

type ProviderTeam struct {
	ID             uint      `gorm:"column:id;primaryKey"`
	TeamID         uint      `gorm:"column:team_id"`
	Provider       string    `gorm:"column:provider"`
	ProviderTeamID string    `gorm:"column:provider_team_id"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

type ProviderMatch struct {
	ID              uint      `gorm:"column:id;primaryKey" db:"id"`
	MatchID         uint      `gorm:"column:match_id" db:"match_id"`
	Provider        string    `gorm:"column:provider" db:"provider"`
	ProviderMatchID string    `gorm:"column:provider_match_id" db:"provider_match_id"`
	StartsAt        time.Time `gorm:"column:starts_at" db:"starts_at"`
	HomeScore       int       `gorm:"column:home_score" db:"home_score"`
	AwayScore       int       `gorm:"column:away_score" db:"away_score"`
	Status          string    `gorm:"column:status" db:"status"`
	ProviderStatus  *string   `gorm:"column:provider_status" db:"provider_status"`
	CreatedAt       time.Time `gorm:"column:created_at" db:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" db:"updated_at"`
}

type Match struct {
	ID           uint      `gorm:"column:id;primaryKey" db:"id"`
	HomeTeamID   uint      `gorm:"column:home_team_id" db:"home_team_id"`
	AwayTeamID   uint      `gorm:"column:away_team_id" db:"away_team_id"`
	StartsAt     time.Time `gorm:"column:starts_at" db:"starts_at"`
	ResultStatus string    `gorm:"column:result_status;default:not_scheduled" db:"result_status"`
	HomeScore    *int      `gorm:"column:home_score" db:"home_score"`
	AwayScore    *int      `gorm:"column:away_score" db:"away_score"`
	CreatedAt    time.Time `gorm:"column:created_at" db:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" db:"updated_at"`

	CheckResultTask *CheckResultTask
	HomeTeam        *Team `gorm:"foreignKey:HomeTeamID"`
	AwayTeam        *Team `gorm:"foreignKey:AwayTeamID"`
}

type Subscription struct {
	ID              uint       `gorm:"column:id;primaryKey" db:"id"`
	Url             string     `gorm:"column:url;unique" db:"url"`
	MatchID         uint       `gorm:"column:match_id" db:"match_id"`
	Key             string     `gorm:"column:key" db:"key"`
	CreatedAt       time.Time  `gorm:"column:created_at" db:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at" db:"updated_at"`
	Status          string     `gorm:"column:status;default:pending" db:"status"`
	SubscriberError *string    `gorm:"column:subscriber_error" db:"subscriber_error"`
	NotifiedAt      *time.Time `gorm:"column:notified_at" db:"notified_at"`

	Match *Match `gorm:"foreignKey:MatchID"`
}

type CheckResultTask struct {
	ID            uint      `gorm:"column:id;primaryKey" db:"id"`
	MatchID       uint      `gorm:"column:match_id;unique" db:"match_id"`
	Name          string    `gorm:"column:name;unique" db:"name"`
	AttemptNumber uint      `gorm:"column:attempt_number;default:1" db:"attempt_number"`
	ExecuteAt     time.Time `gorm:"column:execute_at" db:"execute_at"`
	CreatedAt     time.Time `gorm:"column:created_at" db:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" db:"updated_at"`

	Match *Match `gorm:"foreignKey:MatchID"`
}

func toDomainProviderTeam(pt ProviderTeam) models.ProviderTeam {
	return models.ProviderTeam{
		ID:             pt.ID,
		TeamID:         pt.TeamID,
		Provider:       pt.Provider,
		ProviderTeamID: pt.ProviderTeamID,
	}
}

func toDomainAlias(a Alias) models.Alias {
	return models.Alias{
		Alias:  a.Alias,
		TeamID: a.TeamID,
	}
}

func toDomainAliases(aliases []Alias) []models.Alias {
	mapped := make([]models.Alias, 0, len(aliases))

	for _, alias := range aliases {
		mapped = append(mapped, toDomainAlias(alias))
	}

	return mapped
}

func toDomainProvider(p Provider) models.Provider {
	return models.Provider{
		ID:   p.ID,
		Name: p.Name,
	}
}

func toDomainProviderMatch(p ProviderMatch) models.ProviderMatch {
	return models.ProviderMatch{
		ID:              p.ID,
		MatchID:         p.MatchID,
		Provider:        p.Provider,
		ProviderMatchID: p.ProviderMatchID,
		StartsAt:        p.StartsAt,
		HomeScore:       p.HomeScore,
		AwayScore:       p.AwayScore,
		Status:          models.ExternalMatchStatus(p.Status),
		ProviderStatus:  p.ProviderStatus,
	}
}

func toDomainCheckResultTask(t CheckResultTask) models.CheckResultTask {
	return models.CheckResultTask{
		ID:            t.ID,
		MatchID:       t.MatchID,
		Name:          t.Name,
		AttemptNumber: t.AttemptNumber,
	}
}

func toDomainMatch(m Match) models.Match {
	match := models.Match{
		ID:           m.ID,
		StartsAt:     m.StartsAt,
		HomeTeamID:   m.HomeTeamID,
		AwayTeamID:   m.AwayTeamID,
		ResultStatus: models.ResultStatus(m.ResultStatus),
		HomeScore:    m.HomeScore,
		AwayScore:    m.AwayScore,
	}

	if m.CheckResultTask != nil {
		checkResultTask := toDomainCheckResultTask(*m.CheckResultTask)
		match.CheckResultTask = &checkResultTask
	}

	return match
}

func toDomainSubscription(s Subscription) models.Subscription {
	var match models.Match

	if s.Match != nil {
		match = toDomainMatch(*s.Match)
	}

	return models.Subscription{
		ID:         s.ID,
		Url:        s.Url,
		MatchID:    s.MatchID,
		Key:        s.Key,
		CreatedAt:  s.CreatedAt,
		Status:     models.SubscriptionStatus(s.Status),
		NotifiedAt: s.NotifiedAt,
		Match:      &match,
	}
}

func toDomainSubscriptions(s []Subscription) []models.Subscription {
	subscriptions := make([]models.Subscription, 0, len(s))
	for i := range s {
		subscriptions = append(subscriptions, toDomainSubscription(s[i]))
	}

	return subscriptions
}
