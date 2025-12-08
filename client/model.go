package client

type FixturesResponse struct {
	Response []Result `json:"response"`
}

type Result struct {
	Fixture Fixture `json:"fixture"`
	Teams   Teams   `json:"teams"`
	Goals   Goals   `json:"goals"`
	Score   Score   `json:"score"`
}

type Fixture struct {
	ID     uint   `json:"id"`
	Status Status `json:"status"`
	Date   string `json:"date"`
}

type Teams struct {
	Home Team `json:"home"`
	Away Team `json:"away"`
}

type Team struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type Goals struct {
	Home uint `json:"home"`
	Away uint `json:"away"`
}

type Score struct {
	Fulltime  Goals `json:"fulltime"`
	Extratime Goals `json:"extratime"`
}

type Status struct {
	Short string `json:"short"`
	Long  string `json:"long"`
}

type TeamsResponse struct {
	Response []TeamsResult `json:"response"`
}

type TeamsResult struct {
	Team Team `json:"team"`
}

type LeaguesResponse struct {
	Response []LeagueResult `json:"response"`
}

type LeagueResult struct {
	League  League  `json:"league"`
	Country Country `json:"country"`
}

type League struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type Country struct {
	Name string `json:"name"`
}

type FixtureSearch struct {
	Season   *uint
	Timezone *string
	Date     *string
	TeamID   *uint
	ID       *uint
}

type TeamsSearch struct {
	Season uint
	League uint
}

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
