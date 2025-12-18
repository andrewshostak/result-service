package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/errs"
	"gorm.io/gorm"
)

type AliasRepository struct {
	db *gorm.DB
}

func NewAliasRepository(db *gorm.DB) *AliasRepository {
	return &AliasRepository{db: db}
}

func (r *AliasRepository) Find(ctx context.Context, alias string) (*Alias, error) {
	var a Alias

	result := r.db.WithContext(ctx).Joins("ExternalTeam").Where("alias ILIKE ?", alias).First(&a)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("alias %s not found: %w", alias, errs.AliasNotFoundError{Message: result.Error.Error()})
		}

		return nil, result.Error
	}

	return &a, nil
}

func (r *AliasRepository) SaveInTrx(ctx context.Context, alias string, footballAPITeamID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team := Team{}
		if err := tx.Create(&team).Error; err != nil {
			return fmt.Errorf("failed to create team: %w", err)
		}

		a := Alias{TeamID: team.ID, Alias: alias}
		if err := tx.Create(&a).Error; err != nil {
			return fmt.Errorf("failed to create alias: %w", err)
		}

		if err := tx.Create(&ExternalTeam{ID: footballAPITeamID, TeamID: team.ID}).Error; err != nil {
			return fmt.Errorf("failed to create football api team: %w", err)
		}

		return nil
	})
}

func (r *AliasRepository) Search(ctx context.Context, alias string) ([]Alias, error) {
	var aliases []Alias
	result := r.db.WithContext(ctx).Where("alias ILIKE ?", "%"+alias+"%").Limit(10).Find(&aliases)

	if result.Error != nil {
		return nil, result.Error
	}

	return aliases, nil
}
