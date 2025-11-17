package tfe

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// PlanExecutor handles real Terraform plan execution
type PlanExecutor struct {
	runRepo       domain.TFERunRepository
	planRepo      domain.TFEPlanRepository
	configVerRepo domain.TFEConfigurationVersionRepository
	blobStore     storage.UnitStore
	unitRepo      domain.UnitRepository
}

// NewPlanExecutor creates a new plan executor
func NewPlanExecutor(
	runRepo domain.TFERunRepository,
	planRepo domain.TFEPlanRepository,
	configVerRepo domain.TFEConfigurationVersionRepository,
	blobStore storage.UnitStore,
	unitRepo domain.UnitRepository,
) *PlanExecutor {
	return &PlanExecutor{
		runRepo:       runRepo,
		planRepo:      planRepo,
		configVerRepo: configVerRepo,
		blobStore:     blobStore,
		unitRepo:      unitRepo,
	}
}

// ExecutePlan executes a Terraform plan for a run
func (e *PlanExecutor) ExecutePlan(ctx context.Context, runID string) error {
	logger := slog.Default().With(
		slog.String("operation", "execute_plan"),
		slog.String("run_id", runID),
	)
	
	logger.Info("starting plan execution")

	// Get run
	run, err := e.runRepo.GetRun(ctx, runID)
	if err != nil {
		logger.Error("failed to get run", slog.String("error", err.Error()))
		return fmt.Errorf("failed to get run: %w", err)
	}
	logger.Info("retrieved run", 
		slog.String("config_version_id", run.ConfigurationVersionID),
		slog.String("unit_id", run.UnitID))

	// Acquire lock before starting terraform operations
	// This prevents concurrent plans/applies on the same unit
	lockInfo := &storage.LockInfo{
		ID:      fmt.Sprintf("tfe-plan-%s", runID),
		Who:     fmt.Sprintf("terraform-plan@run-%s", runID),
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
			return e.handlePlanError(ctx, run.ID, run.PlanID, logger, errMsg)
		}
		logger.Error("failed to acquire lock", slog.String("error", err.Error()))
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, fmt.Sprintf("Failed to acquire lock: %v", err))
	}
	
	logger.Info("unit lock acquired successfully")
	
	// Track whether lock has been manually released (to avoid double-unlock in defer)
	lockReleased := false
	
	// Ensure lock is released when we're done (success or failure)
	defer func() {
		if lockReleased {
			// Lock was manually released (e.g., before spawning apply), skip defer unlock
			return
		}
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

	// Update run status to "planning"
	if err := e.runRepo.UpdateRunStatus(ctx, runID, "planning"); err != nil {
		logger.Error("failed to update status to planning", slog.String("error", err.Error()))
		return fmt.Errorf("failed to update run status: %w", err)
	}
	logger.Info("updated run status to planning")

	// Get configuration version
	configVer, err := e.configVerRepo.GetConfigurationVersion(ctx, run.ConfigurationVersionID)
	if err != nil {
		return fmt.Errorf("failed to get configuration version: %w", err)
	}

	// Check if configuration was uploaded
	if configVer.Status != "uploaded" || configVer.ArchiveBlobID == nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, "Configuration not uploaded")
	}

	// Download configuration archive from blob storage
	archivePath := fmt.Sprintf("config-versions/%s/archive.tar.gz", configVer.ID)
	archiveData, err := e.blobStore.DownloadBlob(ctx, archivePath)
	if err != nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, fmt.Sprintf("Failed to download archive: %v", err))
	}

	logger.Info("downloaded configuration archive", 
		slog.Int("bytes", len(archiveData)),
		slog.String("config_version_id", configVer.ID))

	// Extract to temp directory
	workDir, err := extractArchive(archiveData)
	if err != nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, fmt.Sprintf("Failed to extract archive: %v", err))
	}
	defer cleanupWorkDir(workDir)

	logger.Info("extracted archive", slog.String("work_dir", workDir))

	// Create an override file to disable cloud/remote backend
	// This is required for server-side execution to prevent circular dependencies
	if err := createBackendOverride(workDir); err != nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, fmt.Sprintf("Failed to create backend override: %v", err))
	}

	// Download current state for this unit (if it exists)
	// Construct org-scoped state ID: <orgID>/<unitID>
	stateID := fmt.Sprintf("%s/%s", run.OrgID, run.UnitID)
	stateData, err := e.blobStore.Download(ctx, stateID)
	if err == nil {
		// Write state to terraform.tfstate in the working directory
		statePath := filepath.Join(workDir, "terraform.tfstate")
		if err := os.WriteFile(statePath, stateData, 0644); err != nil {
			logger.Warn("failed to write state file", slog.String("error", err.Error()))
		} else {
			logger.Info("downloaded and wrote existing state", 
				slog.String("state_id", stateID),
				slog.Int("bytes", len(stateData)))
		}
	} else {
		logger.Info("no existing state found, starting fresh", 
			slog.String("state_id", stateID))
	}

	// Run terraform plan
	_, logs, hasChanges, adds, changes, destroys, err := e.runTerraformPlan(ctx, workDir, run.IsDestroy)

	// Store logs in blob storage (use UploadBlob - no lock checks needed for logs)
	logBlobID := fmt.Sprintf("plans/%s/logs.txt", *run.PlanID)
	if err := e.blobStore.UploadBlob(ctx, logBlobID, []byte(logs)); err != nil {
		logger.Error("failed to store plan logs", slog.String("error", err.Error()))
	}

	// Generate signed log URL
	logReadURL := fmt.Sprintf("/tfe/api/v2/plans/%s/logs/logs", *run.PlanID)

	// Update plan with results
	planStatus := "finished"
	if err != nil {
		planStatus = "errored"
		logs = logs + "\n\nError: " + err.Error()
		// Store error in run for user visibility
		if updateErr := e.runRepo.UpdateRunError(ctx, run.ID, err.Error()); updateErr != nil {
			logger.Error("failed to update run error", slog.String("error", updateErr.Error()))
		}
	}

	planUpdates := &domain.TFEPlanUpdate{
		Status:               &planStatus,
		ResourceAdditions:    &adds,
		ResourceChanges:      &changes,
		ResourceDestructions: &destroys,
		HasChanges:           &hasChanges,
		LogBlobID:            &logBlobID,
		LogReadURL:           &logReadURL,
	}


	if err := e.planRepo.UpdatePlan(ctx, *run.PlanID, planUpdates); err != nil {
		return fmt.Errorf("failed to update plan: %w", err)
	}

	// Update run status and can_apply
	// Use "planned" status (not "planned_and_finished") - this is what Terraform CLI expects
	runStatus := "planned"
	canApply := (err == nil) // Can apply if plan succeeded (regardless of whether there are changes)
	
	if err != nil {
		runStatus = "errored"
	}

	logger.Info("updating run status", 
		slog.String("status", runStatus),
		slog.Bool("can_apply", canApply))
	
	if err := e.runRepo.UpdateRunStatusAndCanApply(ctx, run.ID, runStatus, canApply); err != nil {
		logger.Error("failed to update run", slog.String("error", err.Error()))
		return fmt.Errorf("failed to update run: %w", err)
	}

	logger.Info("plan execution completed", 
		slog.String("status", runStatus),
		slog.Bool("can_apply", canApply),
		slog.Bool("has_changes", hasChanges),
		slog.Int("adds", adds),
		slog.Int("changes", changes),
		slog.Int("destroys", destroys))

	// Only auto-trigger apply if AutoApply flag is true (i.e., terraform apply -auto-approve)
	logger.Debug("auto-apply check", 
		slog.Bool("auto_apply", run.AutoApply),
		slog.Bool("plan_succeeded", err == nil))
	
	if run.AutoApply && err == nil {
		logger.Info("triggering auto-apply")
		
		// Queue the apply by updating the run status
		if err := e.runRepo.UpdateRunStatus(ctx, run.ID, "apply_queued"); err != nil {
			logger.Error("failed to queue apply", slog.String("error", err.Error()))
			return nil // Don't fail the plan if we can't queue the apply
		}
		
		// CRITICAL: Release the plan lock BEFORE spawning apply goroutine
		// Otherwise we get a race condition where apply tries to acquire while plan still holds it
		logger.Info("releasing plan lock before triggering apply", slog.String("unit_id", run.UnitID))
		if unlockErr := e.unitRepo.Unlock(ctx, run.UnitID, lockInfo.ID); unlockErr != nil {
			logger.Error("failed to release plan lock before apply", 
				slog.String("error", unlockErr.Error()),
				slog.String("unit_id", run.UnitID))
			return fmt.Errorf("failed to release lock before apply: %w", unlockErr)
		}
		lockReleased = true // Mark as released to prevent defer from trying again
		logger.Info("plan lock released, apply can now acquire it")
		
		// Trigger apply execution in background
		// Use a new context to avoid cancellation propagation issues
		applyCtx, cancel := context.WithCancel(context.Background())
		go func() {
			defer cancel()
			applyLogger := slog.Default().With(
				slog.String("operation", "auto_apply"),
				slog.String("run_id", run.ID),
			)
			applyLogger.Info("starting async apply execution")
			applyExecutor := NewApplyExecutor(e.runRepo, e.planRepo, e.configVerRepo, e.blobStore, e.unitRepo)
			if err := applyExecutor.ExecuteApply(applyCtx, run.ID); err != nil {
				applyLogger.Error("apply execution failed", slog.String("error", err.Error()))
			} else {
				applyLogger.Info("apply execution completed successfully")
			}
		}()
		
		// Return without triggering defer (lock already released)
		return nil
	}

	// Normal case: defer will release the lock
	return nil
}

// runTerraformPlan executes terraform init and plan using terraform-exec
// This provides clean, structured output without local execution indicators
func (e *PlanExecutor) runTerraformPlan(ctx context.Context, workDir string, isDestroy bool) (output string, logs string, hasChanges bool, adds, changes, destroys int, err error) {
	logger := slog.Default().With(slog.String("work_dir", workDir))
	var logBuffer bytes.Buffer

	// Find terraform binary
	terraformPath, err := exec.LookPath("terraform")
	if err != nil {
		return "", "", false, 0, 0, 0, fmt.Errorf("terraform binary not found: %w", err)
	}

	// Create terraform-exec instance
	tf, err := tfexec.NewTerraform(workDir, terraformPath)
	if err != nil {
		return "", "", false, 0, 0, 0, fmt.Errorf("failed to create terraform executor: %w", err)
	}

	// Capture all output to our log buffer (this is clean output, no local indicators!)
	tf.SetStdout(&logBuffer)
	tf.SetStderr(&logBuffer)

	// Run terraform init (backend override file already created to disable cloud/remote backend)
	logger.Info("running terraform init")
	err = tf.Init(ctx, tfexec.Upgrade(false))
	if err != nil {
		logger.Error("terraform init failed", slog.String("error", err.Error()))
		return "", logBuffer.String(), false, 0, 0, 0, fmt.Errorf("terraform init failed: %w", err)
	}

	// Clear init output - HashiCorp TFC doesn't show init to users
	logBuffer.Reset()

	// Run terraform plan WITHOUT -out to avoid "Saved the plan to..." message
	// We'll run it again with -json to get structured data
	logger.Info("running terraform plan for human-readable output")
	
	if isDestroy {
		hasChanges, err = tf.Plan(ctx, tfexec.Destroy(true))
	} else {
		hasChanges, err = tf.Plan(ctx)
	}

	// Get the human-readable logs (clean output, no "saved plan" messages!)
	planLogs := logBuffer.String()

	// Handle plan errors
	if err != nil {
		logger.Error("terraform plan failed", slog.String("error", err.Error()))
		return "", planLogs, false, 0, 0, 0, fmt.Errorf("terraform plan failed: %w", err)
	}

	// Now run again with structured JSON to get resource counts
	// Reset buffer and redirect to discard human output from this run
	logBuffer.Reset()
	var jsonBuffer bytes.Buffer
	
	logger.Info("running terraform plan for structured JSON output")
	planFile := filepath.Join(workDir, "tfplan")

	// Temporarily redirect output so we don't pollute logs with duplicate plan
	tf.SetStdout(&jsonBuffer)
	tf.SetStderr(&jsonBuffer)
	
	if isDestroy {
		_, err = tf.Plan(ctx, tfexec.Destroy(true), tfexec.Out(planFile))
	} else {
		_, err = tf.Plan(ctx, tfexec.Out(planFile))
	}
	
	if err != nil {
		logger.Warn("failed to generate structured plan", slog.String("error", err.Error()))
		// Not fatal - we have the human-readable logs already
	} else {
		// Get structured JSON output from the saved plan
		planStruct, err := tf.ShowPlanFile(ctx, planFile)
		if err != nil {
			logger.Warn("failed to read structured plan", slog.String("error", err.Error()))
			planStruct = nil
		} else {
			// Extract resource counts from structured plan
			if planStruct != nil && planStruct.ResourceChanges != nil {
				for _, rc := range planStruct.ResourceChanges {
					if rc.Change == nil {
						continue
					}
					actions := rc.Change.Actions
					if actions.Create() {
						adds++
					} else if actions.Update() {
						changes++
					} else if actions.Delete() {
						destroys++
					} else if actions.Replace() {
						adds++
						destroys++
		}
	}
			}
		}
	}
	
	hasChanges = adds > 0 || changes > 0 || destroys > 0

	logger.Info("plan completed",
		slog.Bool("has_changes", hasChanges),
		slog.Int("adds", adds),
		slog.Int("changes", changes),
		slog.Int("destroys", destroys))

	return "", planLogs, hasChanges, adds, changes, destroys, nil
}

// handlePlanError handles plan execution errors
func (e *PlanExecutor) handlePlanError(ctx context.Context, runID string, planID *string, logger *slog.Logger, errorMsg string) error {
	logger.Error("plan execution failed", slog.String("error", errorMsg))

	// Update plan status if we have a plan ID
	if planID != nil {
		errStatus := "errored"
		planUpdates := &domain.TFEPlanUpdate{
			Status: &errStatus,
		}
		_ = e.planRepo.UpdatePlan(ctx, *planID, planUpdates)
	}

	// Store error in database so user can see it
	if err := e.runRepo.UpdateRunError(ctx, runID, errorMsg); err != nil {
		logger.Error("failed to update run error in database", slog.String("error", err.Error()))
	}

	return fmt.Errorf("plan execution failed: %s", errorMsg)
}

// extractArchive extracts a tar.gz archive to a temp directory
func extractArchive(data []byte) (string, error) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "terraform-plan-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Create gzip reader
	gzipReader, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Extract all files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("failed to read tar: %w", err)
		}

		// Construct target path
		target := filepath.Join(tempDir, header.Name)

		// Ensure target is within tempDir (security check)
		if !strings.HasPrefix(target, filepath.Clean(tempDir)+string(os.PathSeparator)) {
			os.RemoveAll(tempDir)
			return "", fmt.Errorf("illegal file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, 0755); err != nil {
				os.RemoveAll(tempDir)
				return "", fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directory if needed
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				os.RemoveAll(tempDir)
				return "", fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			outFile, err := os.Create(target)
			if err != nil {
				os.RemoveAll(tempDir)
				return "", fmt.Errorf("failed to create file: %w", err)
			}

			// Copy contents
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				os.RemoveAll(tempDir)
				return "", fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()
		}
	}

	return tempDir, nil
}

// cleanupWorkDir removes the temporary work directory
func cleanupWorkDir(dir string) {
	if dir != "" {
		if err := os.RemoveAll(dir); err != nil {
			slog.Warn("failed to cleanup work directory", 
				slog.String("dir", dir),
				slog.String("error", err.Error()))
		}
	}
}

// createBackendOverride removes cloud/backend configuration from Terraform files
// This is required for server-side execution to prevent circular dependencies
func createBackendOverride(workDir string) error {
	// Walk through all .tf files and remove cloud{} and backend{} blocks from terraform{} blocks
	err := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Only process .tf files
		if !info.IsDir() && strings.HasSuffix(path, ".tf") {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", path, err)
			}
			
			contentStr := string(content)
			
			// Check if file contains terraform block with cloud or backend
			if strings.Contains(contentStr, "cloud") || strings.Contains(contentStr, "backend") {
				slog.Info("removing cloud/backend configuration", slog.String("file", path))
				
				// Comment out cloud and backend blocks
				// This is a simple approach - we comment out lines containing "cloud {" and "backend "
				lines := strings.Split(contentStr, "\n")
				var inBlock bool
				var blockDepth int
				
				for i, line := range lines {
					trimmed := strings.TrimSpace(line)
					
					// Start of cloud or backend block
					if (strings.Contains(trimmed, "cloud {") || strings.Contains(trimmed, "backend ")) && !strings.HasPrefix(trimmed, "#") {
						lines[i] = "# " + line + " # Disabled by TFE executor"
						inBlock = true
						blockDepth = strings.Count(line, "{") - strings.Count(line, "}")
						continue
					}
					
					// Inside block - comment out
					if inBlock {
						blockDepth += strings.Count(line, "{") - strings.Count(line, "}")
						lines[i] = "# " + line
						
						if blockDepth <= 0 {
							inBlock = false
						}
					}
				}
				
				modifiedContent := strings.Join(lines, "\n")
				if err := os.WriteFile(path, []byte(modifiedContent), info.Mode()); err != nil {
					return fmt.Errorf("failed to write %s: %w", path, err)
				}
			}
		}
		
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("failed to process terraform files: %w", err)
	}
	
	slog.Info("successfully removed cloud/backend configuration from terraform files")
	return nil
}
