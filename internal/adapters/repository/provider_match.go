package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProviderMatchRepository struct {
	db *gorm.DB
}

func NewProviderMatchRepository(db *gorm.DB) *ProviderMatchRepository {
	return &ProviderMatchRepository{db: db}
}

func (r *ProviderMatchRepository) Save(ctx context.Context, providerMatch models.ProviderMatch) (*models.ProviderMatch, error) {
	toSave := ProviderMatch{
		MatchID:         providerMatch.MatchID,
		Provider:        providerMatch.Provider,
		ProviderMatchID: providerMatch.ProviderMatchID,
		StartsAt:        providerMatch.StartsAt,
		HomeScore:       providerMatch.HomeScore,
		AwayScore:       providerMatch.AwayScore,
		Status:          string(providerMatch.Status),
		ProviderStatus:  providerMatch.ProviderStatus,
	}

	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "match_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"provider_match_id", "starts_at", "home_score",
			"away_score", "status", "provider_status", "updated_at",
		}),
	}).Create(&toSave)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to save provider match: %w", result.Error)
	}

	domain := toDomainProviderMatch(toSave)
	return &domain, nil
}
