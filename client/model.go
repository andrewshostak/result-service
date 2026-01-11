package client

import "time"

type Notification struct {
	Url  string
	Key  string
	Home uint
	Away uint
}

type NotificationBody struct {
	Home uint `json:"home"`
	Away uint `json:"away"`
}

// TODO: rename after removing football-api
type MatchesResponse struct {
	Leagues []LeagueFotmob `json:"leagues"`
}

type LeagueFotmob struct {
	Ccode            string        `json:"ccode"`
	Name             string        `json:"name"`
	ParentLeagueName string        `json:"parentLeagueName"`
	Matches          []MatchFotmob `json:"matches"`
}

type MatchFotmob struct {
	ID       int          `json:"id"`
	Home     TeamFotmob   `json:"home"`
	Away     TeamFotmob   `json:"away"`
	StatusID int          `json:"statusId"`
	Status   StatusFotmob `json:"status"`
}

type StatusFotmob struct {
	UTCTime string `json:"utcTime"`
}

type TeamFotmob struct {
	ID       int    `json:"id"`
	Score    int    `json:"score"`
	Name     string `json:"name"`
	LongName string `json:"longName"`
}

type Task struct {
	Name      string
	ExecuteAt time.Time
}
