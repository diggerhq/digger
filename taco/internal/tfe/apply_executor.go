package tfe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// ApplyExecutor handles real Terraform apply execution
type ApplyExecutor struct {
	runRepo       domain.TFERunRepository
	planRepo      domain.TFEPlanRepository
	configVerRepo domain.TFEConfigurationVersionRepository
	blobStore     storage.UnitStore
}

// NewApplyExecutor creates a new apply executor
func NewApplyExecutor(
	runRepo domain.TFERunRepository,
	planRepo domain.TFEPlanRepository,
	configVerRepo domain.TFEConfigurationVersionRepository,
	blobStore storage.UnitStore,
) *ApplyExecutor {
	return &ApplyExecutor{
		runRepo:       runRepo,
		planRepo:      planRepo,
		configVerRepo: configVerRepo,
		blobStore:     blobStore,
	}
}

// ExecuteApply executes a Terraform apply for a run
func (e *ApplyExecutor) ExecuteApply(ctx context.Context, runID string) error {
	fmt.Printf("Starting apply execution for run %s\n", runID)

	// Get run
	run, err := e.runRepo.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to get run: %w", err)
	}

	// Check if run can be applied
	if run.Status != "planned_and_finished" && run.Status != "apply_queued" {
		return fmt.Errorf("run cannot be applied in status: %s", run.Status)
	}

	// Update run status to "applying"
	if err := e.runRepo.UpdateRunStatus(ctx, runID, "applying"); err != nil {
		return fmt.Errorf("failed to update run status: %w", err)
	}

	// Get configuration version
	configVer, err := e.configVerRepo.GetConfigurationVersion(ctx, run.ConfigurationVersionID)
	if err != nil {
		return fmt.Errorf("failed to get configuration version: %w", err)
	}

	// Download configuration archive
	archivePath := fmt.Sprintf("config-versions/%s/archive.tar.gz", configVer.ID)
	archiveData, err := e.blobStore.Download(ctx, archivePath)
	if err != nil {
		return e.handleApplyError(ctx, run.ID, fmt.Sprintf("Failed to download archive: %v", err))
	}

	// Extract to temp directory
	workDir, err := extractArchive(archiveData)
	if err != nil {
		return e.handleApplyError(ctx, run.ID, fmt.Sprintf("Failed to extract archive: %v", err))
	}
	defer cleanupWorkDir(workDir)

	fmt.Printf("Extracted archive to %s for apply\n", workDir)

	// Remove cloud/backend configuration to prevent circular dependencies
	if err := createBackendOverride(workDir); err != nil {
		return e.handleApplyError(ctx, run.ID, fmt.Sprintf("Failed to remove backend configuration: %v", err))
	}

	// Download current state for this unit (must exist before apply)
	// Construct org-scoped state ID: <orgID>/<unitID>
	stateID := fmt.Sprintf("%s/%s", run.OrgID, run.UnitID)
	stateData, err := e.blobStore.Download(ctx, stateID)
	if err != nil {
		fmt.Printf("Warning: Failed to download state for %s: %v\n", stateID, err)
		// Continue anyway - might be a fresh deployment
	} else {
		// Write state to terraform.tfstate in the working directory
		statePath := filepath.Join(workDir, "terraform.tfstate")
		if err := os.WriteFile(statePath, stateData, 0644); err != nil {
			return e.handleApplyError(ctx, run.ID, fmt.Sprintf("Failed to write state file: %v", err))
		}
		fmt.Printf("Downloaded and wrote existing state for %s (%d bytes)\n", stateID, len(stateData))
	}

	// Run terraform apply
	logs, err := e.runTerraformApply(ctx, workDir, run.IsDestroy)

	// Store apply logs in blob storage (use UploadBlob - no lock checks needed for logs)
	applyLogBlobID := fmt.Sprintf("runs/%s/apply-logs.txt", run.ID)
	if storeErr := e.blobStore.UploadBlob(ctx, applyLogBlobID, []byte(logs)); storeErr != nil {
		fmt.Printf("Failed to store apply logs: %v\n", storeErr)
	}

	// Update run status
	runStatus := "applied"
	if err != nil {
		runStatus = "errored"
		logs = logs + "\n\nError: " + err.Error()
		// Store error logs even on failure
		_ = e.blobStore.UploadBlob(ctx, applyLogBlobID, []byte(logs))
	} else {
		// Upload the updated state back to storage after successful apply
		// Construct org-scoped state ID: <orgID>/<unitID>
		stateID := fmt.Sprintf("%s/%s", run.OrgID, run.UnitID)
		statePath := filepath.Join(workDir, "terraform.tfstate")
		newStateData, readErr := os.ReadFile(statePath)
		if readErr != nil {
			fmt.Printf("Warning: Failed to read updated state file: %v\n", readErr)
		} else {
			// Upload state with empty lock ID (state is already locked during apply)
			if uploadErr := e.blobStore.Upload(ctx, stateID, newStateData, ""); uploadErr != nil {
				fmt.Printf("ERROR: Failed to upload updated state for %s: %v\n", stateID, uploadErr)
				// This is critical - mark as errored
				runStatus = "errored"
				logs = logs + fmt.Sprintf("\n\nCritical Error: Failed to upload state: %v\n", uploadErr)
			} else {
				fmt.Printf("Successfully uploaded updated state for %s (%d bytes)\n", stateID, len(newStateData))
			}
		}
	}

	if err := e.runRepo.UpdateRunStatus(ctx, run.ID, runStatus); err != nil {
		return fmt.Errorf("failed to update run status: %w", err)
	}

	fmt.Printf("Apply execution completed for run %s: status=%s\n", runID, runStatus)

	if err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	return nil
}

// runTerraformApply executes terraform init and apply
func (e *ApplyExecutor) runTerraformApply(ctx context.Context, workDir string, isDestroy bool) (logs string, err error) {
	var allLogs strings.Builder

	// Run terraform init (cloud/backend config already removed by createBackendOverride)
	fmt.Printf("Running terraform init in %s\n", workDir)
	initCmd := exec.CommandContext(ctx, "terraform", "init", "-no-color", "-input=false")
	initCmd.Dir = workDir
	initCmd.Env = append(os.Environ(), "TF_IN_AUTOMATION=1")
	initOutput, initErr := initCmd.CombinedOutput()
	allLogs.WriteString("=== Terraform Init ===\n")
	allLogs.Write(initOutput)
	allLogs.WriteString("\n\n")

	if initErr != nil {
		return allLogs.String(), fmt.Errorf("terraform init failed: %w", initErr)
	}

	// Run terraform apply
	fmt.Printf("Running terraform apply in %s\n", workDir)
	applyArgs := []string{"apply", "-no-color", "-input=false", "-auto-approve"}
	if isDestroy {
		applyArgs = []string{"destroy", "-no-color", "-input=false", "-auto-approve"}
	}

	applyCmd := exec.CommandContext(ctx, "terraform", applyArgs...)
	applyCmd.Dir = workDir
	applyCmd.Env = append(os.Environ(), "TF_IN_AUTOMATION=1")
	applyOutput, applyErr := applyCmd.CombinedOutput()
	allLogs.WriteString("=== Terraform Apply ===\n")
	allLogs.Write(applyOutput)
	allLogs.WriteString("\n")

	if applyErr != nil {
		return allLogs.String(), fmt.Errorf("terraform apply failed: %w", applyErr)
	}

	return allLogs.String(), nil
}

// handleApplyError handles apply execution errors
func (e *ApplyExecutor) handleApplyError(ctx context.Context, runID string, errorMsg string) error {
	fmt.Printf("Apply error for run %s: %s\n", runID, errorMsg)

	// Update run status
	_ = e.runRepo.UpdateRunStatus(ctx, runID, "errored")

	return fmt.Errorf("apply execution failed: %s", errorMsg)
}

