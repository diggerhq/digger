package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

type RemoteRunActivityRepository struct {
	db *gorm.DB
}

func NewRemoteRunActivityRepository(db *gorm.DB) *RemoteRunActivityRepository {
	return &RemoteRunActivityRepository{db: db}
}

func (r *RemoteRunActivityRepository) CreateActivity(ctx context.Context, activity *domain.RemoteRunActivity) (string, error) {
	record := &types.RemoteRunActivity{
		ID:              activity.ID,
		RunID:           activity.RunID,
		OrgID:           activity.OrgID,
		UnitID:          activity.UnitID,
		Operation:       activity.Operation,
		Status:          activity.Status,
		TriggeredBy:     activity.TriggeredBy,
		TriggeredSource: activity.TriggeredSource,
		SandboxProvider: activity.SandboxProvider,
		SandboxJobID:    activity.SandboxJobID,
		StartedAt:       activity.StartedAt,
		CompletedAt:     activity.CompletedAt,
		DurationMs:      activity.DurationMS,
		ErrorMessage:    activity.ErrorMessage,
	}

	if record.Status == "" {
		record.Status = "pending"
	}

	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return "", fmt.Errorf("failed to create remote run activity: %w", err)
	}

	return record.ID, nil
}

func (r *RemoteRunActivityRepository) MarkRunning(ctx context.Context, activityID string, startedAt time.Time, sandboxProvider string) error {
	result := r.db.WithContext(ctx).Model(&types.RemoteRunActivity{}).
		Where("id = ?", activityID).
		Updates(map[string]interface{}{
			"status":           "running",
			"started_at":       startedAt,
			"sandbox_provider": sandboxProvider,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to mark remote run activity as running: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("remote run activity not found: %s", activityID)
	}
	return nil
}

func (r *RemoteRunActivityRepository) MarkCompleted(
	ctx context.Context,
	activityID string,
	status string,
	completedAt time.Time,
	duration time.Duration,
	sandboxJobID *string,
	errorMessage *string,
) error {
	updates := map[string]interface{}{
		"status":       status,
		"completed_at": completedAt,
	}

	if duration > 0 {
		updates["duration_ms"] = duration.Milliseconds()
	} else {
		updates["duration_ms"] = nil
	}

	if sandboxJobID != nil {
		updates["sandbox_job_id"] = *sandboxJobID
	}
	if errorMessage != nil {
		updates["error_message"] = *errorMessage
	}

	result := r.db.WithContext(ctx).Model(&types.RemoteRunActivity{}).
		Where("id = ?", activityID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to mark remote run activity as completed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("remote run activity not found: %s", activityID)
	}
	return nil
}

func (r *RemoteRunActivityRepository) ListActivities(ctx context.Context, filters domain.ActivityFilters) ([]*domain.RemoteRunActivity, error) {
	query := r.db.WithContext(ctx).Model(&types.RemoteRunActivity{})

	// Apply filters
	query = query.Where("org_id = ?", filters.OrgID)

	if filters.UnitID != nil {
		query = query.Where("unit_id = ?", *filters.UnitID)
	}
	if filters.Status != nil {
		query = query.Where("status = ?", *filters.Status)
	}
	if filters.StartDate != nil {
		query = query.Where("created_at >= ?", *filters.StartDate)
	}
	if filters.EndDate != nil {
		query = query.Where("created_at <= ?", *filters.EndDate)
	}

	// Pagination
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	} else {
		query = query.Limit(100) // Default limit
	}
	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}

	// Order by most recent first
	query = query.Order("created_at DESC")

	var records []types.RemoteRunActivity
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to list remote run activities: %w", err)
	}

	// Convert to domain models
	activities := make([]*domain.RemoteRunActivity, len(records))
	for i, record := range records {
		activities[i] = &domain.RemoteRunActivity{
			ID:              record.ID,
			RunID:           record.RunID,
			OrgID:           record.OrgID,
			UnitID:          record.UnitID,
			Operation:       record.Operation,
			Status:          record.Status,
			TriggeredBy:     record.TriggeredBy,
			TriggeredSource: record.TriggeredSource,
			SandboxProvider: record.SandboxProvider,
			SandboxJobID:    record.SandboxJobID,
			StartedAt:       record.StartedAt,
			CompletedAt:     record.CompletedAt,
			DurationMS:      record.DurationMs,
			ErrorMessage:    record.ErrorMessage,
			CreatedAt:       record.CreatedAt,
			UpdatedAt:       record.UpdatedAt,
		}
	}

	return activities, nil
}

func (r *RemoteRunActivityRepository) GetUsageSummary(ctx context.Context, orgID string, startDate, endDate *time.Time) (*domain.UsageSummary, error) {
	query := r.db.WithContext(ctx).Model(&types.RemoteRunActivity{}).
		Where("org_id = ?", orgID)

	if startDate != nil {
		query = query.Where("created_at >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("created_at <= ?", *endDate)
	}

	var records []types.RemoteRunActivity
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to get usage summary: %w", err)
	}

	summary := &domain.UsageSummary{
		ByOperation: make(map[string]int),
		ByUnit:      make(map[string]float64),
	}

	for _, record := range records {
		summary.TotalRuns++

		if record.Status == "succeeded" {
			summary.SuccessfulRuns++
		} else if record.Status == "failed" {
			summary.FailedRuns++
		}

		// Count by operation
		summary.ByOperation[record.Operation]++

		// Sum minutes by unit
		if record.DurationMs != nil && *record.DurationMs > 0 {
			minutes := float64(*record.DurationMs) / 60000.0
			summary.TotalMinutes += minutes
			summary.ByUnit[record.UnitID] += minutes
		}
	}

	// Estimate cost at $0.10 per minute (adjust as needed)
	summary.EstimatedCostUSD = summary.TotalMinutes * 0.10

	return summary, nil
}
