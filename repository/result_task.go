package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/andrewshostak/result-service/errs"
	"gorm.io/gorm"
)

type ResultTaskRepository struct {
	db *gorm.DB
}

func NewResultTaskRepository(db *gorm.DB) *ResultTaskRepository {
	return &ResultTaskRepository{db: db}
}

func (r *ResultTaskRepository) Create(ctx context.Context, name string, matchID uint) (*ResultTask, error) {
	resultTask := ResultTask{Name: name, MatchID: matchID}
	result := r.db.WithContext(ctx).Create(&resultTask)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return nil, fmt.Errorf("result task already exists: %w", errs.ResultTaskAlreadyExistsError{Message: result.Error.Error()})
		}

		return nil, fmt.Errorf("failed to create result task: %w", result.Error)
	}

	return &resultTask, nil
}
