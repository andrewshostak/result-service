package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CheckResultTaskRepository struct {
	db *gorm.DB
}

func NewCheckResultTaskRepository(db *gorm.DB) *CheckResultTaskRepository {
	return &CheckResultTaskRepository{db: db}
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
