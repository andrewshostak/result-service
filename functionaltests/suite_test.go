//go:build functional

package functionaltests

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

type FunctionalTestSuite struct {
	suite.Suite

	postgresContainer *postgres.PostgresContainer
	appContainer      testcontainers.Container
	cloudTasksClient  testcontainers.Container
	testNetwork       *testcontainers.DockerNetwork

	db *sqlx.DB

	apiBaseURL string
	httpClient *http.Client
}

// called once before all tests
func (s *FunctionalTestSuite) SetupSuite() {
	ctx := context.Background()

	// create a shared Docker network
	nw, err := network.New(ctx)
	s.Require().NoError(err)
	s.testNetwork = nw

	s.T().Log("starting postgresql container...")

	pgContainer, err := postgres.Run(ctx,
		"postgres:15.4-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		network.WithNetwork([]string{"postgres"}, nw),
		postgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.postgresContainer = pgContainer

	// get connection string
	connectionString, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	s.Require().NoError(err)

	// connect to database
	s.db, err = sqlx.Connect("postgres", connectionString)
	s.Require().NoError(err)

	// run migrations
	err = s.runMigrations()
	s.Require().NoError(err)

	s.T().Log("starting cloud-tasks-emulator container...")

	reqEmulator := testcontainers.ContainerRequest{
		Image:    "ghcr.io/aertje/cloud-tasks-emulator:latest",
		Networks: []string{nw.Name},
		NetworkAliases: map[string][]string{
			nw.Name: {"cloud-tasks-emulator"},
		},
		ExposedPorts: []string{"8123/tcp"},
		WaitingFor:   wait.ForListeningPort("8123/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: reqEmulator,
		Started:          true,
	})
	s.Require().NoError(err)

	s.cloudTasksClient = container

	// start application container
	s.T().Log("Starting application container...")

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:       "../",
			Dockerfile:    "functionaltests/functional.Dockerfile",
			PrintBuildLog: true,
		},
		Networks:     []string{nw.Name},
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"GIN_MODE": gin.TestMode,

			"PG_HOST":     "postgres",
			"PG_PORT":     "5432",
			"PG_PASSWORD": "postgres",

			"SECRET_KEY":      "i_am_a_secret_key",
			"HASHED_API_KEYS": "a87a39c7ddb9682faa412e209834b92d96470cc21878f391c719b3357a8126387b3817628dca009b5e5a66a9e576bbf9361d8b60a7f85f5cfd3f17c15cfed6b5",

			"GOOGLE_CLOUD_PROJECT_ID":            "test-project",
			"GOOGLE_CLOUD_REGION":                "europe-west3",
			"GOOGLE_CLOUD_BASE_URL":              "http://cloud-tasks-emulator:8123",
			"GOOGLE_CLOUD_SERVICE_ACCOUNT_EMAIL": "test-sa@test-project.iam.gserviceaccount.com",
			// better to have an absolute path. if it doesn't work - try google-test-credentials.json
			"GOOGLE_APPLICATION_CREDENTIALS": "/app/google-test-credentials.json",
		},
		Cmd: []string{"./bin/server"},
		WaitingFor: wait.ForListeningPort("8080/tcp").
			WithStartupTimeout(20 * time.Second),
	}

	appContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	s.Require().NoError(err)
	s.appContainer = appContainer

	// get app URL
	host, err := appContainer.Host(ctx)
	s.Require().NoError(err)
	port, err := appContainer.MappedPort(ctx, "8080")
	s.Require().NoError(err)

	s.apiBaseURL = fmt.Sprintf("http://%s:%s", host, port.Port())
	s.T().Logf("Application running at %s", s.apiBaseURL)

	s.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
}

// called once after all tests
func (s *FunctionalTestSuite) TearDownSuite() {
	ctx := context.Background()

	// terminate containers in reverse order
	if s.appContainer != nil {
		s.T().Log("Stopping application container...")
		_ = s.appContainer.Terminate(ctx)
	}

	if s.db != nil {
		_ = s.db.Close()
	}

	if s.cloudTasksClient != nil {
		s.T().Log("Stopping cloud tasks container...")
		_ = s.cloudTasksClient.Terminate(ctx)
	}

	if s.postgresContainer != nil {
		s.T().Log("Stopping PostgreSQL container...")
		_ = s.postgresContainer.Terminate(ctx)
	}

	if s.testNetwork != nil {
		s.T().Log("Removing test network...")
		_ = s.testNetwork.Remove(ctx)
	}
}

// called before each test
func (s *FunctionalTestSuite) SetupTest() {
	s.cleanDatabase()
}

func (s *FunctionalTestSuite) runMigrations() error {
	driver, err := migratepg.WithInstance(s.db.DB, &migratepg.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance("file://../database/migrations", "postgres", driver)
	if err != nil {
		return err
	}

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

func (s *FunctionalTestSuite) cleanDatabase() {
	tables := []string{
		"teams",
		"external_teams",
		"matches",
		"aliases",
		"subscriptions",
		"external_matches",
		"check_result_tasks",
	}
	for _, table := range tables {
		_, _ = s.db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
	}
}

func (s *FunctionalTestSuite) DB() *sqlx.DB {
	return s.db
}

func (s *FunctionalTestSuite) APIBaseURL() string {
	return s.apiBaseURL
}

func TestServerSuite(t *testing.T) {
	suite.Run(t, new(FunctionalTestSuite))
}
