package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
)

type ProviderMatchRepository struct {
	db *gorm.DB
}

func NewProviderMatchRepository(db *gorm.DB) *ProviderMatchRepository {
	return &ProviderMatchRepository{db: db}
}

func (r *ProviderMatchRepository) Save(ctx context.Context, id *uint, providerMatch models.ProviderMatch) (*models.ProviderMatch, error) {
	toSave := ProviderMatch{
		MatchID:         providerMatch.MatchID,
		ProviderID:      providerMatch.ProviderID,
		ProviderMatchID: providerMatch.ProviderMatchID,
		StartsAt:        providerMatch.StartsAt,
		HomeScore:       providerMatch.HomeScore,
		AwayScore:       providerMatch.AwayScore,
		Status:          string(providerMatch.Status),
		ProviderStatus:  providerMatch.ProviderStatus,
	}
	if id != nil {
		toSave.ID = *id
	}

	result := r.db.WithContext(ctx).Save(&toSave)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to save provider match: %w", result.Error)
	}

	domain := toDomainProviderMatch(toSave)
	return &domain, nil
}
