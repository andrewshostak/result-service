package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
)

type ProviderRepository struct {
	db *gorm.DB
}

func NewProviderRepository(db *gorm.DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

func (r *ProviderRepository) FindByName(ctx context.Context, name string) (*models.Provider, error) {
	var p Provider

	result := r.db.WithContext(ctx).Where("name = ?", name).First(&p)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, models.NewResourceNotFoundError(fmt.Errorf("provider %s not found: %w", name, result.Error))
		}

		return nil, fmt.Errorf("failed to find provider: %w", result.Error)
	}

	domain := toDomainProvider(p)
	return &domain, nil
}
