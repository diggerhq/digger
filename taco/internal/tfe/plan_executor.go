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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/sandbox"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/hashicorp/terraform-exec/tfexec"
)

// PlanExecutor handles real Terraform plan execution
type PlanExecutor struct {
	runRepo       domain.TFERunRepository
	planRepo      domain.TFEPlanRepository
	configVerRepo domain.TFEConfigurationVersionRepository
	blobStore     storage.UnitStore
	unitRepo      domain.UnitRepository
	sandbox       sandbox.Sandbox
	activityRepo  domain.RemoteRunActivityRepository
}

// NewPlanExecutor creates a new plan executor
func NewPlanExecutor(
	runRepo domain.TFERunRepository,
	planRepo domain.TFEPlanRepository,
	configVerRepo domain.TFEConfigurationVersionRepository,
	blobStore storage.UnitStore,
	unitRepo domain.UnitRepository,
	sandboxProvider sandbox.Sandbox,
	activityRepo domain.RemoteRunActivityRepository,
) *PlanExecutor {
	return &PlanExecutor{
		runRepo:       runRepo,
		planRepo:      planRepo,
		configVerRepo: configVerRepo,
		blobStore:     blobStore,
		unitRepo:      unitRepo,
		sandbox:       sandboxProvider,
		activityRepo:  activityRepo,
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

	unitMeta, err := e.unitRepo.Get(ctx, run.UnitID)
	if err != nil {
		logger.Error("failed to load unit metadata", slog.String("error", err.Error()))
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, fmt.Sprintf("Failed to load workspace metadata: %v", err))
	}

	logger.Info("🔍 PLAN EXECUTOR: Checking execution path",
		slog.String("unit_id", run.UnitID),
		slog.String("unit_name", unitMeta.Name),
		slog.String("execution_mode", func() string {
			if unitMeta.TFEExecutionMode != nil {
				return *unitMeta.TFEExecutionMode
			}
			return "not set"
		}()),
		slog.Bool("sandbox_available", e.sandbox != nil),
		slog.String("sandbox_provider", func() string {
			if e.sandbox != nil {
				return e.sandbox.Name()
			}
			return "none"
		}()))

	useSandbox := requiresSandbox(unitMeta)
	var planActivityID string
	var planActivityStart time.Time
	var planSandboxResult *sandbox.PlanResult

	if useSandbox {
		logger.Info("✅ PLAN EXECUTOR: Remote execution path selected",
			slog.String("unit_id", run.UnitID),
			slog.Bool("activity_repo_available", e.activityRepo != nil))
	} else {
		logger.Info("ℹ️  PLAN EXECUTOR: Local execution path selected",
			slog.String("unit_id", run.UnitID))
	}

	if useSandbox && e.activityRepo != nil {
		activity := &domain.RemoteRunActivity{
			RunID:           run.ID,
			OrgID:           run.OrgID,
			UnitID:          run.UnitID,
			Operation:       "plan",
			Status:          "pending",
			TriggeredBy:     run.CreatedBy,
			TriggeredSource: run.Source,
		}

		if id, err := e.activityRepo.CreateActivity(ctx, activity); err != nil {
			logger.Warn("⚠️  failed to create remote run activity record", slog.String("error", err.Error()))
		} else {
			planActivityID = id
			logger.Info("📝 Created remote run activity record", slog.String("activity_id", planActivityID))
		}
	}

	if useSandbox && e.sandbox == nil {
		logger.Error("❌ FATAL: Workspace requires remote execution but no sandbox provider configured")
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, "Workspace execution mode is remote, but no sandbox provider is configured")
	}

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

	// Chunked logging to prevent memory bloat
	// Upload log chunks as separate S3 objects and clear buffer after each upload
	// Fixed-size 2KB chunks enable offset-based chunk selection (reduces S3 re-downloads)
	const chunkSize = 2 * 1024 // 2KB fixed size
	chunkIndex := 1
	var logBuffer bytes.Buffer
	var logMutex sync.Mutex
	lastLogFlush := time.Now()
	
	// Flush helper - uploads current buffer as a padded 2KB chunk and clears it
	flushLogs := func() error {
		logMutex.Lock()
		if logBuffer.Len() == 0 {
			logMutex.Unlock()
			return nil
		}
		
		// Extract at most chunkSize bytes (2KB)
		dataLen := logBuffer.Len()
		if dataLen > chunkSize {
			dataLen = chunkSize
		}
		data := make([]byte, dataLen)
		copy(data, logBuffer.Bytes()[:dataLen])
		currentChunk := chunkIndex
		chunkIndex++ // Increment NOW before unlock to reserve this chunk number atomically
		
		// Copy remainder BEFORE resetting (crucial - remainder slice points to internal buffer)
		var remainderCopy []byte
		if logBuffer.Len() > dataLen {
			remainder := logBuffer.Bytes()[dataLen:]
			remainderCopy = make([]byte, len(remainder))
			copy(remainderCopy, remainder)
		}
		
		// Now safe to reset and write remainder back
		logBuffer.Reset()
		if len(remainderCopy) > 0 {
			logBuffer.Write(remainderCopy)
		}
		logMutex.Unlock()

		// Pad to fixed 2KB size (rest will be null bytes)
		paddedData := make([]byte, chunkSize)
		copy(paddedData, data)

		// Upload this chunk (key includes zero-padded chunk index)
		chunkKey := fmt.Sprintf("plans/%s/chunks/%08d.log", *run.PlanID, currentChunk)
		err := e.blobStore.UploadBlob(ctx, chunkKey, paddedData)
		
		if err == nil {
			logMutex.Lock()
			lastLogFlush = time.Now()
			logMutex.Unlock()
		}
		return err
	}
	
	
	// Buffered append - only uploads when buffer is large or time has elapsed
	appendLog := func(message string) {
		logMutex.Lock()
		logBuffer.WriteString(message)
		now := time.Now()
		// Flush if buffer exceeds 2KB or 1s has passed
		shouldFlush := logBuffer.Len() > chunkSize || now.Sub(lastLogFlush) > 1*time.Second
		logMutex.Unlock()
		
		if shouldFlush {
			_ = flushLogs() // Ignore errors for progress updates
		}
	}
	
	// Ensure logs are flushed at the end
	defer func() {
		_ = flushLogs()
	}()

	// Update run status to "planning"
	if err := e.runRepo.UpdateRunStatus(ctx, runID, "planning"); err != nil {
		logger.Error("failed to update status to planning", slog.String("error", err.Error()))
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, fmt.Sprintf("Failed to update run status: %v", err))
	}
	logger.Info("updated run status to planning")

	// Note: We no longer set LogBlobID since we use chunked logging
	// The API reads chunks directly from plans/{planID}/chunks/*.log

	appendLog("Preparing terraform run...\n")

	// Get configuration version
	configVer, err := e.configVerRepo.GetConfigurationVersion(ctx, run.ConfigurationVersionID)
	if err != nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, fmt.Sprintf("Failed to get configuration version: %v", err))
	}

	// Check if configuration was uploaded
	if configVer.Status != "uploaded" || configVer.ArchiveBlobID == nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, "Configuration not uploaded")
	}

	appendLog("Downloading configuration...\n")

	// Download configuration archive from blob storage
	archivePath := fmt.Sprintf("config-versions/%s/archive.tar.gz", configVer.ID)
	archiveData, err := e.blobStore.DownloadBlob(ctx, archivePath)
	if err != nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, logger, fmt.Sprintf("Failed to download archive: %v", err))
	}

	logger.Info("downloaded configuration archive",
		slog.Int("bytes", len(archiveData)),
		slog.String("config_version_id", configVer.ID))

	appendLog("Extracting workspace...\n")

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

	var workspaceArchive []byte
	if useSandbox {
		appendLog("Packaging workspace for remote execution...\n")
		workspaceArchive, err = createWorkspaceArchive(workDir)
		if err != nil {
			return e.handlePlanError(ctx, run.ID, run.PlanID, logger, fmt.Sprintf("Failed to package workspace for sandbox execution: %v", err))
		}
		logger.Info("packaged workspace for sandbox execution", slog.Int("bytes", len(workspaceArchive)))
	}

	// Download current state for this unit (if it exists)
	// Construct org-scoped state ID: <orgID>/<unitID>
	stateID := fmt.Sprintf("%s/%s", run.OrgID, run.UnitID)
	stateData, err := e.blobStore.Download(ctx, stateID)
	if err == nil {
		if useSandbox {
			logger.Info("downloaded existing state for sandbox execution",
				slog.String("state_id", stateID),
				slog.Int("bytes", len(stateData)))
		} else {
			// Write state to terraform.tfstate in the working directory
			statePath := filepath.Join(workDir, "terraform.tfstate")
			if err := os.WriteFile(statePath, stateData, 0644); err != nil {
				logger.Warn("failed to write state file", slog.String("error", err.Error()))
			} else {
				logger.Info("downloaded and wrote existing state",
					slog.String("state_id", stateID),
					slog.Int("bytes", len(stateData)))
			}
		}
	} else {
		logger.Info("no existing state found, starting fresh",
			slog.String("state_id", stateID))
	}

	var (
		logs       string
		hasChanges bool
		adds       int
		changes    int
		destroys   int
		planJSON   []byte
		planErr    error
	)

	if useSandbox {
		appendLog("Starting remote execution environment...\n")
		appendLog("Initializing terraform...\n")

		logger.Info("🚀 EXECUTING PLAN IN SANDBOX",
			slog.String("run_id", run.ID),
			slog.String("unit_id", run.UnitID),
			slog.String("sandbox_provider", e.sandbox.Name()),
			slog.Int("workspace_archive_bytes", len(workspaceArchive)),
			slog.Int("state_bytes", len(stateData)))

		if planActivityID != "" && e.activityRepo != nil {
			planActivityStart = time.Now()
			if err := e.activityRepo.MarkRunning(ctx, planActivityID, planActivityStart, e.sandbox.Name()); err != nil {
				logger.Warn("⚠️  failed to mark remote plan running", slog.String("error", err.Error()))
			} else {
				logger.Info("📝 Marked activity as running", slog.String("activity_id", planActivityID))
			}
		}

		// Start heartbeat goroutine for long-running remote executions
		// This provides user feedback and prevents the UI from appearing frozen
		heartbeatDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					appendLog(fmt.Sprintf("Remote plan in progress... (%s)\n", time.Now().Format("15:04:05")))
				case <-heartbeatDone:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
		defer close(heartbeatDone)

		result, execErr := e.executePlanInSandbox(ctx, run, unitMeta, workspaceArchive, stateData, appendLog)
		planSandboxResult = result
		planErr = execErr

		if execErr != nil {
			logger.Error("❌ SANDBOX PLAN FAILED",
				slog.String("run_id", run.ID),
				slog.String("error", execErr.Error()))
		} else {
			logger.Info("✅ SANDBOX PLAN SUCCEEDED",
				slog.String("run_id", run.ID),
				slog.Bool("has_changes", result != nil && result.HasChanges))
		}

		if result != nil {
			logs = result.Logs
			hasChanges = result.HasChanges
			adds = result.ResourceAdditions
			changes = result.ResourceChanges
			destroys = result.ResourceDestructions
			planJSON = result.PlanJSON
		}
		if logs == "" {
			logs = "remote sandbox did not return plan logs"
		}
	} else {
		logger.Info("🏠 EXECUTING PLAN LOCALLY",
			slog.String("run_id", run.ID),
			slog.String("unit_id", run.UnitID),
			slog.String("work_dir", workDir))

		_, planLogs, planHasChanges, planAdds, planChanges, planDestroys, execErr := e.runTerraformPlan(ctx, workDir, run.IsDestroy)
		logs = planLogs
		hasChanges = planHasChanges
		adds = planAdds
		changes = planChanges
		destroys = planDestroys
		planErr = execErr

		if execErr != nil {
			logger.Error("❌ LOCAL PLAN FAILED",
				slog.String("run_id", run.ID),
				slog.String("error", execErr.Error()))
		} else {
			logger.Info("✅ LOCAL PLAN SUCCEEDED",
				slog.String("run_id", run.ID),
				slog.Bool("has_changes", planHasChanges))
		}
	}

	if useSandbox && planActivityID != "" && e.activityRepo != nil && !planActivityStart.IsZero() {
		completedAt := time.Now()
		status := "succeeded"
		var errMsg *string
		if planErr != nil {
			status = "failed"
			msg := planErr.Error()
			errMsg = &msg
		}
		var sandboxJobID *string
		if planSandboxResult != nil && planSandboxResult.RuntimeRunID != "" {
			id := planSandboxResult.RuntimeRunID
			sandboxJobID = &id
		}
		if err := e.activityRepo.MarkCompleted(ctx, planActivityID, status, completedAt, completedAt.Sub(planActivityStart), sandboxJobID, errMsg); err != nil {
			logger.Warn("failed to mark remote plan completion", slog.String("error", err.Error()))
		}
	}

	// Append the actual terraform output to the progress logs
	if !useSandbox {
		appendLog("\n" + logs)
	}

	// Store final status
	if planErr != nil {
		appendLog("\n\nPlan failed\n")
	} else {
		appendLog("\n\nPlan complete\n")
	}

	// Generate signed log URL
	logReadURL := fmt.Sprintf("/tfe/api/v2/plans/%s/logs/logs", *run.PlanID)

	// Update plan with results
	planStatus := "finished"
	if planErr != nil {
		planStatus = "errored"
		appendLog("\nError: " + planErr.Error() + "\n")
		// Store error in run for user visibility
		if updateErr := e.runRepo.UpdateRunError(ctx, run.ID, planErr.Error()); updateErr != nil {
			logger.Error("failed to update run error", slog.String("error", updateErr.Error()))
		}
	}

	planUpdates := &domain.TFEPlanUpdate{
		Status:               &planStatus,
		ResourceAdditions:    &adds,
		ResourceChanges:      &changes,
		ResourceDestructions: &destroys,
		HasChanges:           &hasChanges,
		// LogBlobID removed - we use chunked logging now
		LogReadURL:           &logReadURL,
	}
	if len(planJSON) > 0 {
		jsonStr := string(planJSON)
		planUpdates.PlanOutputJSON = &jsonStr
	}

	if err := e.planRepo.UpdatePlan(ctx, *run.PlanID, planUpdates); err != nil {
		return fmt.Errorf("failed to update plan: %w", err)
	}

	// Update run status and can_apply
	// Use "planned" status (not "planned_and_finished") - this is what Terraform CLI expects
	runStatus := "planned"
	canApply := (planErr == nil) // Can apply if plan succeeded (regardless of whether there are changes)

	if planErr != nil {
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
		slog.Bool("plan_succeeded", planErr == nil))

	if run.AutoApply && planErr == nil {
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
			applyExecutor := NewApplyExecutor(e.runRepo, e.planRepo, e.configVerRepo, e.blobStore, e.unitRepo, e.sandbox, e.activityRepo)
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

func (e *PlanExecutor) executePlanInSandbox(ctx context.Context, run *domain.TFERun, unit *storage.UnitMetadata, archive []byte, stateData []byte, logSink func(string)) (*sandbox.PlanResult, error) {
	if e.sandbox == nil {
		return nil, fmt.Errorf("sandbox provider not configured")
	}
	if len(archive) == 0 {
		return nil, fmt.Errorf("sandbox plan requires configuration archive")
	}

	planID := ""
	if run.PlanID != nil {
		planID = *run.PlanID
	}

	metadata := map[string]string{
		"auto_apply": strconv.FormatBool(run.AutoApply),
	}
	if planID != "" {
		metadata["plan_id"] = planID
	}

	req := &sandbox.PlanRequest{
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
	return e.sandbox.ExecutePlan(ctx, req)
}

func createWorkspaceArchive(workDir string) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	err := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .terraform directories to avoid uploading cache/modules
		if info.IsDir() && strings.HasPrefix(info.Name(), ".terraform") {
			return filepath.SkipDir
		}

		relPath, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, file); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}
		return nil
	})
	if err != nil {
		tw.Close()
		gz.Close()
		return nil, err
	}

	if err := tw.Close(); err != nil {
		gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
