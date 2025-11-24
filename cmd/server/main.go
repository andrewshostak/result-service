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
	cfg := config.Parse()

	logger := loggerinternal.SetupLogger()

	r := gin.Default()

	db := repository.EstablishDatabaseConnection(cfg)
	httpClient := http.Client{}

	ctx := context.Background()
	cloudTasksClient, err := cloudtasks.NewClient(ctx)
	if err != nil {
		panic(err)
	}

	defer cloudTasksClient.Close()

	r.Use(middleware.Authorization(cfg.App.HashedAPIKeys, cfg.App.SecretKey))

	v1 := r.Group("/v1")

	footballAPIClient := client.NewFootballAPIClient(&httpClient, logger, cfg.ExternalAPI.FootballAPIBaseURL, cfg.ExternalAPI.RapidAPIKey)
	_ = client.NewNotifierClient(&httpClient, logger)
	_ = client.NewClient(cfg.GoogleCloud, cloudTasksClient)

	aliasRepository := repository.NewAliasRepository(db)
	matchRepository := repository.NewMatchRepository(db)
	footballAPIFixtureRepository := repository.NewFootballAPIFixtureRepository(db)
	subscriptionRepository := repository.NewSubscriptionRepository(db)

	matchService := service.NewMatchService(
		aliasRepository,
		matchRepository,
		footballAPIFixtureRepository,
		footballAPIClient,
		logger,
		cfg.Result.PollingMaxRetries,
		cfg.Result.PollingInterval,
		cfg.Result.PollingFirstAttemptDelay,
	)
	subscriptionService := service.NewSubscriptionService(subscriptionRepository, matchRepository, aliasRepository, logger)
	aliasService := service.NewAliasService(aliasRepository, logger)

	matchHandler := handler.NewMatchHandler(matchService)
	subscriptionHandler := handler.NewSubscriptionHandler(subscriptionService)
	aliasHandler := handler.NewAliasHandler(aliasService)
	v1.POST("/matches", matchHandler.Create)
	v1.POST("/subscriptions", subscriptionHandler.Create)
	v1.DELETE("/subscriptions", subscriptionHandler.Delete)
	v1.GET("/aliases", aliasHandler.Search)

	_ = r.Run(fmt.Sprintf(":%s", cfg.App.Port))
}
