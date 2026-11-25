package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/infra/http/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type Handlers struct {
	MatchHandler        *handler.MatchHandler
	SubscriptionHandler *handler.SubscriptionHandler
	AliasHandler        *handler.AliasHandler
	TriggerHandler      *handler.TriggerHandler
}

func NewServer(cfg config.Server, handlers Handlers, logger *zerolog.Logger) (*gin.Engine, error) {
	r := gin.New()
	r.Use(middleware.ErrorLogger(logger))
	r.Use(middleware.Recovery())

	registerRoutes(r, cfg, handlers)

	return r, nil
}

// Serve derives shutdownCtx from context.Background(), not ctx, since ctx is already cancelled by the time Shutdown runs.
func Serve(ctx context.Context, engine *gin.Engine, addr string, shutdownTimeout time.Duration, logger *zerolog.Logger) error {
	httpServer := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout) // shutdownTimeout should leave margin under the platform's SIGTERM-to-SIGKILL grace period
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("graceful shutdown deadline exceeded, forcing close")
		return httpServer.Close()
	}

	return nil
}

func registerRoutes(r *gin.Engine, cfg config.Server, handlers Handlers) {
	v1 := r.Group("/v1")
	apiKey := v1.Group("").
		Use(middleware.APIKeyAuth(cfg.App.HashedAPIKeys, cfg.App.SecretKey)).
		Use(middleware.Timeout(cfg.App.Timeout))

	googleAuth := v1.Group("").
		Use(middleware.ValidateGoogleAuth(cfg.GoogleCloud.TargetURL)).
		Use(middleware.Timeout(cfg.App.TriggersTimeout))

	apiKey.POST("/matches", handlers.MatchHandler.Create)
	apiKey.POST("/subscriptions", handlers.SubscriptionHandler.Create)
	apiKey.DELETE("/subscriptions", handlers.SubscriptionHandler.Delete)
	apiKey.GET("/aliases", handlers.AliasHandler.Search)

	googleAuth.POST("/triggers/provider_result_check", handlers.TriggerHandler.CheckProviderMatchResult)
	googleAuth.POST("/triggers/subscriber_notification", handlers.TriggerHandler.NotifySubscriber)
}
