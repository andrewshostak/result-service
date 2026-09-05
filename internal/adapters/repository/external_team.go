package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
)

type ExternalTeamRepository struct {
	db *gorm.DB
}

func NewExternalTeamRepository(db *gorm.DB) *ExternalTeamRepository {
	return &ExternalTeamRepository{db: db}
}

func (r *ExternalTeamRepository) FindExternalTeam(ctx context.Context, externalTeamID uint) (*models.ExternalTeam, error) {
	var t ExternalTeam

	result := r.db.WithContext(ctx).Where("id = ?", externalTeamID).First(&t)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, models.NewResourceNotFoundError(fmt.Errorf("external team %d not found: %w", externalTeamID, result.Error))
		}

		return nil, fmt.Errorf("failed to find external team: %w", result.Error)
	}

	domain := toDomainExternalTeam(t)

	return &domain, nil
}
