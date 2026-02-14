package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/andrewshostak/result-service/internal/app/models"
	"gorm.io/gorm"
)

type SubscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) Create(ctx context.Context, subscription models.Subscription) (*models.Subscription, error) {
	s := Subscription{
		Url:     subscription.Url,
		MatchID: subscription.MatchID,
		Key:     subscription.Key,
	}
	result := r.db.WithContext(ctx).Create(&s)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrForeignKeyViolated) {
			return nil, models.NewUnprocessableContentError(fmt.Errorf("match id does not exist: %w", result.Error))
		}
		if isDuplicateError(result.Error) {
			return nil, models.NewResourceAlreadyExistsError(fmt.Errorf("subscription already exists: %w", result.Error))
		}

		return nil, fmt.Errorf("failed to create subscription: %w", result.Error)
	}

	domain := toDomainSubscription(s)
	return &domain, nil
}

func (r *SubscriptionRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&Subscription{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete subscription: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("subscription doesn't exist")
	}

	return nil
}

func (r *SubscriptionRepository) Get(ctx context.Context, id uint) (*models.Subscription, error) {
	var subscription Subscription
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&subscription)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, models.NewResourceNotFoundError(fmt.Errorf("subscription with id %d not found: %w", id, result.Error))
		}
		return nil, fmt.Errorf("failed to get subscription by id: %w", result.Error)
	}

	domain := toDomainSubscription(subscription)
	return &domain, nil
}

func (r *SubscriptionRepository) One(ctx context.Context, matchID uint, key string, baseURL string) (*models.Subscription, error) {
	var subscription Subscription
	result := r.db.WithContext(ctx).
		Where("match_id = ?", matchID).
		Where("url LIKE ?", baseURL+"%").
		Where("key = ?", key).
		First(&subscription)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, models.NewResourceNotFoundError(fmt.Errorf("subscription is not found: %w", result.Error))
		}

		return nil, fmt.Errorf("failed to find subscription: %w", result.Error)
	}

	domain := toDomainSubscription(subscription)
	return &domain, nil
}

func (r *SubscriptionRepository) List(ctx context.Context, matchID uint) ([]models.Subscription, error) {
	var subscriptions []Subscription
	result := r.db.WithContext(ctx).
		Where("match_id = ?", matchID).
		Find(&subscriptions)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to list subscriptions by match id: %w", result.Error)
	}

	return toDomainSubscriptions(subscriptions), nil
}

func (r *SubscriptionRepository) ListByMatchAndStatus(ctx context.Context, matchID uint, status models.SubscriptionStatus) ([]models.Subscription, error) {
	var subscriptions []Subscription
	result := r.db.WithContext(ctx).
		Where("status = ?", status).
		Where("match_id = ?", matchID).
		Find(&subscriptions)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to list subscriptions by match id and status: %w", result.Error)
	}

	return toDomainSubscriptions(subscriptions), nil
}

func (r *SubscriptionRepository) Update(ctx context.Context, id uint, subscription models.Subscription) error {
	sub := Subscription{ID: id}
	s := Subscription{
		Status:          string(subscription.Status),
		SubscriberError: subscription.SubscriberError,
		NotifiedAt:      subscription.NotifiedAt,
	}
	result := r.db.WithContext(ctx).Model(&sub).Select("Status", "NotifiedAt", "SubscriberError").Updates(s)
	if result.Error != nil {
		return fmt.Errorf("failed to update subscription: %w", result.Error)
	}

	return nil
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
