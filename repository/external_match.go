package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

type ExternalMatchRepository struct {
	db *gorm.DB
}

func NewExternalMatchRepository(db *gorm.DB) *ExternalMatchRepository {
	return &ExternalMatchRepository{db: db}
}

func (r *ExternalMatchRepository) Create(ctx context.Context, externalMatch ExternalMatch, data Data) (*ExternalMatch, error) {
	result := r.db.WithContext(ctx).Create(&externalMatch)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to create external match: %w", result.Error)
	}

	return &externalMatch, nil
}

func (r *ExternalMatchRepository) Save(ctx context.Context, id *uint, externalMatch ExternalMatch) (*ExternalMatch, error) {
	toSave := externalMatch
	if id != nil {
		toSave.ID = *id
	}

	result := r.db.WithContext(ctx).Save(&toSave)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to save external match: %w", result.Error)
	}

	return &toSave, nil
}
