package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/andrewshostak/result-service/errs"
	"gorm.io/gorm"
)

type SubscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) Create(ctx context.Context, subscription Subscription) (*Subscription, error) {
	result := r.db.WithContext(ctx).Create(&subscription)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrForeignKeyViolated) {
			return nil, fmt.Errorf("match id does not exist: %w", errs.WrongMatchIDError{Message: result.Error.Error()})
		}
		if isDuplicateError(result.Error) {
			return nil, fmt.Errorf("subscription already exists: %w", errs.SubscriptionAlreadyExistsError{Message: result.Error.Error()})
		}

		return nil, result.Error
	}

	return &subscription, nil
}

func (r *SubscriptionRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&Subscription{}, id)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("subscription doesn't exist")
	}

	return nil
}

func (r *SubscriptionRepository) Get(ctx context.Context, id uint) (*Subscription, error) {
	var subscription Subscription
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&subscription)

	if result.Error != nil {
		return nil, result.Error
	}

	return &subscription, nil
}

func (r *SubscriptionRepository) One(ctx context.Context, matchID uint, key string, baseURL string) (*Subscription, error) {
	var subscription Subscription
	result := r.db.WithContext(ctx).
		Where("match_id = ?", matchID).
		Where("url LIKE ?", baseURL+"%").
		Where("key = ?", key).
		First(&subscription)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("subscription is not found: %w", errs.SubscriptionNotFoundError{Message: result.Error.Error()})
		}

		return nil, result.Error
	}

	return &subscription, nil
}

func (r *SubscriptionRepository) List(ctx context.Context, matchID uint) ([]Subscription, error) {
	var subscriptions []Subscription
	result := r.db.WithContext(ctx).
		Where("match_id = ?", matchID).
		Find(&subscriptions)

	if result.Error != nil {
		return nil, result.Error
	}

	return subscriptions, nil
}

func (r *SubscriptionRepository) ListPending(ctx context.Context, matchID uint) ([]Subscription, error) {
	var subscriptions []Subscription
	result := r.db.WithContext(ctx).
		Where("status = ?", PendingSub).
		Where("match_id = ?", matchID).
		Find(&subscriptions)

	if result.Error != nil {
		return nil, result.Error
	}

	return subscriptions, nil
}

func (r *SubscriptionRepository) Update(ctx context.Context, id uint, subscription Subscription) error {
	sub := Subscription{ID: id}
	result := r.db.WithContext(ctx).Model(&sub).Updates(subscription)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
