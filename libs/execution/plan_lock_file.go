package execution

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const planLockFileEnvVar = "PLAN_UPLOAD_LOCK_FILE"

func (d ProjectPathProvider) LocalPlanLockFilePath(lockFilePath string) string {
	return filepath.Join(d.ProjectPath, filepath.FromSlash(normalizePlanLockFilePath(lockFilePath)))
}

func (d ProjectPathProvider) StoredPlanLockFilePath(lockFilePath string) string {
	storedPlanPath := d.StoredPlanFilePath()
	storedPlanExtension := path.Ext(storedPlanPath)
	storedPlanBase := strings.TrimSuffix(storedPlanPath, storedPlanExtension)
	lockFileName := path.Base(normalizePlanLockFilePath(lockFilePath))
	lockFileName = strings.TrimLeft(lockFileName, ".")
	if lockFileName == "" {
		lockFileName = "lock"
	}
	return storedPlanBase + "." + lockFileName
}

func PlanLockFilePath() string {
	return normalizePlanLockFilePath(os.Getenv(planLockFileEnvVar))
}

func ValidatePlanLockFilePath(lockFilePath string) error {
	lockFilePath = normalizePlanLockFilePath(lockFilePath)
	if lockFilePath == "" {
		return nil
	}
	if path.IsAbs(lockFilePath) || filepath.IsAbs(lockFilePath) {
		return fmt.Errorf("%s must be relative to the project directory", planLockFileEnvVar)
	}
	if lockFilePath == "." || lockFilePath == ".." || strings.HasPrefix(lockFilePath, "../") {
		return fmt.Errorf("%s must not escape the project directory", planLockFileEnvVar)
	}
	return nil
}

func normalizePlanLockFilePath(lockFilePath string) string {
	lockFilePath = strings.TrimSpace(lockFilePath)
	if lockFilePath == "" {
		return ""
	}
	lockFilePath = strings.ReplaceAll(lockFilePath, "\\", "/")
	return path.Clean(lockFilePath)
}

func (d DiggerExecutor) storePlanLockFile() error {
	lockFilePath := PlanLockFilePath()
	if lockFilePath == "" {
		return nil
	}
	if err := ValidatePlanLockFilePath(lockFilePath); err != nil {
		return err
	}

	localLockFilePath := d.PlanPathProvider.LocalPlanLockFilePath(lockFilePath)
	fileBytes, err := os.ReadFile(localLockFilePath)
	if err != nil {
		return fmt.Errorf("error reading plan lock file %q: %v", localLockFilePath, err)
	}

	storedPlanLockFilePath := d.PlanPathProvider.StoredPlanLockFilePath(lockFilePath)
	err = d.PlanStorage.StorePlanFile(fileBytes, storedPlanLockFilePath, storedPlanLockFilePath)
	if err != nil {
		return fmt.Errorf("error storing plan lock file %q: %v", storedPlanLockFilePath, err)
	}

	return nil
}

func (d DiggerExecutor) retrievePlanLockFile() error {
	lockFilePath := PlanLockFilePath()
	if lockFilePath == "" {
		return nil
	}
	if err := ValidatePlanLockFilePath(lockFilePath); err != nil {
		return err
	}

	localLockFilePath := d.PlanPathProvider.LocalPlanLockFilePath(lockFilePath)
	localLockFileDir := filepath.Dir(localLockFilePath)
	if err := os.MkdirAll(localLockFileDir, 0755); err != nil {
		return fmt.Errorf("error creating plan lock file directory %q: %v", localLockFileDir, err)
	}

	storedPlanLockFilePath := d.PlanPathProvider.StoredPlanLockFilePath(lockFilePath)
	localStoredPlanLockFilePath := localStoredPlanLockFilePath(d.PlanPathProvider, lockFilePath)
	if err := os.MkdirAll(filepath.Dir(localStoredPlanLockFilePath), 0755); err != nil {
		return fmt.Errorf("error creating stored plan lock file directory %q: %v", filepath.Dir(localStoredPlanLockFilePath), err)
	}

	retrievedPlanLockFilePath, err := d.PlanStorage.RetrievePlan(localStoredPlanLockFilePath, storedPlanLockFilePath, storedPlanLockFilePath)
	if err != nil {
		return fmt.Errorf("error retrieving plan lock file %q: %v", storedPlanLockFilePath, err)
	}
	if retrievedPlanLockFilePath == nil || *retrievedPlanLockFilePath == "" {
		return fmt.Errorf("error retrieving plan lock file %q: no file was returned", storedPlanLockFilePath)
	}

	if filepath.Clean(*retrievedPlanLockFilePath) == filepath.Clean(localLockFilePath) {
		return nil
	}

	fileBytes, err := os.ReadFile(*retrievedPlanLockFilePath)
	if err != nil {
		return fmt.Errorf("error reading retrieved plan lock file %q: %v", *retrievedPlanLockFilePath, err)
	}
	if err := os.WriteFile(localLockFilePath, fileBytes, 0644); err != nil {
		return fmt.Errorf("error writing plan lock file %q: %v", localLockFilePath, err)
	}

	return nil
}

func localStoredPlanLockFilePath(planPathProvider PlanPathProvider, lockFilePath string) string {
	localPlanFilePath := planPathProvider.LocalPlanFilePath()
	storedPlanFilePath := planPathProvider.StoredPlanFilePath()
	storedPlanLockFilePath := planPathProvider.StoredPlanLockFilePath(lockFilePath)
	if strings.HasSuffix(localPlanFilePath, storedPlanFilePath) {
		return strings.TrimSuffix(localPlanFilePath, storedPlanFilePath) + storedPlanLockFilePath
	}
	return filepath.Join(filepath.Dir(localPlanFilePath), storedPlanLockFilePath)
}
