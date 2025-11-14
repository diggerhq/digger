package tfe

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// PlanExecutor handles real Terraform plan execution
type PlanExecutor struct {
	runRepo       domain.TFERunRepository
	planRepo      domain.TFEPlanRepository
	configVerRepo domain.TFEConfigurationVersionRepository
	blobStore     storage.UnitStore
}

// NewPlanExecutor creates a new plan executor
func NewPlanExecutor(
	runRepo domain.TFERunRepository,
	planRepo domain.TFEPlanRepository,
	configVerRepo domain.TFEConfigurationVersionRepository,
	blobStore storage.UnitStore,
) *PlanExecutor {
	return &PlanExecutor{
		runRepo:       runRepo,
		planRepo:      planRepo,
		configVerRepo: configVerRepo,
		blobStore:     blobStore,
	}
}

// ExecutePlan executes a Terraform plan for a run
func (e *PlanExecutor) ExecutePlan(ctx context.Context, runID string) error {
	fmt.Printf("[ExecutePlan] === STARTING FOR RUN %s ===\n", runID)

	// Get run
	run, err := e.runRepo.GetRun(ctx, runID)
	if err != nil {
		fmt.Printf("[ExecutePlan] ERROR: Failed to get run: %v\n", err)
		return fmt.Errorf("failed to get run: %w", err)
	}
	fmt.Printf("[ExecutePlan] Got run, configVersionID=%s\n", run.ConfigurationVersionID)

	// Update run status to "planning"
	if err := e.runRepo.UpdateRunStatus(ctx, runID, "planning"); err != nil {
		fmt.Printf("[ExecutePlan] ERROR: Failed to update status to planning: %v\n", err)
		return fmt.Errorf("failed to update run status: %w", err)
	}
	fmt.Printf("[ExecutePlan] Updated run status to 'planning'\n")

	// Get configuration version
	configVer, err := e.configVerRepo.GetConfigurationVersion(ctx, run.ConfigurationVersionID)
	if err != nil {
		return fmt.Errorf("failed to get configuration version: %w", err)
	}

	// Check if configuration was uploaded
	if configVer.Status != "uploaded" || configVer.ArchiveBlobID == nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, "Configuration not uploaded")
	}

	// Download configuration archive from blob storage
	archivePath := fmt.Sprintf("config-versions/%s/archive.tar.gz", configVer.ID)
	archiveData, err := e.blobStore.Download(ctx, archivePath)
	if err != nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, fmt.Sprintf("Failed to download archive: %v", err))
	}

	fmt.Printf("Downloaded %d bytes for configuration version %s\n", len(archiveData), configVer.ID)

	// Extract to temp directory
	workDir, err := extractArchive(archiveData)
	if err != nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, fmt.Sprintf("Failed to extract archive: %v", err))
	}
	defer cleanupWorkDir(workDir)

	fmt.Printf("Extracted archive to %s\n", workDir)

	// Create an override file to disable cloud/remote backend
	// This is required for server-side execution to prevent circular dependencies
	if err := createBackendOverride(workDir); err != nil {
		return e.handlePlanError(ctx, run.ID, run.PlanID, fmt.Sprintf("Failed to create backend override: %v", err))
	}

	// Download current state for this unit (if it exists)
	// Construct org-scoped state ID: <orgID>/<unitID>
	stateID := fmt.Sprintf("%s/%s", run.OrgID, run.UnitID)
	stateData, err := e.blobStore.Download(ctx, stateID)
	if err == nil {
		// Write state to terraform.tfstate in the working directory
		statePath := filepath.Join(workDir, "terraform.tfstate")
		if err := os.WriteFile(statePath, stateData, 0644); err != nil {
			fmt.Printf("Warning: Failed to write state file: %v\n", err)
		} else {
			fmt.Printf("Downloaded and wrote existing state for %s (%d bytes)\n", stateID, len(stateData))
		}
	} else {
		fmt.Printf("No existing state found for %s (will start fresh): %v\n", stateID, err)
	}

	// Run terraform plan
	planOutput, logs, hasChanges, adds, changes, destroys, err := e.runTerraformPlan(ctx, workDir, run.IsDestroy)

	// Store logs in blob storage (use UploadBlob - no lock checks needed for logs)
	logBlobID := fmt.Sprintf("plans/%s/logs.txt", *run.PlanID)
	if err := e.blobStore.UploadBlob(ctx, logBlobID, []byte(logs)); err != nil {
		fmt.Printf("Failed to store logs: %v\n", err)
	}

	// Generate signed log URL
	logReadURL := fmt.Sprintf("/tfe/api/v2/plans/%s/logs/logs", *run.PlanID)

	// Update plan with results
	planStatus := "finished"
	if err != nil {
		planStatus = "errored"
		logs = logs + "\n\nError: " + err.Error()
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

	// Store plan output if not too large
	if len(planOutput) < 1024*1024 { // < 1MB
		planUpdates.PlanOutputJSON = &planOutput
	}

	if err := e.planRepo.UpdatePlan(ctx, *run.PlanID, planUpdates); err != nil {
		return fmt.Errorf("failed to update plan: %w", err)
	}

	// Update run status and can_apply
	runStatus := "planned_and_finished"
	canApply := (err == nil) // Can apply if plan succeeded (regardless of whether there are changes)
	
	if err != nil {
		runStatus = "errored"
	}

	fmt.Printf("[ExecutePlan] Updating run %s status to '%s', canApply=%v\n", run.ID, runStatus, canApply)
	if err := e.runRepo.UpdateRunStatusAndCanApply(ctx, run.ID, runStatus, canApply); err != nil {
		fmt.Printf("[ExecutePlan] ERROR: Failed to update run: %v\n", err)
		return fmt.Errorf("failed to update run: %w", err)
	}

	fmt.Printf("[ExecutePlan] ✅ Plan execution completed for run %s: status=%s, canApply=%v, hasChanges=%v, adds=%d, changes=%d, destroys=%d\n",
		runID, runStatus, canApply, hasChanges, adds, changes, destroys)

	// Only auto-trigger apply if AutoApply flag is true (i.e., terraform apply -auto-approve)
	fmt.Printf("[ExecutePlan] Auto-apply check: run.AutoApply=%v, err=%v\n", run.AutoApply, err)
	if run.AutoApply && err == nil {
		fmt.Printf("[ExecutePlan] Auto-applying run %s (AutoApply=true)\n", runID)
		
		// Queue the apply by updating the run status
		if err := e.runRepo.UpdateRunStatus(ctx, run.ID, "apply_queued"); err != nil {
			fmt.Printf("[ExecutePlan] ERROR: Failed to queue apply: %v\n", err)
			return nil // Don't fail the plan if we can't queue the apply
		}
		
		// Trigger apply execution in background
		go func() {
			fmt.Printf("[ExecutePlan] Starting async apply execution for run %s\n", run.ID)
			applyExecutor := NewApplyExecutor(e.runRepo, e.planRepo, e.configVerRepo, e.blobStore)
			if err := applyExecutor.ExecuteApply(context.Background(), run.ID); err != nil {
				fmt.Printf("[ExecutePlan] ❌ Apply execution failed for run %s: %v\n", run.ID, err)
			} else {
				fmt.Printf("[ExecutePlan] ✅ Apply execution completed successfully for run %s\n", run.ID)
			}
		}()
	}

	return nil
}

// runTerraformPlan executes terraform init and plan
func (e *PlanExecutor) runTerraformPlan(ctx context.Context, workDir string, isDestroy bool) (output string, logs string, hasChanges bool, adds, changes, destroys int, err error) {
	var allLogs strings.Builder

	// Run terraform init (backend override file disables cloud/remote backend)
	fmt.Printf("Running terraform init in %s\n", workDir)
	initCmd := exec.CommandContext(ctx, "terraform", "init", "-no-color", "-input=false")
	initCmd.Dir = workDir
	initCmd.Env = append(os.Environ(), "TF_IN_AUTOMATION=1") // Tell Terraform it's running in automation
	initOutput, initErr := initCmd.CombinedOutput()
	allLogs.WriteString("=== Terraform Init ===\n")
	allLogs.Write(initOutput)
	allLogs.WriteString("\n\n")

	if initErr != nil {
		return "", allLogs.String(), false, 0, 0, 0, fmt.Errorf("terraform init failed: %w", initErr)
	}

	// Run terraform plan
	fmt.Printf("Running terraform plan in %s\n", workDir)
	planArgs := []string{"plan", "-no-color", "-input=false", "-detailed-exitcode"}
	if isDestroy {
		planArgs = append(planArgs, "-destroy")
	}

	planCmd := exec.CommandContext(ctx, "terraform", planArgs...)
	planCmd.Dir = workDir
	planCmd.Env = append(os.Environ(), "TF_IN_AUTOMATION=1")
	planOutput, planErr := planCmd.CombinedOutput()
	allLogs.WriteString("=== Terraform Plan ===\n")
	allLogs.Write(planOutput)
	allLogs.WriteString("\n")

	// Parse output for resource counts
	adds, changes, destroys = parsePlanOutput(string(planOutput))
	hasChanges = adds > 0 || changes > 0 || destroys > 0

	// Exit code 2 means changes detected (not an error)
	if exitErr, ok := planErr.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 2 {
			// Changes detected - this is success
			planErr = nil
		}
	}

	return string(planOutput), allLogs.String(), hasChanges, adds, changes, destroys, planErr
}

// handlePlanError handles plan execution errors
func (e *PlanExecutor) handlePlanError(ctx context.Context, runID string, planID *string, errorMsg string) error {
	fmt.Printf("Plan error for run %s: %s\n", runID, errorMsg)

	// Update plan status if we have a plan ID
	if planID != nil {
		errStatus := "errored"
		planUpdates := &domain.TFEPlanUpdate{
			Status: &errStatus,
		}
		_ = e.planRepo.UpdatePlan(ctx, *planID, planUpdates)
	}

	// Update run status
	_ = e.runRepo.UpdateRunStatus(ctx, runID, "errored")

	return fmt.Errorf("plan execution failed: %s", errorMsg)
}

// parsePlanOutput parses "Plan: X to add, Y to change, Z to destroy" from Terraform output
func parsePlanOutput(output string) (adds, changes, destroys int) {
	// Look for "Plan: X to add, Y to change, Z to destroy"
	planRegex := regexp.MustCompile(`Plan: (\d+) to add, (\d+) to change, (\d+) to destroy`)
	matches := planRegex.FindStringSubmatch(output)
	
	if len(matches) == 4 {
		fmt.Sscanf(matches[1], "%d", &adds)
		fmt.Sscanf(matches[2], "%d", &changes)
		fmt.Sscanf(matches[3], "%d", &destroys)
	}

	return
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
			fmt.Printf("Warning: failed to cleanup work dir %s: %v\n", dir, err)
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
				fmt.Printf("Removing cloud/backend configuration from %s\n", path)
				
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
	
	fmt.Printf("Successfully removed cloud/backend configuration from Terraform files\n")
	return nil
}

