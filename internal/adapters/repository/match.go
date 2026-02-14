package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
)

type MatchRepository struct {
	db *gorm.DB
}

func NewMatchRepository(db *gorm.DB) *MatchRepository {
	return &MatchRepository{db: db}
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

func (r *MatchRepository) One(ctx context.Context, search models.Match) (*models.Match, error) {
	var match Match

	query := r.db.WithContext(ctx).
		Preload("ExternalMatch").
		Preload("CheckResultTask")

	if search.ID != 0 {
		query = query.Where(&Match{ID: search.ID})
	}

	if search.HomeTeamID != 0 {
		query = query.Where(&Match{HomeTeamID: search.HomeTeamID})
	}

	if search.AwayTeamID != 0 {
		query = query.Where(&Match{AwayTeamID: search.AwayTeamID})
	}

	if !search.StartsAt.IsZero() {
		query = query.Where("starts_at >= ?::date", search.StartsAt.Format(time.DateOnly)).
			Where("starts_at < (?::date + '1 day'::interval)", search.StartsAt.Format(time.DateOnly))
	}

	result := query.First(&match)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, models.NewResourceNotFoundError(fmt.Errorf("match not found: %w", result.Error))
		}

		return nil, fmt.Errorf("failed to find match: %w", result.Error)
	}

	domain := toDomainMatch(match)
	return &domain, nil
}

func (r *MatchRepository) Save(ctx context.Context, id *uint, match models.Match) (*models.Match, error) {
	toSave := Match{
		ID:           match.ID,
		HomeTeamID:   match.HomeTeamID,
		AwayTeamID:   match.AwayTeamID,
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
