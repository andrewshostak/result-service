package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/errs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CheckResultTaskRepository struct {
	db *gorm.DB
}

func NewCheckResultTaskRepository(db *gorm.DB) *CheckResultTaskRepository {
	return &CheckResultTaskRepository{db: db}
}

func (r *CheckResultTaskRepository) GetByMatchID(ctx context.Context, matchID uint) (*CheckResultTask, error) {
	task := CheckResultTask{}
	result := r.db.WithContext(ctx).Where(&CheckResultTask{MatchID: matchID}).First(&task)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("check result task of match id %d not found: %w", matchID, errs.CheckResultNotFoundError{Message: result.Error.Error()})
		}

		return nil, fmt.Errorf("failed to get check result task by match id: %w", result.Error)
	}

	return &task, nil
}

func (r *CheckResultTaskRepository) Save(ctx context.Context, checkResultTask CheckResultTask) (*CheckResultTask, error) {
	task := checkResultTask
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "match_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "attempt_number", "execute_at"}),
	}).Create(&task)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to save check result task: %w", result.Error)
	}

	return &task, nil
}
