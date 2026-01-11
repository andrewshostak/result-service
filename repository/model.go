package repository

import (
	"time"
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
