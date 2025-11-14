package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

// TFERunRepository manages TFE runs using GORM
type TFERunRepository struct {
	db *gorm.DB
}

// NewTFERunRepository creates a new TFE run repository
func NewTFERunRepository(db *gorm.DB) *TFERunRepository {
	return &TFERunRepository{db: db}
}

// CreateRun creates a new TFE run
func (r *TFERunRepository) CreateRun(ctx context.Context, run *domain.TFERun) error {
	dbRun := &types.TFERun{
		ID:                     run.ID,
		OrgID:                  run.OrgID,
		UnitID:                 run.UnitID,
		Status:                 run.Status,
		IsDestroy:              run.IsDestroy,
		Message:                run.Message,
		PlanOnly:               run.PlanOnly,
		AutoApply:              run.AutoApply,
		Source:                 run.Source,
		IsCancelable:           run.IsCancelable,
		CanApply:               run.CanApply,
		ConfigurationVersionID: run.ConfigurationVersionID,
		PlanID:                 run.PlanID,
		ApplyID:                run.ApplyID,
		ApplyLogBlobID:         run.ApplyLogBlobID,
		CreatedBy:              run.CreatedBy,
	}

	// Create the run - but GORM may not respect false for boolean fields with database defaults
	if err := r.db.WithContext(ctx).Create(dbRun).Error; err != nil {
		return fmt.Errorf("failed to create run: %w", err)
	}
	
	// Explicitly update PlanOnly if it's false (workaround for database default constraint)
	if !run.PlanOnly {
		if err := r.db.WithContext(ctx).Model(&types.TFERun{}).Where("id = ?", dbRun.ID).Update("plan_only", false).Error; err != nil {
			return fmt.Errorf("failed to update plan_only field: %w", err)
		}
		fmt.Printf("[CreateRun] Explicitly set PlanOnly=false for run %s\n", dbRun.ID)
	}

	// Update the domain model with generated ID and timestamps
	run.ID = dbRun.ID
	run.CreatedAt = dbRun.CreatedAt
	run.UpdatedAt = dbRun.UpdatedAt
	
	fmt.Printf("[CreateRun] Created run: ID=%s, PlanOnly=%v\n", run.ID, run.PlanOnly)

	return nil
}

// GetRun retrieves a run by ID
func (r *TFERunRepository) GetRun(ctx context.Context, runID string) (*domain.TFERun, error) {
	var dbRun types.TFERun
	
	if err := r.db.WithContext(ctx).Where("id = ?", runID).First(&dbRun).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("run not found: %s", runID)
		}
		return nil, fmt.Errorf("failed to get run: %w", err)
	}

	return &domain.TFERun{
		ID:                     dbRun.ID,
		OrgID:                  dbRun.OrgID,
		UnitID:                 dbRun.UnitID,
		CreatedAt:              dbRun.CreatedAt,
		UpdatedAt:              dbRun.UpdatedAt,
		Status:                 dbRun.Status,
		IsDestroy:              dbRun.IsDestroy,
		Message:                dbRun.Message,
		PlanOnly:               dbRun.PlanOnly,
		AutoApply:              dbRun.AutoApply,
		Source:                 dbRun.Source,
		IsCancelable:           dbRun.IsCancelable,
		CanApply:               dbRun.CanApply,
		ConfigurationVersionID: dbRun.ConfigurationVersionID,
		PlanID:                 dbRun.PlanID,
		ApplyID:                dbRun.ApplyID,
		ApplyLogBlobID:         dbRun.ApplyLogBlobID,
		CreatedBy:              dbRun.CreatedBy,
	}, nil
}

// ListRunsForUnit retrieves runs for a specific unit (workspace)
func (r *TFERunRepository) ListRunsForUnit(ctx context.Context, unitID string, limit int) ([]*domain.TFERun, error) {
	var dbRuns []types.TFERun
	
	query := r.db.WithContext(ctx).Where("unit_id = ?", unitID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	if err := query.Find(&dbRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to list runs for unit: %w", err)
	}

	runs := make([]*domain.TFERun, len(dbRuns))
	for i, dbRun := range dbRuns {
		runs[i] = &domain.TFERun{
			ID:                     dbRun.ID,
			OrgID:                  dbRun.OrgID,
			UnitID:                 dbRun.UnitID,
			CreatedAt:              dbRun.CreatedAt,
			UpdatedAt:              dbRun.UpdatedAt,
			Status:                 dbRun.Status,
			IsDestroy:              dbRun.IsDestroy,
			Message:                dbRun.Message,
			PlanOnly:               dbRun.PlanOnly,
			Source:                 dbRun.Source,
			IsCancelable:           dbRun.IsCancelable,
			CanApply:               dbRun.CanApply,
			ConfigurationVersionID: dbRun.ConfigurationVersionID,
			PlanID:                 dbRun.PlanID,
			ApplyID:                dbRun.ApplyID,
			CreatedBy:              dbRun.CreatedBy,
		}
	}

	return runs, nil
}

// UpdateRunStatus updates the status of a run
func (r *TFERunRepository) UpdateRunStatus(ctx context.Context, runID string, status string) error {
	fmt.Printf("[UpdateRunStatus] Attempting to update run %s to status '%s'\n", runID, status)
	
	result := r.db.WithContext(ctx).
		Model(&types.TFERun{}).
		Where("id = ?", runID).
		Update("status", status)

	if result.Error != nil {
		fmt.Printf("[UpdateRunStatus] ERROR: DB error for run %s: %v\n", runID, result.Error)
		return fmt.Errorf("failed to update run status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		fmt.Printf("[UpdateRunStatus] ERROR: No rows affected for run %s (not found)\n", runID)
		return fmt.Errorf("run not found: %s", runID)
	}

	fmt.Printf("[UpdateRunStatus] ✅ Successfully updated run %s to '%s' (%d rows affected)\n", runID, status, result.RowsAffected)
	return nil
}

// UpdateRunPlanID updates the plan ID of a run
func (r *TFERunRepository) UpdateRunPlanID(ctx context.Context, runID string, planID string) error {
	result := r.db.WithContext(ctx).
		Model(&types.TFERun{}).
		Where("id = ?", runID).
		Update("plan_id", planID)

	if result.Error != nil {
		return fmt.Errorf("failed to update run plan ID: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("run not found: %s", runID)
	}

	return nil
}

// UpdateRunStatusAndCanApply updates both status and can_apply fields
func (r *TFERunRepository) UpdateRunStatusAndCanApply(ctx context.Context, runID string, status string, canApply bool) error {
	fmt.Printf("[UpdateRunStatusAndCanApply] Updating run %s to status='%s', canApply=%v\n", runID, status, canApply)
	
	result := r.db.WithContext(ctx).
		Model(&types.TFERun{}).
		Where("id = ?", runID).
		Updates(map[string]interface{}{
			"status":    status,
			"can_apply": canApply,
		})

	if result.Error != nil {
		fmt.Printf("[UpdateRunStatusAndCanApply] ERROR: %v\n", result.Error)
		return fmt.Errorf("failed to update run: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		fmt.Printf("[UpdateRunStatusAndCanApply] ERROR: Run not found\n")
		return fmt.Errorf("run not found: %s", runID)
	}

	fmt.Printf("[UpdateRunStatusAndCanApply] ✅ Updated run %s\n", runID)
	return nil
}

