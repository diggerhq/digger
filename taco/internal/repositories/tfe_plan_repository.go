package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

// TFEPlanRepository manages TFE plans using GORM
type TFEPlanRepository struct {
	db *gorm.DB
}

// NewTFEPlanRepository creates a new TFE plan repository
func NewTFEPlanRepository(db *gorm.DB) *TFEPlanRepository {
	return &TFEPlanRepository{db: db}
}

// CreatePlan creates a new TFE plan
func (r *TFEPlanRepository) CreatePlan(ctx context.Context, plan *domain.TFEPlan) error {
	dbPlan := &types.TFEPlan{
		ID:                   plan.ID,
		OrgID:                plan.OrgID,
		RunID:                plan.RunID,
		Status:               plan.Status,
		ResourceAdditions:    plan.ResourceAdditions,
		ResourceChanges:      plan.ResourceChanges,
		ResourceDestructions: plan.ResourceDestructions,
		HasChanges:           plan.HasChanges,
		LogBlobID:            plan.LogBlobID,
		LogReadURL:           plan.LogReadURL,
		PlanOutputBlobID:     plan.PlanOutputBlobID,
		PlanOutputJSON:       plan.PlanOutputJSON,
		CreatedBy:            plan.CreatedBy,
	}

	if err := r.db.WithContext(ctx).Create(dbPlan).Error; err != nil {
		return fmt.Errorf("failed to create plan: %w", err)
	}

	// Update the domain model with generated ID and timestamps
	plan.ID = dbPlan.ID
	plan.CreatedAt = dbPlan.CreatedAt
	plan.UpdatedAt = dbPlan.UpdatedAt

	return nil
}

// GetPlan retrieves a plan by ID
func (r *TFEPlanRepository) GetPlan(ctx context.Context, planID string) (*domain.TFEPlan, error) {
	var dbPlan types.TFEPlan
	
	if err := r.db.WithContext(ctx).Where("id = ?", planID).First(&dbPlan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("plan not found: %s", planID)
		}
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	return &domain.TFEPlan{
		ID:                   dbPlan.ID,
		OrgID:                dbPlan.OrgID,
		RunID:                dbPlan.RunID,
		CreatedAt:            dbPlan.CreatedAt,
		UpdatedAt:            dbPlan.UpdatedAt,
		Status:               dbPlan.Status,
		ResourceAdditions:    dbPlan.ResourceAdditions,
		ResourceChanges:      dbPlan.ResourceChanges,
		ResourceDestructions: dbPlan.ResourceDestructions,
		HasChanges:           dbPlan.HasChanges,
		LogBlobID:            dbPlan.LogBlobID,
		LogReadURL:           dbPlan.LogReadURL,
		PlanOutputBlobID:     dbPlan.PlanOutputBlobID,
		PlanOutputJSON:       dbPlan.PlanOutputJSON,
		CreatedBy:            dbPlan.CreatedBy,
	}, nil
}

// GetPlanByRunID retrieves a plan by run ID
func (r *TFEPlanRepository) GetPlanByRunID(ctx context.Context, runID string) (*domain.TFEPlan, error) {
	var dbPlan types.TFEPlan
	
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).First(&dbPlan).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("plan not found for run: %s", runID)
		}
		return nil, fmt.Errorf("failed to get plan by run ID: %w", err)
	}

	return &domain.TFEPlan{
		ID:                   dbPlan.ID,
		OrgID:                dbPlan.OrgID,
		RunID:                dbPlan.RunID,
		CreatedAt:            dbPlan.CreatedAt,
		UpdatedAt:            dbPlan.UpdatedAt,
		Status:               dbPlan.Status,
		ResourceAdditions:    dbPlan.ResourceAdditions,
		ResourceChanges:      dbPlan.ResourceChanges,
		ResourceDestructions: dbPlan.ResourceDestructions,
		HasChanges:           dbPlan.HasChanges,
		LogBlobID:            dbPlan.LogBlobID,
		LogReadURL:           dbPlan.LogReadURL,
		PlanOutputBlobID:     dbPlan.PlanOutputBlobID,
		PlanOutputJSON:       dbPlan.PlanOutputJSON,
		CreatedBy:            dbPlan.CreatedBy,
	}, nil
}

// UpdatePlan updates a plan with the provided fields
func (r *TFEPlanRepository) UpdatePlan(ctx context.Context, planID string, updates *domain.TFEPlanUpdate) error {
	// Build update map dynamically based on non-nil fields
	updateMap := make(map[string]interface{})
	
	if updates.Status != nil {
		updateMap["status"] = *updates.Status
	}
	if updates.ResourceAdditions != nil {
		updateMap["resource_additions"] = *updates.ResourceAdditions
	}
	if updates.ResourceChanges != nil {
		updateMap["resource_changes"] = *updates.ResourceChanges
	}
	if updates.ResourceDestructions != nil {
		updateMap["resource_destructions"] = *updates.ResourceDestructions
	}
	if updates.HasChanges != nil {
		updateMap["has_changes"] = *updates.HasChanges
	}
	if updates.LogBlobID != nil {
		updateMap["log_blob_id"] = *updates.LogBlobID
	}
	if updates.LogReadURL != nil {
		updateMap["log_read_url"] = *updates.LogReadURL
	}
	if updates.PlanOutputBlobID != nil {
		updateMap["plan_output_blob_id"] = *updates.PlanOutputBlobID
	}
	if updates.PlanOutputJSON != nil {
		updateMap["plan_output_json"] = *updates.PlanOutputJSON
	}

	if len(updateMap) == 0 {
		return nil // Nothing to update
	}

	result := r.db.WithContext(ctx).
		Model(&types.TFEPlan{}).
		Where("id = ?", planID).
		Updates(updateMap)

	if result.Error != nil {
		return fmt.Errorf("failed to update plan: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("plan not found: %s", planID)
	}

	return nil
}

