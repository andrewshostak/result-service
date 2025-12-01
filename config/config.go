package config

import (
	"time"

	"github.com/caarlos0/env/v9"
)

type Server struct {
	App         App
	ExternalAPI ExternalAPI
	Result      ResultPolling
	PG          PG
	GoogleCloud GoogleCloud
}

type BackfillAliases struct {
	PG PG
}

type Migrate struct {
	PG PG
}

type App struct {
	Port          string   `env:"PORT" envDefault:"8080"`
	HashedAPIKeys []string `env:"HASHED_API_KEYS" envSeparator:","`
	SecretKey     string   `env:"SECRET_KEY,required"`
}

type ExternalAPI struct {
	RapidAPIKey        string `env:"RAPID_API_KEY,required"`
	FootballAPIBaseURL string `env:"FOOTBALL_API_BASE_URL" envDefault:"https://api-football-v1.p.rapidapi.com"`
}

type ResultPolling struct {
	PollingMaxRetries        uint          `env:"POLLING_MAX_RETRIES" envDefault:"5"`
	PollingInterval          time.Duration `env:"POLLING_INTERVAL" envDefault:"15m"`
	PollingFirstAttemptDelay time.Duration `env:"POLLING_FIRST_ATTEMPT_DELAY" envDefault:"115m"`
}

type PG struct {
	Host     string `env:"PG_HOST" envDefault:"localhost"`
	User     string `env:"PG_USER" envDefault:"postgres"`
	Password string `env:"PG_PASSWORD,required"`
	Port     string `env:"PG_PORT" envDefault:"5432"`
	Database string `env:"PG_DATABASE" envDefault:"postgres"`
}

type GoogleCloud struct {
	ProjectID    string `env:"GOOGLE_CLOUD_PROJECT_ID,required"`
	Region       string `env:"GOOGLE_CLOUD_REGION,required"`
	TasksBaseURL string `env:"GOOGLE_CLOUD_BASE_URL,required"` // Base URL to be passed as 'audience' param when creating a cloud task. Then cloud tasks will call this URL.
}

func (s *Server) Parse() {
	if err := env.Parse(s); err != nil {
		panic(err)
	}
}

func (s *BackfillAliases) Parse() {
	if err := env.Parse(s); err != nil {
		panic(err)
	}
}

func (s *Migrate) Parse() {
	if err := env.Parse(s); err != nil {
		panic(err)
	}
}
