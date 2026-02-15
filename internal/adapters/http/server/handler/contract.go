package handler

import (
	"context"

	"github.com/andrewshostak/result-service/internal/app/models"
)

type AliasService interface {
	Search(ctx context.Context, alias string) ([]string, error)
}

type MatchService interface {
	Create(ctx context.Context, request models.CreateMatchRequest) (uint, error)
}

type SubscriptionService interface {
	Create(ctx context.Context, request models.CreateSubscriptionRequest) error
	Delete(ctx context.Context, request models.DeleteSubscriptionRequest) error
}

type ResultCheckerService interface {
	CheckResult(ctx context.Context, matchID uint) error
}

type SubscriberNotifierService interface {
	NotifySubscriber(ctx context.Context, subscriptionID uint) error
}
