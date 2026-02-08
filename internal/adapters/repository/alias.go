package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
)

type AliasRepository struct {
	db *gorm.DB
}

func NewAliasRepository(db *gorm.DB) *AliasRepository {
	return &AliasRepository{db: db}
}

func (r *AliasRepository) Find(ctx context.Context, alias string) (*models.Alias, error) {
	var a Alias

	result := r.db.WithContext(ctx).Joins("ExternalTeam").Where("alias ILIKE ?", alias).First(&a)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errs.NewResourceNotFoundError(fmt.Errorf("alias %s not found: %w", alias, result.Error))
		}

		return nil, fmt.Errorf("failed to find alias: %w", result.Error)
	}

	domain := toDomainAlias(a)

	return &domain, nil
}

func (r *AliasRepository) SaveInTrx(ctx context.Context, alias string, externalTeamID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		team := Team{}
		if err := tx.Create(&team).Error; err != nil {
			return fmt.Errorf("failed to create team: %w", err)
		}

		a := Alias{TeamID: team.ID, Alias: alias}
		if err := tx.Create(&a).Error; err != nil {
			return fmt.Errorf("failed to create alias: %w", err)
		}

		if err := tx.Create(&ExternalTeam{ID: externalTeamID, TeamID: team.ID}).Error; err != nil {
			return fmt.Errorf("failed to create external team: %w", err)
		}

		return nil
	})
}

func (r *AliasRepository) Search(ctx context.Context, alias string) ([]models.Alias, error) {
	var aliases []Alias
	result := r.db.WithContext(ctx).Where("alias ILIKE ?", "%"+alias+"%").Limit(10).Find(&aliases)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to search aliases: %w", result.Error)
	}

	return toDomainAliases(aliases), nil
}
