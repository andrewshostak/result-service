package main

import (
	"context"
	"fmt"
	"net/http"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"github.com/andrewshostak/result-service/client"
	"github.com/andrewshostak/result-service/config"
	"github.com/andrewshostak/result-service/handler"
	loggerinternal "github.com/andrewshostak/result-service/logger"
	"github.com/andrewshostak/result-service/middleware"
	"github.com/andrewshostak/result-service/repository"
	"github.com/andrewshostak/result-service/service"
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
	httpClient := http.Client{}

	ctx := context.Background()
	cloudTasksClient, err := cloudtasks.NewClient(ctx)
	if err != nil {
		panic(err)
	}

	defer cloudTasksClient.Close()

	footballAPIClient := client.NewFootballAPIClient(&httpClient, logger, cfg.ExternalAPI.FootballAPIBaseURL, cfg.ExternalAPI.RapidAPIKey)
	notifierClient := client.NewNotifierClient(&httpClient, logger)
	taskClient := client.NewClient(cfg.GoogleCloud, cloudTasksClient)

	aliasRepository := repository.NewAliasRepository(db)
	matchRepository := repository.NewMatchRepository(db)
	footballAPIFixtureRepository := repository.NewFootballAPIFixtureRepository(db)
	subscriptionRepository := repository.NewSubscriptionRepository(db)
	checkResultTaskRepository := repository.NewCheckResultTaskRepository(db)

	matchService := service.NewMatchService(
		aliasRepository,
		matchRepository,
		footballAPIFixtureRepository,
		checkResultTaskRepository,
		footballAPIClient,
		taskClient,
		logger,
		cfg.Result.PollingMaxRetries,
		cfg.Result.PollingInterval,
		cfg.Result.PollingFirstAttemptDelay,
	)
	subscriptionService := service.NewSubscriptionService(subscriptionRepository, matchRepository, aliasRepository, taskClient, logger)
	aliasService := service.NewAliasService(aliasRepository, logger)
	resultCheckerService := service.NewResultCheckerService(
		cfg.Result,
		matchRepository,
		footballAPIFixtureRepository,
		subscriptionRepository,
		checkResultTaskRepository,
		taskClient,
		footballAPIClient,
		logger,
	)
	subscriberNotifierService := service.NewSubscriberNotifierService(subscriptionRepository, matchRepository, notifierClient, logger)

	matchHandler := handler.NewMatchHandler(matchService)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService)
	aliasHandler := handler.NewAliasHandler(aliasService)
	triggerHandler := handler.NewTriggerHandler(resultCheckerService, subscriberNotifierService)

	v1 := r.Group("/v1")
	apiKey := v1.Group("").Use(middleware.APIKeyAuth(cfg.App.HashedAPIKeys, cfg.App.SecretKey))
	googleAuth := v1.Group("").Use(middleware.ValidateGoogleAuth(cfg.GoogleCloud.TasksBaseURL))

	apiKey.POST("/matches", matchHandler.Create)
	apiKey.POST("/subscriptions", subscriptionHandler.Create)
	apiKey.DELETE("/subscriptions", subscriptionHandler.Delete)
	apiKey.GET("/aliases", aliasHandler.Search)

	googleAuth.POST("/triggers/result_check", triggerHandler.CheckResult)
	googleAuth.POST("/triggers/subscriber_notification", triggerHandler.NotifySubscriber)

	_ = r.Run(fmt.Sprintf(":%s", cfg.App.Port))
}
