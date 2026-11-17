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
		ProviderMatchID: checkResultTask.ProviderMatchID,
		Name:            checkResultTask.Name,
		AttemptNumber:   checkResultTask.AttemptNumber,
		ExecuteAt:       checkResultTask.ExecuteAt,
	}

	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider_match_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "attempt_number", "execute_at"}),
	}).Create(&task)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to save check result task: %w", result.Error)
	}

	domain := toDomainCheckResultTask(task)
	return &domain, nil
}

func (r *CheckResultTaskRepository) FindByProviderMatchID(ctx context.Context, providerMatchID uint) (*models.CheckResultTask, error) {
	var task CheckResultTask

	result := r.db.WithContext(ctx).Where("provider_match_id = ?", providerMatchID).First(&task)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, models.NewResourceNotFoundError(fmt.Errorf("check result task not found: %w", result.Error))
		}

		return nil, fmt.Errorf("failed to find check result task: %w", result.Error)
	}

	domain := toDomainCheckResultTask(task)
	return &domain, nil
}

func (r *CheckResultTaskRepository) ListByMatchID(ctx context.Context, matchID uint) ([]models.CheckResultTask, error) {
	var tasks []CheckResultTask

	result := r.db.WithContext(ctx).
		Joins("JOIN provider_matches ON provider_matches.id = check_result_tasks.provider_match_id").
		Where("provider_matches.match_id = ?", matchID).
		Find(&tasks)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to list check result tasks: %w", result.Error)
	}

	domain := make([]models.CheckResultTask, 0, len(tasks))
	for _, task := range tasks {
		domain = append(domain, toDomainCheckResultTask(task))
	}

	return domain, nil
}
