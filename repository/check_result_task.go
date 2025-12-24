package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/andrewshostak/result-service/errs"
	"gorm.io/gorm"
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

		return nil, result.Error
	}

	return &task, nil
}

func (r *CheckResultTaskRepository) Create(ctx context.Context, checkResultTask CheckResultTask) (*CheckResultTask, error) {
	result := r.db.WithContext(ctx).Create(&checkResultTask)
	if result.Error != nil {
		if isDuplicateError(result.Error) {
			return nil, fmt.Errorf("check result task already exists: %w", errs.CheckResultTaskAlreadyExistsError{Message: result.Error.Error()})
		}

		return nil, fmt.Errorf("failed to create check result task: %w", result.Error)
	}

	return &checkResultTask, nil
}

func (r *CheckResultTaskRepository) Update(ctx context.Context, id uint, checkResultTask CheckResultTask) (*CheckResultTask, error) {
	task := CheckResultTask{ID: id}
	result := r.db.WithContext(ctx).Model(&task).Updates(checkResultTask)
	if result.Error != nil {
		return nil, result.Error
	}

	return &task, nil
}

func isDuplicateError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	if strings.Contains(err.Error(), "duplicate key") {
		return true
	}

	return false
}
