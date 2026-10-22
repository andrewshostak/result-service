package match

import (
	"context"
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/rs/zerolog"
)

type AliasRepository interface {
	Find(ctx context.Context, alias string) (*models.Alias, error)
}

type MatchRepository interface {
	One(ctx context.Context, search models.Match) (*models.Match, error)
	Save(ctx context.Context, id *uint, match models.Match) (*models.Match, error)
	Update(ctx context.Context, id uint, match models.Match) (*models.Match, error)
}

type ProviderTeamRepository interface {
	FindByTeamAndProvider(ctx context.Context, teamID uint, provider string) (*models.ProviderTeam, error)
}

type ProviderMatchRepository interface {
	Save(ctx context.Context, providerMatch models.ProviderMatch) (*models.ProviderMatch, error)
	ListByMatchID(ctx context.Context, matchID uint) ([]models.ProviderMatch, error)
}

type CheckResultTaskRepository interface {
	Save(ctx context.Context, checkResultTask models.CheckResultTask) (*models.CheckResultTask, error)
}

type SubscriptionRepository interface {
	ListByMatchAndStatus(ctx context.Context, matchID uint, status models.SubscriptionStatus) ([]models.Subscription, error)
	Update(ctx context.Context, id uint, subscription models.Subscription) error
}

type ResultProvider interface {
	GetMatches(ctx context.Context, date time.Time) ([]models.ExternalAPIMatch, error)
	Name() string
}

type TaskClient interface {
	GetResultCheckTask(ctx context.Context, matchID uint, attempt uint) (*models.Task, error)
	ScheduleResultCheck(ctx context.Context, matchID uint, attempt uint, scheduleAt time.Time) (*models.Task, error)
	ScheduleSubscriberNotification(ctx context.Context, subscriptionID uint) error
}

type Logger interface {
	Error() *zerolog.Event
	Info() *zerolog.Event
	Debug() *zerolog.Event
}
