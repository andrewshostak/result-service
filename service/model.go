package service

import (
	"encoding/json"
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

	FootballApiFixtures []FootballAPIFixture
	CheckResultTask     *CheckResultTask
	HomeTeam            *Team
	AwayTeam            *Team
}

type CheckResultTask struct {
	ID            uint
	MatchID       uint
	Name          string
	AttemptNumber uint
}

type Team struct {
	ID uint

	Aliases []Alias
}

type Alias struct {
	Alias  string
	TeamID uint

	FootballApiTeam *FootballApiTeam
}

type FootballApiTeam struct {
	ID     uint
	TeamID uint
}

type FootballAPIFixture struct {
	ID   uint
	Home uint
	Away uint
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

type Data struct {
	Fixture Fixture       `json:"fixture"`
	Teams   TeamsExternal `json:"teams"`
	Goals   Goals         `json:"goals"`
}

type LeagueData struct {
	League  League
	Country Country
}

type Fixture struct {
	ID     uint   `json:"id"`
	Status Status `json:"status"`
	Date   string `json:"date"`
}

type TeamsExternal struct {
	Home TeamExternal `json:"home"`
	Away TeamExternal `json:"away"`
}

type TeamExternal struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type Goals struct {
	Home uint `json:"home"`
	Away uint `json:"away"`
}

type Status struct {
	Short string `json:"short"`
	Long  string `json:"long"`
}

type League struct {
	ID   uint
	Name string
}

type Country struct {
	Name string
}

func fromRepositoryFootballAPIFixture(f repository.FootballApiFixture) (*FootballAPIFixture, error) {
	d := &Data{}
	err := json.Unmarshal(f.Data.Bytes, d)
	if err != nil {
		return nil, err
	}

	return &FootballAPIFixture{
		ID:   f.ID,
		Home: d.Goals.Home,
		Away: d.Goals.Away,
	}, nil
}

func fromClientFootballAPIFixture(c client.Result) Data {
	return Data{
		Fixture: Fixture{
			ID: c.Fixture.ID,
			Status: Status{
				Short: c.Fixture.Status.Short,
				Long:  c.Fixture.Status.Long,
			},
			Date: c.Fixture.Date,
		},
		Teams: TeamsExternal{
			Home: TeamExternal{
				ID:   c.Teams.Home.ID,
				Name: c.Teams.Home.Name,
			},
			Away: TeamExternal{
				ID:   c.Teams.Away.ID,
				Name: c.Teams.Away.Name,
			},
		},
		Goals: Goals{
			Home: c.Goals.Home,
			Away: c.Goals.Away,
		},
	}
}

func fromClientFootballAPILeague(l client.LeagueResult) LeagueData {
	return LeagueData{
		League: League{
			ID:   l.League.ID,
			Name: l.League.Name,
		},
		Country: Country{Name: l.Country.Name},
	}
}

func fromClientFootballAPITeam(t client.TeamsResult) TeamExternal {
	return TeamExternal{
		ID:   t.Team.ID,
		Name: t.Team.Name,
	}
}

func fromClientFootballAPITeams(t []client.TeamsResult) []TeamExternal {
	mapped := make([]TeamExternal, 0, len(t))
	for i := range t {
		mapped = append(mapped, fromClientFootballAPITeam(t[i]))
	}

	return mapped
}

func fromRepositoryMatch(m repository.Match) (*Match, error) {
	fixtures := make([]FootballAPIFixture, 0, len(m.FootballApiFixtures))
	for _, fixture := range m.FootballApiFixtures {
		repoApiFixture, err := fromRepositoryFootballAPIFixture(fixture)
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, *repoApiFixture)
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

	var checkResultTask CheckResultTask
	if m.CheckResultTask != nil {
		checkResultTask = fromRepositoryCheckResultTask(*m.CheckResultTask)
	}
	return &Match{
		ID:                  m.ID,
		StartsAt:            m.StartsAt,
		ResultStatus:        ResultStatus(m.ResultStatus),
		FootballApiFixtures: fixtures,
		CheckResultTask:     &checkResultTask,
		HomeTeam:            homeTeam,
		AwayTeam:            awayTeam,
	}, nil
}

func fromRepositoryMatches(m []repository.Match) ([]Match, error) {
	matches := make([]Match, 0, len(m))
	for i := range m {
		match, err := fromRepositoryMatch(m[i])
		if err != nil {
			return nil, err
		}
		matches = append(matches, *match)
	}

	return matches, nil
}

func fromRepositoryFootballAPITeam(t repository.FootballApiTeam) FootballApiTeam {
	return FootballApiTeam{
		ID:     t.ID,
		TeamID: t.TeamID,
	}
}

func fromRepositoryAlias(a repository.Alias) Alias {
	var footballAPITeam *FootballApiTeam

	if a.FootballApiTeam != nil {
		mapped := fromRepositoryFootballAPITeam(*a.FootballApiTeam)
		footballAPITeam = &mapped
	}

	return Alias{
		Alias:           a.Alias,
		TeamID:          a.TeamID,
		FootballApiTeam: footballAPITeam,
	}
}

func fromRepositorySubscription(s repository.Subscription) (*Subscription, error) {
	var match *Match

	if s.Match != nil {
		mapped, err := fromRepositoryMatch(*s.Match)
		if err != nil {
			return nil, err
		}
		match = mapped
	}

	return &Subscription{
		ID:         s.ID,
		Url:        s.Url,
		MatchID:    s.MatchID,
		Key:        s.Key,
		CreatedAt:  s.CreatedAt,
		Status:     SubscriptionStatus(s.Status),
		NotifiedAt: s.NotifiedAt,
		Match:      match,
	}, nil
}

func fromRepositorySubscriptions(s []repository.Subscription) ([]Subscription, error) {
	subscriptions := make([]Subscription, 0, len(s))
	for i := range s {
		repositorySubscription, err := fromRepositorySubscription(s[i])
		if err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, *repositorySubscription)
	}

	return subscriptions, nil
}

func fromRepositoryCheckResultTask(t repository.CheckResultTask) CheckResultTask {
	return CheckResultTask{
		ID:            t.ID,
		MatchID:       t.MatchID,
		Name:          t.Name,
		AttemptNumber: t.AttemptNumber,
	}
}

func toRepositoryFootballAPIFixtureData(data Data) repository.Data {
	return repository.Data{
		Fixture: repository.Fixture{
			ID: data.Fixture.ID,
			Status: repository.Status{
				Short: data.Fixture.Status.Short,
				Long:  data.Fixture.Status.Long,
			},
			Date: data.Fixture.Date,
		},
		Teams: repository.TeamsExternal{
			Home: repository.TeamExternal{
				ID:   data.Teams.Home.ID,
				Name: data.Teams.Home.Name,
			},
			Away: repository.TeamExternal{
				ID:   data.Teams.Away.ID,
				Name: data.Teams.Away.Name,
			},
		},
		Goals: repository.Goals{
			Home: data.Goals.Home,
			Away: data.Goals.Away,
		},
	}
}
