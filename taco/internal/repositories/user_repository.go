package repositories

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

// userRepository implements UserRepository using GORM
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepository{db: db}
}

// NewUserRepositoryFromQueryStore creates a repository from a query store
func NewUserRepositoryFromQueryStore(queryStore interface{}) domain.UserRepository {
	db := GetDBFromQueryStore(queryStore)
	if db == nil {
		return nil
	}
	return NewUserRepository(db)
}

// EnsureUser creates a user if it doesn't exist, or returns existing user (idempotent)
func (r *userRepository) EnsureUser(ctx context.Context, subject, email string) (*domain.User, error) {
	var entity types.User
	err := r.db.WithContext(ctx).Where("subject = ?", subject).First(&entity).Error
	
	if err == nil {
		return &domain.User{
			ID:        entity.ID,
			Subject:   entity.Subject,
			Email:     entity.Email,
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		}, nil
	}
	
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	
	now := time.Now()
	entity = types.User{
		Subject:   subject,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
	
	if err := r.db.WithContext(ctx).Create(&entity).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	slog.Info("User created", "subject", subject, "email", email)
	
	return &domain.User{
		ID:        entity.ID,
		Subject:   entity.Subject,
		Email:     entity.Email,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}, nil
}

// Get retrieves a user by subject
func (r *userRepository) Get(ctx context.Context, subject string) (*domain.User, error) {
	var entity types.User
	err := r.db.WithContext(ctx).Where("subject = ?", subject).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return &domain.User{
		ID:        entity.ID,
		Subject:   entity.Subject,
		Email:     entity.Email,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}, nil
}

// GetByEmail retrieves a user by email
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var entity types.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	
	return &domain.User{
		ID:        entity.ID,
		Subject:   entity.Subject,
		Email:     entity.Email,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}, nil
}

// List returns all users
func (r *userRepository) List(ctx context.Context) ([]*domain.User, error) {
	var entities []*types.User
	err := r.db.WithContext(ctx).Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	
	users := make([]*domain.User, len(entities))
	for i, entity := range entities {
		users[i] = &domain.User{
			ID:        entity.ID,
			Subject:   entity.Subject,
			Email:     entity.Email,
			CreatedAt: entity.CreatedAt,
			UpdatedAt: entity.UpdatedAt,
		}
	}
	
	return users, nil
}

