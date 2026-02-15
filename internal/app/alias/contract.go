package alias

import (
	"context"
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
	"github.com/rs/zerolog"
)

type AliasRepository interface {
	Search(ctx context.Context, alias string) ([]models.Alias, error)
	Find(ctx context.Context, alias string) (*models.Alias, error)
	SaveInTrx(ctx context.Context, alias string, externalTeamID uint) error
}

type ExternalAPIClient interface {
	GetTeams(ctx context.Context, date time.Time) ([]models.ExternalAPITeam, error)
}

type Logger interface {
	Error() *zerolog.Event
	Info() *zerolog.Event
	Debug() *zerolog.Event
}
