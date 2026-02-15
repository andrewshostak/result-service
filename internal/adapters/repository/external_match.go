package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
)

type ExternalMatchRepository struct {
	db *gorm.DB
}

func NewExternalMatchRepository(db *gorm.DB) *ExternalMatchRepository {
	return &ExternalMatchRepository{db: db}
}

func (r *ExternalMatchRepository) Save(ctx context.Context, id *uint, externalMatch models.ExternalMatch) (*models.ExternalMatch, error) {
	toSave := ExternalMatch{
		ID:        externalMatch.ID,
		MatchID:   externalMatch.MatchID,
		HomeScore: externalMatch.HomeScore,
		AwayScore: externalMatch.AwayScore,
		Status:    string(externalMatch.Status),
	}
	if id != nil {
		toSave.ID = *id
	}

	result := r.db.WithContext(ctx).Save(&toSave)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to save external match: %w", result.Error)
	}

	domain := toDomainExternalMatch(toSave)
	return &domain, nil
}
