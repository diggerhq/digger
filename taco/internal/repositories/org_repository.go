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
func (r *orgRepository) Create(ctx context.Context, name, displayName, createdBy string) (*domain.Organization, error) {
	now := time.Now()
	entity := &types.Organization{
		Name:        name,
		DisplayName: displayName,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	slog.Info("Organization created",
		"uuid", entity.ID,
		"name", name,
		"displayName", displayName,
	)

	return &domain.Organization{
		ID:          entity.ID,
		Name:        entity.Name,
		ExternalID:  entity.ExternalID,
		DisplayName: entity.DisplayName,
		CreatedBy:   entity.CreatedBy,
		CreatedAt:   entity.CreatedAt,
		UpdatedAt:   entity.UpdatedAt,
	}, nil
}

// Get retrieves an organization by UUID
func (r *orgRepository) Get(ctx context.Context, orgUUID string) (*domain.Organization, error) {
	var entity types.Organization
	err := r.db.WithContext(ctx).Where("id = ?", orgUUID).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrOrgNotFound
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return &domain.Organization{
		ID:          entity.ID,
		Name:        entity.Name,
		ExternalID:  entity.ExternalID,
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

	orgs := make([]*domain.Organization, len(entities))
	for i, entity := range entities {
		orgs[i] = &domain.Organization{
			ID:          entity.ID,
			Name:        entity.Name,
			ExternalID:  entity.ExternalID,
			DisplayName: entity.DisplayName,
			CreatedBy:   entity.CreatedBy,
			CreatedAt:   entity.CreatedAt,
			UpdatedAt:   entity.UpdatedAt,
		}
	}

	return orgs, nil
}

// Delete deletes an organization by UUID
func (r *orgRepository) Delete(ctx context.Context, orgUUID string) error {
	result := r.db.WithContext(ctx).Where("id = ?", orgUUID).Delete(&types.Organization{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete organization: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrOrgNotFound
	}

	slog.Info("Organization deleted", "uuid", orgUUID)
	return nil
}

func (r *orgRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context, txRepo domain.OrganizationRepository) error) error {
    return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        // Create a new repository instance using the transaction
        txRepo := &orgRepository{db: tx}
        return fn(ctx, txRepo)
    })
}

// EnsureSystemOrganization creates the system org if it doesn't exist (for auth-disabled mode)
func EnsureSystemOrganization(ctx context.Context, db *gorm.DB, systemOrgID string) error {
	now := time.Now()
	org := types.Organization{
		ID:          systemOrgID,
		Name:        "system",
		DisplayName: "System Organization",
		CreatedBy:   "system",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	
	// Use FirstOrCreate to be idempotent
	result := db.WithContext(ctx).Where(types.Organization{ID: systemOrgID}).FirstOrCreate(&org)
	return result.Error
}
