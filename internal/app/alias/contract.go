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
	SaveInTrx(ctx context.Context, alias string, providerTeam models.ProviderTeam) error
	SaveForTeam(ctx context.Context, alias string, teamID uint) error
}

type ProviderTeamRepository interface {
	FindByProviderAndProviderTeamID(ctx context.Context, provider, providerTeamID string) (*models.ProviderTeam, error)
}

type ProviderClient interface {
	GetTeams(ctx context.Context, date time.Time) ([]models.ProviderAPITeam, error)
}

type Logger interface {
	Error() *zerolog.Event
	Info() *zerolog.Event
	Debug() *zerolog.Event
}
