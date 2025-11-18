package service

import (
	"context"

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
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, resultStatus repository.ResultStatus) ([]repository.Match, error)
	One(ctx context.Context, search repository.Match) (*repository.Match, error)
	Update(ctx context.Context, id uint, resultStatus repository.ResultStatus) (*repository.Match, error)
}

type FootballAPIFixtureRepository interface {
	Create(ctx context.Context, fixture repository.FootballApiFixture, data repository.Data) (*repository.FootballApiFixture, error)
	Update(ctx context.Context, id uint, data repository.Data) (*repository.FootballApiFixture, error)
}

type FootballAPIClient interface {
	SearchFixtures(ctx context.Context, search client.FixtureSearch) (*client.FixturesResponse, error)
	SearchLeagues(ctx context.Context, season uint) (*client.LeaguesResponse, error)
	SearchTeams(ctx context.Context, search client.TeamsSearch) (*client.TeamsResponse, error)
}

type NotifierClient interface {
	Notify(ctx context.Context, notification client.Notification) error
}

type SubscriptionRepository interface {
	Create(ctx context.Context, subscription repository.Subscription) (*repository.Subscription, error)
	Delete(ctx context.Context, id uint) error
	One(ctx context.Context, matchID uint, key string, baseURL string) (*repository.Subscription, error)
	List(ctx context.Context, matchID uint) ([]repository.Subscription, error)
	ListUnNotified(ctx context.Context) ([]repository.Subscription, error)
	Update(ctx context.Context, id uint, subscription repository.Subscription) error
}

type SeasonHelper interface {
	CurrentSeason() int
}

type Logger interface {
	Error() *zerolog.Event
	Info() *zerolog.Event
}
