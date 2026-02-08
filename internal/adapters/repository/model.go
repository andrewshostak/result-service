package repository

import (
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
)

type Alias struct {
	ID     uint   `gorm:"column:id;primaryKey"`
	TeamID uint   `gorm:"column:team_id"`
	Alias  string `gorm:"column:alias;unique"`

	ExternalTeam *ExternalTeam `gorm:"foreignKey:TeamID;references:TeamID"`
}

type Team struct {
	ID uint `gorm:"column:id;primaryKey"`

	Aliases []Alias
}

type ExternalTeam struct {
	ID     uint `gorm:"column:id;primaryKey"`
	TeamID uint `gorm:"column:team_id"`
}

type Match struct {
	ID           uint      `gorm:"column:id;primaryKey"`
	HomeTeamID   uint      `gorm:"column:home_team_id"`
	AwayTeamID   uint      `gorm:"column:away_team_id"`
	StartsAt     time.Time `gorm:"column:starts_at"`
	ResultStatus string    `gorm:"column:result_status;default:not_scheduled"`

	ExternalMatch   *ExternalMatch
	CheckResultTask *CheckResultTask
	HomeTeam        *Team `gorm:"foreignKey:HomeTeamID"`
	AwayTeam        *Team `gorm:"foreignKey:AwayTeamID"`
}

type ExternalMatch struct {
	ID        uint   `gorm:"column:id;primaryKey"`
	MatchID   uint   `gorm:"column:match_id"`
	HomeScore int    `gorm:"column:home_score"`
	AwayScore int    `gorm:"column:away_score"`
	Status    string `gorm:"column:status"`

	Match *Match `gorm:"foreignKey:MatchID"`
}

type Subscription struct {
	ID              uint       `gorm:"column:id;primaryKey"`
	Url             string     `gorm:"column:url;unique"`
	MatchID         uint       `gorm:"column:match_id"`
	Key             string     `gorm:"column:key;unique"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	Status          string     `gorm:"column:status;default:pending"`
	SubscriberError *string    `gorm:"column:subscriber_error"`
	NotifiedAt      *time.Time `gorm:"column:notified_at"`

	Match *Match `gorm:"foreignKey:MatchID"`
}

type CheckResultTask struct {
	ID            uint      `gorm:"column:id;primaryKey"`
	MatchID       uint      `gorm:"column:match_id;unique"`
	Name          string    `gorm:"column:name;unique"`
	AttemptNumber uint      `gorm:"column:attempt_number;default:1"`
	ExecuteAt     time.Time `gorm:"column:execute_at"`
	CreatedAt     time.Time `gorm:"column:created_at"`

	Match *Match `gorm:"foreignKey:MatchID"`
}

func toDomainAlias(a Alias) models.Alias {
	var externalTeam *models.ExternalTeam

	if a.ExternalTeam != nil {
		mapped := toDomainExternalTeam(*a.ExternalTeam)
		externalTeam = &mapped
	}

	return models.Alias{
		Alias:        a.Alias,
		TeamID:       a.TeamID,
		ExternalTeam: externalTeam,
	}
}

func toDomainAliases(aliases []Alias) []models.Alias {
	mapped := make([]models.Alias, 0, len(aliases))

	for _, alias := range aliases {
		mapped = append(mapped, toDomainAlias(alias))
	}

	return mapped
}

func toDomainExternalTeam(t ExternalTeam) models.ExternalTeam {
	return models.ExternalTeam{
		ID:     t.ID,
		TeamID: t.TeamID,
	}
}

func toDomainExternalMatch(f ExternalMatch) models.ExternalMatch {
	return models.ExternalMatch{
		ID:        f.ID,
		MatchID:   f.MatchID,
		HomeScore: f.HomeScore,
		AwayScore: f.AwayScore,
		Status:    models.ExternalMatchStatus(f.Status),
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
		ResultStatus: models.ResultStatus(m.ResultStatus),
	}

	if m.ExternalMatch != nil {
		externalMatch := toDomainExternalMatch(*m.ExternalMatch)
		match.ExternalMatch = &externalMatch
	}

	if m.HomeTeam != nil {
		match.HomeTeam = &models.Team{ID: m.HomeTeam.ID, Aliases: toDomainAliases(m.HomeTeam.Aliases)}
	}

	if m.AwayTeam != nil {
		match.AwayTeam = &models.Team{ID: m.AwayTeam.ID, Aliases: toDomainAliases(m.AwayTeam.Aliases)}
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
