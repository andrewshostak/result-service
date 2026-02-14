package server

import (
	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/infra/http/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	MatchHandler        *handler.MatchHandler
	SubscriptionHandler *handler.SubscriptionHandler
	AliasHandler        *handler.AliasHandler
	TriggerHandler      *handler.TriggerHandler
}

func NewServer(cfg config.Server, handlers Handlers) (*gin.Engine, error) {
	r := gin.Default()

	registerRoutes(r, cfg, handlers)

	return r, nil
}

func registerRoutes(r *gin.Engine, cfg config.Server, handlers Handlers) {
	v1 := r.Group("/v1")
	apiKey := v1.Group("").
		Use(middleware.APIKeyAuth(cfg.App.HashedAPIKeys, cfg.App.SecretKey)).
		Use(middleware.Timeout(cfg.App.Timeout))

	googleAuth := v1.Group("").
		Use(middleware.ValidateGoogleAuth(cfg.GoogleCloud.TasksBaseURL)).
		Use(middleware.Timeout(cfg.App.TriggersTimeout))

	apiKey.POST("/matches", handlers.MatchHandler.Create)
	apiKey.POST("/subscriptions", handlers.SubscriptionHandler.Create)
	apiKey.DELETE("/subscriptions", handlers.SubscriptionHandler.Delete)
	apiKey.GET("/aliases", handlers.AliasHandler.Search)

	googleAuth.POST("/triggers/result_check", handlers.TriggerHandler.CheckResult)
	googleAuth.POST("/triggers/subscriber_notification", handlers.TriggerHandler.NotifySubscriber)
}
