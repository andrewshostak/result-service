package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewshostak/result-service/errs"
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
		return nil, result.Error
	}

	return &match, nil
}

func (r *MatchRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&Match{}, id)
	if result.Error != nil {
		return result.Error
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
		return nil, result.Error
	}

	return matches, nil
}

func (r *MatchRepository) One(ctx context.Context, search Match) (*Match, error) {
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
			message := fmt.Sprintf("match between teams with ids %d and %d starting at %s is not found", search.HomeTeamID, search.AwayTeamID, search.StartsAt)
			return nil, fmt.Errorf("%s: %w", message, errs.MatchNotFoundError{Message: result.Error.Error()})
		}

		return nil, result.Error
	}

	return &match, nil
}

func (r *MatchRepository) Save(ctx context.Context, id *uint, match Match) (*Match, error) {
	toSave := match
	if id != nil {
		toSave.ID = *id
	}

	result := r.db.WithContext(ctx).Save(&toSave)
	if result.Error != nil {
		return nil, result.Error
	}

	return &toSave, nil
}

func (r *MatchRepository) Update(ctx context.Context, id uint, resultStatus string) (*Match, error) {
	match := Match{ID: id}
	result := r.db.WithContext(ctx).Model(&match).Updates(Match{ResultStatus: resultStatus})
	if result.Error != nil {
		return nil, result.Error
	}

	return &match, nil
}
