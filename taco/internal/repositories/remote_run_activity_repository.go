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
