package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/fotmob"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/notifier"
	"github.com/andrewshostak/result-service/internal/adapters/http/client/task"
	"github.com/andrewshostak/result-service/internal/adapters/http/server/handler"
	"github.com/andrewshostak/result-service/internal/adapters/repository"
	"github.com/andrewshostak/result-service/internal/app/alias"
	"github.com/andrewshostak/result-service/internal/app/match"
	"github.com/andrewshostak/result-service/internal/app/subscription"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/middleware"
	"github.com/gin-gonic/gin"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "run",
		Short: "Server starts running the server",
		Run:   startServer,
	}

	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

func startServer(_ *cobra.Command, _ []string) {
	cfg := config.Parse[config.Server]()

	logger := loggerinternal.SetupLogger()

	r := gin.Default()

	db := repository.EstablishDatabaseConnection(cfg.PG)
	httpClient := http.Client{Timeout: cfg.App.TriggersTimeout - (2 * time.Second)}

	ctx := context.Background()
	cloudTasksClient, err := cloudtasks.NewClient(ctx)
	if err != nil {
		panic(err)
	}

	defer cloudTasksClient.Close()

	fotmobClient := fotmob.NewFotmobClient(&httpClient, logger, cfg.ExternalAPI)
	notifierClient := notifier.NewNotifierClient(&httpClient, logger)
	taskClient := task.NewClient(cfg.GoogleCloud, cfg.App.TriggersTimeout+(2*time.Second), cloudTasksClient)

	aliasRepository := repository.NewAliasRepository(db)
	matchRepository := repository.NewMatchRepository(db)
	externalMatchRepository := repository.NewExternalMatchRepository(db)
	subscriptionRepository := repository.NewSubscriptionRepository(db)
	checkResultTaskRepository := repository.NewCheckResultTaskRepository(db)

	matchService := match.NewMatchService(
		cfg.Result,
		aliasRepository,
		matchRepository,
		externalMatchRepository,
		checkResultTaskRepository,
		fotmobClient,
		taskClient,
		logger,
	)
	subscriptionService := subscription.NewSubscriptionService(subscriptionRepository, matchRepository, aliasRepository, taskClient, logger)
	aliasService := alias.NewAliasService(aliasRepository, logger)
	resultCheckerService := match.NewResultCheckerService(
		cfg.Result,
		matchRepository,
		externalMatchRepository,
		subscriptionRepository,
		checkResultTaskRepository,
		taskClient,
		fotmobClient,
		logger,
	)
	subscriberNotifierService := subscription.NewSubscriberNotifierService(subscriptionRepository, matchRepository, notifierClient, logger)

	matchHandler := handler.NewMatchHandler(matchService)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService)
	aliasHandler := handler.NewAliasHandler(aliasService)
	triggerHandler := handler.NewTriggerHandler(resultCheckerService, subscriberNotifierService)

	v1 := r.Group("/v1")
	apiKey := v1.Group("").
		Use(middleware.APIKeyAuth(cfg.App.HashedAPIKeys, cfg.App.SecretKey)).
		Use(middleware.Timeout(cfg.App.Timeout))

	googleAuth := v1.Group("").
		Use(middleware.ValidateGoogleAuth(cfg.GoogleCloud.TasksBaseURL)).
		Use(middleware.Timeout(cfg.App.TriggersTimeout))

	apiKey.POST("/matches", matchHandler.Create)
	apiKey.POST("/subscriptions", subscriptionHandler.Create)
	apiKey.DELETE("/subscriptions", subscriptionHandler.Delete)
	apiKey.GET("/aliases", aliasHandler.Search)

	googleAuth.POST("/triggers/result_check", triggerHandler.CheckResult)
	googleAuth.POST("/triggers/subscriber_notification", triggerHandler.NotifySubscriber)

	_ = r.Run(fmt.Sprintf(":%s", cfg.App.Port))
}
