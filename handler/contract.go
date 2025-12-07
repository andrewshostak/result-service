package handler

import (
	"context"

	"github.com/andrewshostak/result-service/service"
)

type AliasService interface {
	Search(ctx context.Context, alias string) ([]string, error)
}

type MatchService interface {
	Create(ctx context.Context, request service.CreateMatchRequest) (uint, error)
}

type SubscriptionService interface {
	Create(ctx context.Context, request service.CreateSubscriptionRequest) error
	Delete(ctx context.Context, request service.DeleteSubscriptionRequest) error
}

type ResultCheckerService interface {
	CheckResult(ctx context.Context, matchID uint) error
}
