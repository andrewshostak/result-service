package subscription

import (
	"context"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/rs/zerolog"
)

type AliasRepository interface {
	Find(ctx context.Context, alias string) (*models.Alias, error)
}

type SubscriptionRepository interface {
	Create(ctx context.Context, subscription models.Subscription) (*models.Subscription, error)
	Delete(ctx context.Context, id uint) error
	One(ctx context.Context, matchID uint, key string, baseURL string) (*models.Subscription, error)
	List(ctx context.Context, matchID uint) ([]models.Subscription, error)
	Update(ctx context.Context, id uint, subscription models.Subscription) error
	Get(ctx context.Context, id uint) (*models.Subscription, error)
}

type MatchRepository interface {
	One(ctx context.Context, search models.Match) (*models.Match, error)
	Delete(ctx context.Context, id uint) error
}

type NotifierClient interface {
	Notify(ctx context.Context, notification models.SubscriberNotification) error
}

type TaskClient interface {
	DeleteResultCheckTask(ctx context.Context, taskName string) error
}

type Logger interface {
	Error() *zerolog.Event
	Info() *zerolog.Event
	Debug() *zerolog.Event
}
