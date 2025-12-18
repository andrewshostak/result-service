package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgtype"
	"gorm.io/gorm"
)

type ExternalMatchRepository struct {
	db *gorm.DB
}

func NewExternalMatchRepository(db *gorm.DB) *ExternalMatchRepository {
	return &ExternalMatchRepository{db: db}
}

func (r *ExternalMatchRepository) Create(ctx context.Context, externalMatch ExternalMatch, data Data) (*ExternalMatch, error) {
	dataAsJson, err := toJsonB(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonb data: %w", err)
	}

	externalMatch.Data = *dataAsJson
	result := r.db.WithContext(ctx).Create(&externalMatch)
	if result.Error != nil {
		return nil, result.Error
	}

	return &externalMatch, nil
}

func (r *ExternalMatchRepository) Save(ctx context.Context, externalMatch ExternalMatch, data Data) (*ExternalMatch, error) {
	dataAsJson, err := toJsonB(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonb data: %w", err)
	}

	externalMatch.Data = *dataAsJson
	result := r.db.WithContext(ctx).Save(&externalMatch)
	if result.Error != nil {
		return nil, result.Error
	}

	return &externalMatch, nil
}

func (r *ExternalMatchRepository) Update(ctx context.Context, id uint, data Data) (*ExternalMatch, error) {
	dataAsJson, err := toJsonB(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonb data: %w", err)
	}

	fixture := ExternalMatch{ID: id}
	result := r.db.WithContext(ctx).Model(&fixture).Updates(ExternalMatch{Data: *dataAsJson})
	if result.Error != nil {
		return nil, result.Error
	}

	return &fixture, nil
}

func toJsonB(result interface{}) (*pgtype.JSONB, error) {
	var externalDataAsJson pgtype.JSONB
	if err := externalDataAsJson.Set(result); err != nil {
		return nil, err
	}

	return &externalDataAsJson, nil
}
