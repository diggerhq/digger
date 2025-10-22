package repositories

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

const (
	queryOrgByName = "name = ?"
)

// orgRepository implements OrganizationRepository using GORM
// This is where the infrastructure concerns live - hidden from domain and handlers
type orgRepository struct {
	db *gorm.DB
}

// NewOrgRepository creates a new organization repository
// Takes a GORM database connection as infrastructure dependency
func NewOrgRepository(db *gorm.DB) domain.OrganizationRepository {
	return &orgRepository{db: db}
}

// NewOrgRepositoryFromQueryStore creates a repository from a query store
// This helper extracts the underlying database connection from the query store
func NewOrgRepositoryFromQueryStore(queryStore interface{}) domain.OrganizationRepository {
	db := GetDBFromQueryStore(queryStore)
	if db == nil {
		return nil
	}
	return NewOrgRepository(db)
}

// Create creates a new organization
func (r *orgRepository) Create(ctx context.Context, orgID, name, createdBy string) (*domain.Organization, error) {
	// Normalize org ID to lowercase for case-insensitivity
	orgID = strings.ToLower(strings.TrimSpace(orgID))
	
	// Validate org ID format (domain logic)
	if err := domain.ValidateOrgID(orgID); err != nil {
		return nil, err
	}

	// Check if org already exists (infrastructure logic)
	var existing types.Organization
	err := r.db.WithContext(ctx).Where(queryOrgByName, orgID).First(&existing).Error
	if err == nil {
		return nil, domain.ErrOrgExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing org: %w", err)
	}

	// Create new org entity
	now := time.Now()
	entity := &types.Organization{
		Name:        orgID,
		DisplayName: name,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	slog.Info("Organization created successfully",
		"orgID", orgID,
		"name", name,
		"createdBy", createdBy,
	)

	// Convert entity to domain model
	return &domain.Organization{
		ID:          entity.ID,
		Name:        entity.Name,
		DisplayName: entity.DisplayName,
		CreatedBy:   entity.CreatedBy,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}, nil
}

// Get retrieves an organization by ID
func (r *orgRepository) Get(ctx context.Context, orgID string) (*domain.Organization, error) {
	var entity types.Organization
	err := r.db.WithContext(ctx).Where(queryOrgByName, orgID).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOrgNotFound
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// Convert entity to domain model
	return &domain.Organization{
		ID:          entity.ID,
		Name:        entity.Name,
		DisplayName: entity.DisplayName,
		CreatedBy:   entity.CreatedBy,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}, nil
}

// List returns all organizations
func (r *orgRepository) List(ctx context.Context) ([]*domain.Organization, error) {
	var entities []*types.Organization
	err := r.db.WithContext(ctx).Find(&entities).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}

	// Convert entities to domain models
	orgs := make([]*domain.Organization, len(entities))
	for i, entity := range entities {
		orgs[i] = &domain.Organization{
			ID:          entity.ID,
			Name:        entity.Name,
			DisplayName: entity.DisplayName,
			CreatedBy:   entity.CreatedBy,
			CreatedAt:   entity.CreatedAt,
			UpdatedAt:   entity.UpdatedAt,
		}
	}

	return orgs, nil
}

// Delete deletes an organization
func (r *orgRepository) Delete(ctx context.Context, orgID string) error {
	result := r.db.WithContext(ctx).Where(queryOrgByName, orgID).Delete(&types.Organization{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete organization: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrOrgNotFound
	}

	slog.Info("Organization deleted", "orgID", orgID)
	return nil
}

func (r *orgRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context, txRepo domain.OrganizationRepository) error) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // Create a new repository instance using the transaction
        txRepo := &orgRepository{db: tx}
        return fn(ctx, txRepo)
    })
}

