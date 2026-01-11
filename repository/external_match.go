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
