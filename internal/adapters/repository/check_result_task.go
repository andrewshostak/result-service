package repository

import (
	"context"
	"fmt"

	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CheckResultTaskRepository struct {
	db *gorm.DB
}

func NewCheckResultTaskRepository(db *gorm.DB) *CheckResultTaskRepository {
	return &CheckResultTaskRepository{db: db}
}

func (r *CheckResultTaskRepository) Save(ctx context.Context, checkResultTask models.CheckResultTask) (*models.CheckResultTask, error) {
	task := CheckResultTask{
		MatchID:       checkResultTask.MatchID,
		Name:          checkResultTask.Name,
		AttemptNumber: checkResultTask.AttemptNumber,
		ExecuteAt:     checkResultTask.ExecuteAt,
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "match_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "attempt_number", "execute_at"}),
	}).Create(&task)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to save check result task: %w", result.Error)
	}

	domain := toDomainCheckResultTask(task)
	return &domain, nil
}
