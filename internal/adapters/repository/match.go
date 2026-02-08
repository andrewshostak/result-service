package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/errs"
	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
)

type MatchRepository struct {
	db *gorm.DB
}

func NewMatchRepository(db *gorm.DB) *MatchRepository {
	return &MatchRepository{db: db}
}

func (r *MatchRepository) Create(ctx context.Context, match Match) (*Match, error) {
	result := r.db.WithContext(ctx).Create(&match)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to create match: %w", result.Error)
	}

	return &match, nil
}

func (r *MatchRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&Match{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete match: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("match doesn't exist")
	}

	return nil
}

func (r *MatchRepository) List(ctx context.Context, resultStatus string) ([]Match, error) {
	var matches []Match
	result := r.db.WithContext(ctx).
		Where(&Match{ResultStatus: resultStatus}).
		Preload("ExternalMatch").
		Preload("HomeTeam.Aliases").
		Preload("AwayTeam.Aliases").
		Find(&matches)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to list matches: %w", result.Error)
	}

	return matches, nil
}

func (r *MatchRepository) One(ctx context.Context, search models.Match) (*models.Match, error) {
	var match Match

	query := r.db.WithContext(ctx).
		Preload("ExternalMatch").
		Preload("CheckResultTask")

	if search.ID != 0 {
		query = query.Where(&Match{ID: search.ID})
	}

	if search.HomeTeam != nil && search.HomeTeam.ID != 0 {
		query = query.Where(&Match{HomeTeamID: search.HomeTeam.ID})
	}

	if search.AwayTeam != nil && search.AwayTeam.ID != 0 {
		query = query.Where(&Match{AwayTeamID: search.AwayTeam.ID})
	}

	if !search.StartsAt.IsZero() {
		query = query.Where("starts_at >= ?::date", search.StartsAt.Format(time.DateOnly)).
			Where("starts_at < (?::date + '1 day'::interval)", search.StartsAt.Format(time.DateOnly))
	}

	result := query.First(&match)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errs.NewResourceNotFoundError(fmt.Errorf("match not found: %w", result.Error))
		}

		return nil, fmt.Errorf("failed to find match: %w", result.Error)
	}

	domain := toDomainMatch(match)
	return &domain, nil
}

func (r *MatchRepository) Save(ctx context.Context, id *uint, match models.Match) (*models.Match, error) {
	toSave := Match{
		ID:           match.ID,
		HomeTeamID:   match.HomeTeam.ID,
		AwayTeamID:   match.AwayTeam.ID,
		StartsAt:     match.StartsAt,
		ResultStatus: string(match.ResultStatus),
	}
	if id != nil {
		toSave.ID = *id
	}

	result := r.db.WithContext(ctx).Save(&toSave)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to save match: %w", result.Error)
	}

	domain := toDomainMatch(toSave)
	return &domain, nil
}

func (r *MatchRepository) Update(ctx context.Context, id uint, resultStatus models.ResultStatus) (*models.Match, error) {
	match := Match{ID: id}
	result := r.db.WithContext(ctx).Model(&match).Updates(Match{ResultStatus: string(resultStatus)})
	if result.Error != nil {
		return nil, fmt.Errorf("failed to update match: %w", result.Error)
	}

	domain := toDomainMatch(match)
	return &domain, nil
}
