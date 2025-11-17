package tfe

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// ApplyExecutor handles real Terraform apply execution
type ApplyExecutor struct {
	runRepo       domain.TFERunRepository
	planRepo      domain.TFEPlanRepository
	configVerRepo domain.TFEConfigurationVersionRepository
	blobStore     storage.UnitStore
	unitRepo      domain.UnitRepository
}

// NewApplyExecutor creates a new apply executor
func NewApplyExecutor(
	runRepo domain.TFERunRepository,
	planRepo domain.TFEPlanRepository,
	configVerRepo domain.TFEConfigurationVersionRepository,
	blobStore storage.UnitStore,
	unitRepo domain.UnitRepository,
) *ApplyExecutor {
	return &ApplyExecutor{
		runRepo:       runRepo,
		planRepo:      planRepo,
		configVerRepo: configVerRepo,
		blobStore:     blobStore,
		unitRepo:      unitRepo,
	}
}

// ExecuteApply executes a Terraform apply for a run
func (e *ApplyExecutor) ExecuteApply(ctx context.Context, runID string) error {
	logger := slog.Default().With(
		slog.String("operation", "execute_apply"),
		slog.String("run_id", runID),
	)
	
	logger.Info("starting apply execution")

	// Get run
	run, err := e.runRepo.GetRun(ctx, runID)
	if err != nil {
		logger.Error("failed to get run", slog.String("error", err.Error()))
		return fmt.Errorf("failed to get run: %w", err)
	}

	// Check if run can be applied
	// Allow apply from "planned" (waiting for confirmation) or "apply_queued" status
	if run.Status != "planned" && run.Status != "apply_queued" {
		logger.Error("run cannot be applied", slog.String("status", run.Status))
		return fmt.Errorf("run cannot be applied in status: %s", run.Status)
	}

	// Acquire lock before starting terraform apply
	// This prevents concurrent applies/plans on the same unit
	lockInfo := &storage.LockInfo{
		ID:      fmt.Sprintf("tfe-apply-%s", runID),
		Who:     fmt.Sprintf("terraform-apply@run-%s", runID),
		Version: "1.0.0",
		Created: time.Now(),
	}
	
	logger.Info("acquiring unit lock", 
		slog.String("unit_id", run.UnitID),
		slog.String("lock_id", lockInfo.ID))
	
	if err := e.unitRepo.Lock(ctx, run.UnitID, lockInfo); err != nil {
		if err == storage.ErrLockConflict {
			// Unit is locked by another operation
			currentLock, _ := e.unitRepo.GetLock(ctx, run.UnitID)
			errMsg := fmt.Sprintf("Unit is locked by another operation (locked by: %s). Please wait and try again.", 
				currentLock.Who)
			logger.Warn("lock conflict - unit already locked", 
				slog.String("unit_id", run.UnitID),
				slog.String("locked_by", currentLock.Who),
				slog.String("lock_id", currentLock.ID))
			return e.handleApplyError(ctx, run.ID, logger, errMsg)
		}
		logger.Error("failed to acquire lock", slog.String("error", err.Error()))
		return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to acquire lock: %v", err))
	}
	
	logger.Info("unit lock acquired successfully")
	
	// Ensure lock is released when we're done (success or failure)
	defer func() {
		logger.Info("releasing unit lock", slog.String("unit_id", run.UnitID))
		if unlockErr := e.unitRepo.Unlock(ctx, run.UnitID, lockInfo.ID); unlockErr != nil {
			logger.Error("failed to release lock", 
				slog.String("error", unlockErr.Error()),
				slog.String("unit_id", run.UnitID),
				slog.String("lock_id", lockInfo.ID))
		} else {
			logger.Info("unit lock released successfully")
		}
	}()

	// Update run status to "applying"
	if err := e.runRepo.UpdateRunStatus(ctx, runID, "applying"); err != nil {
		logger.Error("failed to update run status", slog.String("error", err.Error()))
		return fmt.Errorf("failed to update run status: %w", err)
	}
	
	logger.Info("updated run status to applying")

	// Get configuration version
	configVer, err := e.configVerRepo.GetConfigurationVersion(ctx, run.ConfigurationVersionID)
	if err != nil {
		return fmt.Errorf("failed to get configuration version: %w", err)
	}

	// Download configuration archive
	archivePath := fmt.Sprintf("config-versions/%s/archive.tar.gz", configVer.ID)
	archiveData, err := e.blobStore.DownloadBlob(ctx, archivePath)
	if err != nil {
		return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to download archive: %v", err))
	}

	// Extract to temp directory
	workDir, err := extractArchive(archiveData)
	if err != nil {
		return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to extract archive: %v", err))
	}
	defer cleanupWorkDir(workDir)

	logger.Info("extracted archive for apply", slog.String("work_dir", workDir))

	// Remove cloud/backend configuration to prevent circular dependencies
	if err := createBackendOverride(workDir); err != nil {
		return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to remove backend configuration: %v", err))
	}

	// Download current state for this unit (must exist before apply)
	// Construct org-scoped state ID: <orgID>/<unitID>
	stateID := fmt.Sprintf("%s/%s", run.OrgID, run.UnitID)
	stateData, err := e.blobStore.Download(ctx, stateID)
	if err != nil {
		logger.Warn("failed to download state, continuing anyway", 
			slog.String("state_id", stateID),
			slog.String("error", err.Error()))
		// Continue anyway - might be a fresh deployment
	} else {
		// Write state to terraform.tfstate in the working directory
		statePath := filepath.Join(workDir, "terraform.tfstate")
		if err := os.WriteFile(statePath, stateData, 0644); err != nil {
			return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to write state file: %v", err))
		}
		logger.Info("downloaded and wrote existing state", 
			slog.String("state_id", stateID),
			slog.Int("bytes", len(stateData)))
	}

	// Run terraform apply
	logs, err := e.runTerraformApply(ctx, workDir, run.IsDestroy)

	// Store apply logs in blob storage (use UploadBlob - no lock checks needed for logs)
	applyLogBlobID := fmt.Sprintf("runs/%s/apply-logs.txt", run.ID)
	if storeErr := e.blobStore.UploadBlob(ctx, applyLogBlobID, []byte(logs)); storeErr != nil {
		logger.Error("failed to store apply logs", slog.String("error", storeErr.Error()))
	}

	// Update run status
	runStatus := "applied"
	if err != nil {
		runStatus = "errored"
		logs = logs + "\n\nError: " + err.Error()
		// Store error logs even on failure
		_ = e.blobStore.UploadBlob(ctx, applyLogBlobID, []byte(logs))
		// Store error in run for user visibility
		if updateErr := e.runRepo.UpdateRunError(ctx, run.ID, err.Error()); updateErr != nil {
			logger.Error("failed to update run error", slog.String("error", updateErr.Error()))
		}
	} else {
		// Upload the updated state back to storage after successful apply
		// Construct org-scoped state ID: <orgID>/<unitID>
		stateID := fmt.Sprintf("%s/%s", run.OrgID, run.UnitID)
		statePath := filepath.Join(workDir, "terraform.tfstate")
		newStateData, readErr := os.ReadFile(statePath)
		if readErr != nil {
			logger.Warn("failed to read updated state file", slog.String("error", readErr.Error()))
		} else {
			// Upload state with lock ID to unlock it after upload
			if uploadErr := e.blobStore.Upload(ctx, stateID, newStateData, lockInfo.ID); uploadErr != nil {
				logger.Error("failed to upload updated state", 
					slog.String("state_id", stateID),
					slog.String("error", uploadErr.Error()))
				// This is critical - mark as errored
				runStatus = "errored"
				errMsg := fmt.Sprintf("Failed to upload state: %v", uploadErr)
				logs = logs + "\n\nCritical Error: " + errMsg + "\n"
				// Store error in database
				if updateErr := e.runRepo.UpdateRunError(ctx, run.ID, errMsg); updateErr != nil {
					logger.Error("failed to update run error", slog.String("error", updateErr.Error()))
				}
			} else {
				logger.Info("successfully uploaded updated state", 
					slog.String("state_id", stateID),
					slog.Int("bytes", len(newStateData)))
			}
		}
	}

	if err := e.runRepo.UpdateRunStatus(ctx, run.ID, runStatus); err != nil {
		logger.Error("failed to update run status", slog.String("error", err.Error()))
		return fmt.Errorf("failed to update run status: %w", err)
	}

	logger.Info("apply execution completed", slog.String("status", runStatus))

	if err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	return nil
}

// runTerraformApply executes terraform init and apply using terraform-exec
// This provides clean output without local execution indicators
func (e *ApplyExecutor) runTerraformApply(ctx context.Context, workDir string, isDestroy bool) (logs string, err error) {
	logger := slog.Default().With(slog.String("work_dir", workDir))
	var logBuffer bytes.Buffer

	// Find terraform binary
	terraformPath, err := exec.LookPath("terraform")
	if err != nil {
		return "", fmt.Errorf("terraform binary not found: %w", err)
	}

	// Create terraform-exec instance
	tf, err := tfexec.NewTerraform(workDir, terraformPath)
	if err != nil {
		return "", fmt.Errorf("failed to create terraform executor: %w", err)
	}

	// Capture all output to our log buffer
	tf.SetStdout(&logBuffer)
	tf.SetStderr(&logBuffer)

	// Run terraform init (cloud/backend config already removed by createBackendOverride)
	logger.Info("running terraform init")
	err = tf.Init(ctx, tfexec.Upgrade(false))
	if err != nil {
		logger.Error("terraform init failed", slog.String("error", err.Error()))
		return logBuffer.String(), fmt.Errorf("terraform init failed: %w", err)
	}

	// Clear init output - HashiCorp TFC doesn't show init to users
	logBuffer.Reset()

	// Run terraform apply
	logger.Info("running terraform apply", slog.Bool("is_destroy", isDestroy))
	
	if isDestroy {
		err = tf.Destroy(ctx)
	} else {
		err = tf.Apply(ctx)
	}

	// Get the apply logs
	applyLogs := logBuffer.String()

	if err != nil {
		logger.Error("terraform apply failed", slog.String("error", err.Error()))
		return applyLogs, fmt.Errorf("terraform apply failed: %w", err)
	}

	logger.Info("terraform apply completed successfully")
	return applyLogs, nil
}

// handleApplyError handles apply execution errors
func (e *ApplyExecutor) handleApplyError(ctx context.Context, runID string, logger *slog.Logger, errorMsg string) error {
	logger.Error("apply execution failed", slog.String("error", errorMsg))

	// Store error in database so user can see it
	if err := e.runRepo.UpdateRunError(ctx, runID, errorMsg); err != nil {
		logger.Error("failed to update run error in database", slog.String("error", err.Error()))
	}

	return fmt.Errorf("apply execution failed: %s", errorMsg)
}

