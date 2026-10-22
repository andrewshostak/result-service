package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
)

type ProviderTeamRepository struct {
	db *gorm.DB
}

func NewProviderTeamRepository(db *gorm.DB) *ProviderTeamRepository {
	return &ProviderTeamRepository{db: db}
}

func (r *ProviderTeamRepository) FindByTeamAndProvider(ctx context.Context, teamID uint, provider string) (*models.ProviderTeam, error) {
	var pt ProviderTeam

	result := r.db.WithContext(ctx).Where("team_id = ? AND provider = ?", teamID, provider).First(&pt)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, models.NewResourceNotFoundError(fmt.Errorf("provider team for team_id %d and provider %s not found: %w", teamID, provider, result.Error))
		}

		return nil, fmt.Errorf("failed to find provider team: %w", result.Error)
	}

	domain := toDomainProviderTeam(pt)
	return &domain, nil
}
