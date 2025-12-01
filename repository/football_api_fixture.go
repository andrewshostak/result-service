package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgtype"
	"gorm.io/gorm"
)

type FootballAPIFixtureRepository struct {
	db *gorm.DB
}

func NewFootballAPIFixtureRepository(db *gorm.DB) *FootballAPIFixtureRepository {
	return &FootballAPIFixtureRepository{db: db}
}

func (r *FootballAPIFixtureRepository) Create(ctx context.Context, fixture FootballApiFixture, data Data) (*FootballApiFixture, error) {
	dataAsJson, err := toJsonB(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonb data: %w", err)
	}

	fixture.Data = *dataAsJson
	result := r.db.WithContext(ctx).Create(&fixture)
	if result.Error != nil {
		return nil, result.Error
	}

	return &fixture, nil
}

func (r *FootballAPIFixtureRepository) Save(ctx context.Context, fixture FootballApiFixture, data Data) (*FootballApiFixture, error) {
	dataAsJson, err := toJsonB(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonb data: %w", err)
	}

	fixture.Data = *dataAsJson
	result := r.db.WithContext(ctx).Save(&fixture)
	if result.Error != nil {
		return nil, result.Error
	}

	return &fixture, nil
}

func (r *FootballAPIFixtureRepository) Update(ctx context.Context, id uint, data Data) (*FootballApiFixture, error) {
	dataAsJson, err := toJsonB(data)
	if err != nil {
		return nil, fmt.Errorf("failed to create jsonb data: %w", err)
	}

	fixture := FootballApiFixture{ID: id}
	result := r.db.WithContext(ctx).Model(&fixture).Updates(FootballApiFixture{Data: *dataAsJson})
	if result.Error != nil {
		return nil, result.Error
	}

	return &fixture, nil
}

func toJsonB(result interface{}) (*pgtype.JSONB, error) {
	var fixtureAsJson pgtype.JSONB
	if err := fixtureAsJson.Set(result); err != nil {
		return nil, err
	}

	return &fixtureAsJson, nil
}
