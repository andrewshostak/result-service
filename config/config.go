package config

import (
	"time"

	"github.com/caarlos0/env/v9"
)

type Server struct {
	App         App
	ExternalAPI ExternalAPI
	Result      ResultCheck
	PG          PG
	GoogleCloud GoogleCloud
}

type BackfillAliases struct {
	PG          PG
	ExternalAPI ExternalAPI
}

type Migrate struct {
	PG PG
}

type App struct {
	Port            string        `env:"PORT" envDefault:"8080"`
	HashedAPIKeys   []string      `env:"HASHED_API_KEYS" envSeparator:","`
	SecretKey       string        `env:"SECRET_KEY,required"`
	Timeout         time.Duration `env:"TIMEOUT" envDefault:"10s"`
	TriggersTimeout time.Duration `env:"TRIGGERS_TIMEOUT" envDefault:"20s"`
}

type ExternalAPI struct {
	FotmobAPIBaseURL string `env:"FOTMOB_API_BASE_URL" envDefault:"https://www.fotmob.com"`
}

type ResultCheck struct {
	MaxRetries        uint          `env:"MAX_RETRIES" envDefault:"5"`
	Interval          time.Duration `env:"INTERVAL" envDefault:"15m"`
	FirstAttemptDelay time.Duration `env:"FIRST_ATTEMPT_DELAY" envDefault:"115m"`
}

type PG struct {
	Host     string `env:"PG_HOST" envDefault:"localhost"`
	User     string `env:"PG_USER" envDefault:"postgres"`
	Password string `env:"PG_PASSWORD,required"`
	Port     string `env:"PG_PORT" envDefault:"5432"`
	Database string `env:"PG_DATABASE" envDefault:"postgres"`
}

type GoogleCloud struct {
	ProjectID           string `env:"GOOGLE_CLOUD_PROJECT_ID,required"`
	Region              string `env:"GOOGLE_CLOUD_REGION,required"`
	TasksBaseURL        string `env:"GOOGLE_CLOUD_BASE_URL,required"` // Base URL to be passed as 'audience' param when creating a cloud task. Then cloud tasks will call this URL.
	ServiceAccountEmail string `env:"GOOGLE_CLOUD_SERVICE_ACCOUNT_EMAIL,required"`

	CheckResultQueueName      string `env:"GOOGLE_CLOUD_CHECK_RESULT_QUEUE_NAME,required" envDefault:"check-result"`
	NotifySubscriberQueueName string `env:"GOOGLE_CLOUD_NOTIFY_SUBSCRIBER_QUEUE_NAME,required" envDefault:"notify-subscriber"`
}

func Parse[T any]() T {
	config := new(T)
	if err := env.Parse(config); err != nil {
		panic(err)
	}

	return *config
}
