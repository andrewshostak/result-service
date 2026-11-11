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
		MatchID:         checkResultTask.MatchID,
		ProviderMatchID: checkResultTask.ProviderMatchID,
		Name:            checkResultTask.Name,
		AttemptNumber:   checkResultTask.AttemptNumber,
		ExecuteAt:       checkResultTask.ExecuteAt,
	}

	// Both match_id and provider_match_id are unique during the migration - see todo.md Feature #1.
	conflictColumn := "match_id"
	if checkResultTask.ProviderMatchID != nil {
		conflictColumn = "provider_match_id"
	}

	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: conflictColumn}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "attempt_number", "execute_at"}),
	}).Create(&task)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to save check result task: %w", result.Error)
	}

	domain := toDomainCheckResultTask(task)
	return &domain, nil
}
