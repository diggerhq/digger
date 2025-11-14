package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

// TFEConfigurationVersionRepository manages TFE configuration versions using GORM
type TFEConfigurationVersionRepository struct {
	db *gorm.DB
}

// NewTFEConfigurationVersionRepository creates a new TFE configuration version repository
func NewTFEConfigurationVersionRepository(db *gorm.DB) *TFEConfigurationVersionRepository {
	return &TFEConfigurationVersionRepository{db: db}
}

// CreateConfigurationVersion creates a new configuration version
func (r *TFEConfigurationVersionRepository) CreateConfigurationVersion(ctx context.Context, cv *domain.TFEConfigurationVersion) error {
	fmt.Printf("DEBUG CreateConfigurationVersion: Input cv.ID=%s, cv.UnitID=%s, cv.OrgID=%s\n", cv.ID, cv.UnitID, cv.OrgID)
	
	dbCV := &types.TFEConfigurationVersion{
		ID:               cv.ID,
		OrgID:            cv.OrgID,
		UnitID:           cv.UnitID,
		Status:           cv.Status,
		Source:           cv.Source,
		Speculative:      cv.Speculative,
		AutoQueueRuns:    cv.AutoQueueRuns,
		Provisional:      cv.Provisional,
		Error:            cv.Error,
		ErrorMessage:     cv.ErrorMessage,
		UploadURL:        cv.UploadURL,
		UploadedAt:       cv.UploadedAt,
		ArchiveBlobID:    cv.ArchiveBlobID,
		StatusTimestamps: cv.StatusTimestamps,
		CreatedBy:        cv.CreatedBy,
	}
	
	fmt.Printf("DEBUG Before GORM Create: dbCV.ID=%s, dbCV.UnitID=%s, dbCV.OrgID=%s\n", dbCV.ID, dbCV.UnitID, dbCV.OrgID)

	if err := r.db.WithContext(ctx).Create(dbCV).Error; err != nil {
		fmt.Printf("DEBUG GORM Create ERROR: %v, Generated ID was: %s\n", err, dbCV.ID)
		return fmt.Errorf("failed to create configuration version: %w", err)
	}
	
	fmt.Printf("DEBUG After GORM Create SUCCESS: Generated ID=%s\n", dbCV.ID)

	// Update the domain model with generated timestamps and ID
	cv.ID = dbCV.ID
	cv.CreatedAt = dbCV.CreatedAt
	cv.UpdatedAt = dbCV.UpdatedAt

	return nil
}

// GetConfigurationVersion retrieves a configuration version by ID
func (r *TFEConfigurationVersionRepository) GetConfigurationVersion(ctx context.Context, cvID string) (*domain.TFEConfigurationVersion, error) {
	var dbCV types.TFEConfigurationVersion
	
	if err := r.db.WithContext(ctx).Where("id = ?", cvID).First(&dbCV).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("configuration version not found: %s", cvID)
		}
		return nil, fmt.Errorf("failed to get configuration version: %w", err)
	}

	return &domain.TFEConfigurationVersion{
		ID:               dbCV.ID,
		OrgID:            dbCV.OrgID,
		UnitID:           dbCV.UnitID,
		CreatedAt:        dbCV.CreatedAt,
		UpdatedAt:        dbCV.UpdatedAt,
		Status:           dbCV.Status,
		Source:           dbCV.Source,
		Speculative:      dbCV.Speculative,
		AutoQueueRuns:    dbCV.AutoQueueRuns,
		Provisional:      dbCV.Provisional,
		Error:            dbCV.Error,
		ErrorMessage:     dbCV.ErrorMessage,
		UploadURL:        dbCV.UploadURL,
		UploadedAt:       dbCV.UploadedAt,
		ArchiveBlobID:    dbCV.ArchiveBlobID,
		StatusTimestamps: dbCV.StatusTimestamps,
		CreatedBy:        dbCV.CreatedBy,
	}, nil
}

// UpdateConfigurationVersionStatus updates the status and optionally the uploaded timestamp
func (r *TFEConfigurationVersionRepository) UpdateConfigurationVersionStatus(ctx context.Context, cvID string, status string, uploadedAt *time.Time, archiveBlobID *string) error {
	updateMap := map[string]interface{}{
		"status": status,
	}
	
	if uploadedAt != nil {
		updateMap["uploaded_at"] = *uploadedAt
	}
	
	if archiveBlobID != nil {
		updateMap["archive_blob_id"] = *archiveBlobID
	}

	result := r.db.WithContext(ctx).
		Model(&types.TFEConfigurationVersion{}).
		Where("id = ?", cvID).
		Updates(updateMap)

	if result.Error != nil {
		return fmt.Errorf("failed to update configuration version status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("configuration version not found: %s", cvID)
	}

	return nil
}

// ListConfigurationVersionsForUnit retrieves configuration versions for a specific unit (workspace)
func (r *TFEConfigurationVersionRepository) ListConfigurationVersionsForUnit(ctx context.Context, unitID string, limit int) ([]*domain.TFEConfigurationVersion, error) {
	var dbCVs []types.TFEConfigurationVersion
	
	query := r.db.WithContext(ctx).Where("unit_id = ?", unitID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	if err := query.Find(&dbCVs).Error; err != nil {
		return nil, fmt.Errorf("failed to list configuration versions for unit: %w", err)
	}

	cvs := make([]*domain.TFEConfigurationVersion, len(dbCVs))
	for i, dbCV := range dbCVs {
		cvs[i] = &domain.TFEConfigurationVersion{
			ID:               dbCV.ID,
			OrgID:            dbCV.OrgID,
			UnitID:           dbCV.UnitID,
			CreatedAt:        dbCV.CreatedAt,
			UpdatedAt:        dbCV.UpdatedAt,
			Status:           dbCV.Status,
			Source:           dbCV.Source,
			Speculative:      dbCV.Speculative,
			AutoQueueRuns:    dbCV.AutoQueueRuns,
			Provisional:      dbCV.Provisional,
			Error:            dbCV.Error,
			ErrorMessage:     dbCV.ErrorMessage,
			UploadURL:        dbCV.UploadURL,
			UploadedAt:       dbCV.UploadedAt,
			ArchiveBlobID:    dbCV.ArchiveBlobID,
			StatusTimestamps: dbCV.StatusTimestamps,
			CreatedBy:        dbCV.CreatedBy,
		}
	}

	return cvs, nil
}

