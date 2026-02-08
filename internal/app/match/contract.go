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
	Update(ctx context.Context, id uint, resultStatus models.ResultStatus) (*models.Match, error)
}

type ExternalMatchRepository interface {
	Save(ctx context.Context, id *uint, externalMatch models.ExternalMatch) (*models.ExternalMatch, error)
}

type CheckResultTaskRepository interface {
	Save(ctx context.Context, checkResultTask models.CheckResultTask) (*models.CheckResultTask, error)
}

type SubscriptionRepository interface {
	ListByMatchAndStatus(ctx context.Context, matchID uint, status models.SubscriptionStatus) ([]models.Subscription, error)
	Update(ctx context.Context, id uint, subscription models.Subscription) error
}

type ExternalAPIClient interface {
	GetMatchesByDate(ctx context.Context, date time.Time) ([]models.ExternalAPILeague, error)
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
