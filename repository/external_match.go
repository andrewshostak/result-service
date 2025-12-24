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
		return nil, result.Error
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
		return nil, result.Error
	}

	return &toSave, nil
}

func (r *ExternalMatchRepository) Update(ctx context.Context, id uint, data Data) (*ExternalMatch, error) {
	return nil, fmt.Errorf("not implemented")
	// TODO
	//dataAsJson, err := toJsonB(data)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to create jsonb data: %w", err)
	//}
	//
	//fixture := ExternalMatch{ID: id}
	//result := r.db.WithContext(ctx).Model(&fixture).Updates(ExternalMatch{Data: *dataAsJson})
	//if result.Error != nil {
	//	return nil, result.Error
	//}
	//
	//return &fixture, nil
}
