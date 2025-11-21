package tfe

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/sandbox"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/hashicorp/terraform-exec/tfexec"
)

// ApplyExecutor handles real Terraform apply execution
type ApplyExecutor struct {
	runRepo       domain.TFERunRepository
	planRepo      domain.TFEPlanRepository
	configVerRepo domain.TFEConfigurationVersionRepository
	blobStore     storage.UnitStore
	unitRepo      domain.UnitRepository
	sandbox       sandbox.Sandbox
	activityRepo  domain.RemoteRunActivityRepository
}

// NewApplyExecutor creates a new apply executor
func NewApplyExecutor(
	runRepo domain.TFERunRepository,
	planRepo domain.TFEPlanRepository,
	configVerRepo domain.TFEConfigurationVersionRepository,
	blobStore storage.UnitStore,
	unitRepo domain.UnitRepository,
	sandboxProvider sandbox.Sandbox,
	activityRepo domain.RemoteRunActivityRepository,
) *ApplyExecutor {
	return &ApplyExecutor{
		runRepo:       runRepo,
		planRepo:      planRepo,
		configVerRepo: configVerRepo,
		blobStore:     blobStore,
		unitRepo:      unitRepo,
		sandbox:       sandboxProvider,
		activityRepo:  activityRepo,
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

	unitMeta, err := e.unitRepo.Get(ctx, run.UnitID)
	if err != nil {
		logger.Error("failed to load unit metadata", slog.String("error", err.Error()))
		return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to load workspace metadata: %v", err))
	}

	useSandbox := requiresSandbox(unitMeta)
	var applyActivityID string
	var applyActivityStart time.Time
	var applySandboxResult *sandbox.ApplyResult
	if useSandbox && e.activityRepo != nil {
		activity := &domain.RemoteRunActivity{
			RunID:           run.ID,
			OrgID:           run.OrgID,
			UnitID:          run.UnitID,
			Operation:       "apply",
			Status:          "pending",
			TriggeredBy:     run.CreatedBy,
			TriggeredSource: run.Source,
		}
		if id, err := e.activityRepo.CreateActivity(ctx, activity); err != nil {
			logger.Warn("failed to create remote apply activity", slog.String("error", err.Error()))
		} else {
			applyActivityID = id
		}
	}
	if useSandbox && e.sandbox == nil {
		return e.handleApplyError(ctx, run.ID, logger, "Workspace execution mode is remote, but no sandbox provider is configured")
	}

	// Check if run can be applied
	// Allow apply from "planned" (waiting for confirmation) or "apply_queued" status
	if run.Status != "planned" && run.Status != "apply_queued" {
		logger.Error("run cannot be applied", slog.String("status", run.Status))
		return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Run cannot be applied in status: %s", run.Status))
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

	// Buffered logging to reduce blob storage roundtrips
	applyLogBlobID := fmt.Sprintf("runs/%s/apply-logs.txt", run.ID)
	var logBuffer bytes.Buffer
	var logMutex sync.Mutex
	lastLogFlush := time.Now()
	lastFlushSize := 0
	
	flushLogs := func() error {
		logMutex.Lock()
		defer logMutex.Unlock()
		if logBuffer.Len() == 0 {
			return nil
		}
		err := e.blobStore.UploadBlob(ctx, applyLogBlobID, logBuffer.Bytes())
		if err == nil {
			lastLogFlush = time.Now()
			lastFlushSize = logBuffer.Len()
		}
		return err
	}
	
	appendLog := func(message string) {
		logMutex.Lock()
		logBuffer.WriteString(message)
		now := time.Now()
		// Flush if we have >1KB of NEW data or if 1s has passed
		shouldFlush := (logBuffer.Len()-lastFlushSize) > 1024 || now.Sub(lastLogFlush) > 1*time.Second
		logMutex.Unlock()
		
		if shouldFlush {
			_ = flushLogs()
		}
	}
	
	defer func() {
		_ = flushLogs()
	}()

	// Update run status to "applying"
	if err := e.runRepo.UpdateRunStatus(ctx, runID, "applying"); err != nil {
		logger.Error("failed to update run status", slog.String("error", err.Error()))
		return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to update run status: %v", err))
	}

	logger.Info("updated run status to applying")

	appendLog("Starting terraform apply...\n")
	appendLog("Downloading configuration...\n")

	// Get configuration version
	configVer, err := e.configVerRepo.GetConfigurationVersion(ctx, run.ConfigurationVersionID)
	if err != nil {
		return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to get configuration version: %v", err))
	}

	// Download configuration archive
	archivePath := fmt.Sprintf("config-versions/%s/archive.tar.gz", configVer.ID)
	archiveData, err := e.blobStore.DownloadBlob(ctx, archivePath)
	if err != nil {
		return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to download archive: %v", err))
	}

	appendLog("Extracting workspace...\n")

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

	var workspaceArchive []byte
	if useSandbox {
		workspaceArchive, err = createWorkspaceArchive(workDir)
		if err != nil {
			return e.handleApplyError(ctx, run.ID, logger, fmt.Sprintf("Failed to package workspace for sandbox apply: %v", err))
		}
		logger.Info("packaged workspace for sandbox apply", slog.Int("bytes", len(workspaceArchive)))
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
	} else if useSandbox {
		logger.Info("downloaded existing state for sandbox apply",
			slog.String("state_id", stateID),
			slog.Int("bytes", len(stateData)))
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

	// Run terraform apply (locally or via sandbox)
	var (
		logs         string
		applyErr     error
		updatedState []byte
	)

	if useSandbox {
		appendLog("Starting remote execution environment...\n")
		appendLog("Initializing terraform...\n")
		appendLog("Running terraform apply...\n")

		if applyActivityID != "" && e.activityRepo != nil {
			applyActivityStart = time.Now()
			if err := e.activityRepo.MarkRunning(ctx, applyActivityID, applyActivityStart, e.sandbox.Name()); err != nil {
				logger.Warn("failed to mark remote apply running", slog.String("error", err.Error()))
			}
		}

		// Start heartbeat goroutine for long-running remote applies
		heartbeatDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					appendLog(fmt.Sprintf("Remote apply in progress... (%s)\n", time.Now().Format("15:04:05")))
				case <-heartbeatDone:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
		defer close(heartbeatDone)

		result, execErr := e.executeApplyInSandbox(ctx, run, unitMeta, workspaceArchive, stateData, appendLog)
		applySandboxResult = result
		applyErr = execErr
		if result != nil {
			logs = result.Logs
			updatedState = result.State
		}
		if logs == "" {
			logs = "remote sandbox did not return apply logs"
		}
	} else {
		localLogs, execErr := e.runTerraformApply(ctx, workDir, run.IsDestroy)
		logs = localLogs
		applyErr = execErr
		if execErr == nil {
			statePath := filepath.Join(workDir, "terraform.tfstate")
			if data, readErr := os.ReadFile(statePath); readErr != nil {
				logger.Warn("failed to read updated state file", slog.String("error", readErr.Error()))
			} else {
				updatedState = data
			}
		}
	}

	// Append the actual terraform output to the progress logs
	appendLog("\n" + logs)

	// Store final status
	if applyErr != nil {
		appendLog("\n\nApply failed\n")
	} else {
		appendLog("\n\nApply complete\n")
	}

	// Update run status
	runStatus := "applied"
	if applyErr != nil {
		runStatus = "errored"
		logs = logs + "\n\nError: " + applyErr.Error()
		_ = e.blobStore.UploadBlob(ctx, applyLogBlobID, []byte(logs))
		if updateErr := e.runRepo.UpdateRunError(ctx, run.ID, applyErr.Error()); updateErr != nil {
			logger.Error("failed to update run error", slog.String("error", updateErr.Error()))
		}
	} else {
		stateID := fmt.Sprintf("%s/%s", run.OrgID, run.UnitID)
		if len(updatedState) == 0 {
			logger.Warn("no updated state returned after apply; state upload skipped",
				slog.String("state_id", stateID))
		} else {
			if uploadErr := e.blobStore.Upload(ctx, stateID, updatedState, lockInfo.ID); uploadErr != nil {
				logger.Error("failed to upload updated state",
					slog.String("state_id", stateID),
					slog.String("error", uploadErr.Error()))
				runStatus = "errored"
				errMsg := fmt.Sprintf("Failed to upload state: %v", uploadErr)
				logs = logs + "\n\nCritical Error: " + errMsg + "\n"
				_ = e.blobStore.UploadBlob(ctx, applyLogBlobID, []byte(logs))
				if updateErr := e.runRepo.UpdateRunError(ctx, run.ID, errMsg); updateErr != nil {
					logger.Error("failed to update run error", slog.String("error", updateErr.Error()))
				}
				applyErr = fmt.Errorf(errMsg)
			} else {
				logger.Info("successfully uploaded updated state",
					slog.String("state_id", stateID),
					slog.Int("bytes", len(updatedState)))
			}
		}
	}

	if err := e.runRepo.UpdateRunStatus(ctx, run.ID, runStatus); err != nil {
		logger.Error("failed to update run status", slog.String("error", err.Error()))
		return fmt.Errorf("failed to update run status: %w", err)
	}

	logger.Info("apply execution completed", slog.String("status", runStatus))

	if applyErr != nil {
		return fmt.Errorf("apply failed: %w", applyErr)
	}

	if useSandbox && applyActivityID != "" && e.activityRepo != nil && !applyActivityStart.IsZero() {
		completedAt := time.Now()
		status := "succeeded"
		var errMsg *string
		if applyErr != nil {
			status = "failed"
			msg := applyErr.Error()
			errMsg = &msg
		}
		var sandboxJobID *string
		if applySandboxResult != nil && applySandboxResult.RuntimeRunID != "" {
			id := applySandboxResult.RuntimeRunID
			sandboxJobID = &id
		}
		if err := e.activityRepo.MarkCompleted(ctx, applyActivityID, status, completedAt, completedAt.Sub(applyActivityStart), sandboxJobID, errMsg); err != nil {
			logger.Warn("failed to mark remote apply completion", slog.String("error", err.Error()))
		}
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

func (e *ApplyExecutor) executeApplyInSandbox(ctx context.Context, run *domain.TFERun, unit *storage.UnitMetadata, archive []byte, stateData []byte, logSink func(string)) (*sandbox.ApplyResult, error) {
	if e.sandbox == nil {
		return nil, fmt.Errorf("sandbox provider not configured")
	}
	if len(archive) == 0 {
		return nil, fmt.Errorf("sandbox apply requires configuration archive")
	}

	metadata := map[string]string{
		"auto_apply": strconv.FormatBool(run.AutoApply),
	}

	planID := ""
	if run.PlanID != nil {
		planID = *run.PlanID
		metadata["plan_id"] = planID
	}

	req := &sandbox.ApplyRequest{
		RunID:                  run.ID,
		PlanID:                 planID,
		OrgID:                  run.OrgID,
		UnitID:                 run.UnitID,
		ConfigurationVersionID: run.ConfigurationVersionID,
		IsDestroy:              run.IsDestroy,
		TerraformVersion:       terraformVersionForUnit(unit),
		Engine:                 engineForUnit(unit),
		WorkingDirectory:       workingDirectoryForUnit(unit),
		ConfigArchive:          archive,
		State:                  stateData,
		Metadata:               metadata,
		LogSink:                logSink,
	}
	return e.sandbox.ExecuteApply(ctx, req)
}
