package service

import (
	"context"
	"time"

	"github.com/andrewshostak/result-service/client"
	"github.com/andrewshostak/result-service/repository"
	"github.com/rs/zerolog"
)

type AliasRepository interface {
	Find(ctx context.Context, alias string) (*repository.Alias, error)
	SaveInTrx(ctx context.Context, alias string, footballAPITeamID uint) error
	Search(ctx context.Context, alias string) ([]repository.Alias, error)
}

type MatchRepository interface {
	Create(ctx context.Context, match repository.Match) (*repository.Match, error)
	Save(ctx context.Context, id *uint, match repository.Match) (*repository.Match, error)
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, resultStatus string) ([]repository.Match, error)
	One(ctx context.Context, search repository.Match) (*repository.Match, error)
	Update(ctx context.Context, id uint, resultStatus string) (*repository.Match, error)
}

type FootballAPIFixtureRepository interface {
	Create(ctx context.Context, fixture repository.FootballApiFixture, data repository.Data) (*repository.FootballApiFixture, error)
	Save(ctx context.Context, fixture repository.FootballApiFixture, data repository.Data) (*repository.FootballApiFixture, error)
	Update(ctx context.Context, id uint, data repository.Data) (*repository.FootballApiFixture, error)
}

type CheckResultTaskRepository interface {
	Create(ctx context.Context, name string, matchID uint) (*repository.CheckResultTask, error)
	Update(ctx context.Context, id uint, checkResultTask repository.CheckResultTask) (*repository.CheckResultTask, error)
}

type FootballAPIClient interface {
	SearchFixtures(ctx context.Context, search client.FixtureSearch) (*client.FixturesResponse, error)
	SearchLeagues(ctx context.Context, season uint) (*client.LeaguesResponse, error)
	SearchTeams(ctx context.Context, search client.TeamsSearch) (*client.TeamsResponse, error)
}

type FotmobClient interface {
	GetMatchesByDate(ctx context.Context, date time.Time) (*client.MatchesResponse, error)
}

type TaskClient interface {
	ScheduleResultCheck(ctx context.Context, matchID uint, attempt uint, scheduleAt time.Time) (*string, error)
	ScheduleSubscriberNotification(ctx context.Context, subscriptionID uint) error
	DeleteResultCheckTask(ctx context.Context, taskName string) error
}

type NotifierClient interface {
	Notify(ctx context.Context, notification client.Notification) error
}

type SubscriptionRepository interface {
	Create(ctx context.Context, subscription repository.Subscription) (*repository.Subscription, error)
	Delete(ctx context.Context, id uint) error
	One(ctx context.Context, matchID uint, key string, baseURL string) (*repository.Subscription, error)
	Get(ctx context.Context, id uint) (*repository.Subscription, error)
	List(ctx context.Context, matchID uint) ([]repository.Subscription, error)
	ListByMatchAndStatus(ctx context.Context, matchID uint, status string) ([]repository.Subscription, error)
	Update(ctx context.Context, id uint, subscription repository.Subscription) error
}

type SeasonHelper interface {
	CurrentSeason() int
}

type Logger interface {
	Error() *zerolog.Event
	Info() *zerolog.Event
}
