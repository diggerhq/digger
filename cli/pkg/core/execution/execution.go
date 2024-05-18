package execution

import (
	"fmt"
	"github.com/samber/lo"
	"log"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/cli/pkg/core/locking"
	"github.com/diggerhq/digger/cli/pkg/core/reporting"
	"github.com/diggerhq/digger/cli/pkg/core/runners"
	"github.com/diggerhq/digger/cli/pkg/core/storage"
	"github.com/diggerhq/digger/cli/pkg/core/terraform"
	"github.com/diggerhq/digger/cli/pkg/core/utils"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/orchestrator"
)

type Executor interface {
	Plan() (*terraform.PlanSummary, bool, bool, string, string, error)
	Apply() (bool, string, error)
	Destroy() (bool, error)
}

type LockingExecutorWrapper struct {
	ProjectLock locking.ProjectLock
	Executor    Executor
}

func (l LockingExecutorWrapper) Plan() (*terraform.PlanSummary, bool, bool, string, string, error) {
	plan := ""
	locked, err := l.ProjectLock.Lock()
	if err != nil {
		return nil, false, false, "", "", fmt.Errorf("digger plan, error locking project: %v", err)
	}
	log.Printf("Lock result: %t\n", locked)
	if locked {
		return l.Executor.Plan()
	} else {
		return nil, false, false, plan, "", nil
	}
}

func (l LockingExecutorWrapper) Apply() (bool, string, error) {
	locked, err := l.ProjectLock.Lock()
	if err != nil {
		msg := fmt.Sprintf("digger apply, error locking project: %v", err)
		return false, msg, fmt.Errorf(msg)
	}
	log.Printf("Lock result: %t\n", locked)
	if locked {
		return l.Executor.Apply()
	} else {
		return false, "couldn't lock ", nil
	}
}

func (l LockingExecutorWrapper) Destroy() (bool, error) {
	locked, err := l.ProjectLock.Lock()
	if err != nil {
		return false, fmt.Errorf("digger destroy, error locking project: %v", err)
	}
	log.Printf("Lock result: %t\n", locked)
	if locked {
		return l.Executor.Destroy()
	} else {
		return false, nil
	}
}

func (l LockingExecutorWrapper) Unlock() error {
	err := l.ProjectLock.ForceUnlock()
	if err != nil {
		return fmt.Errorf("failed to aquire lock: %s, %v", l.ProjectLock.LockId(), err)
	}
	return nil
}

func (l LockingExecutorWrapper) Lock() error {
	_, err := l.ProjectLock.Lock()
	if err != nil {
		return fmt.Errorf("failed to aquire lock: %s, %v", l.ProjectLock.LockId(), err)
	}
	return nil
}

type DiggerExecutor struct {
	ProjectNamespace  string
	ProjectName       string
	ProjectPath       string
	StateEnvVars      map[string]string
	CommandEnvVars    map[string]string
	RunEnvVars        map[string]string
	ApplyStage        *orchestrator.Stage
	PlanStage         *orchestrator.Stage
	CommandRunner     runners.CommandRun
	TerraformExecutor terraform.TerraformExecutor
	Reporter          reporting.Reporter
	PlanStorage       storage.PlanStorage
	PlanPathProvider  PlanPathProvider
}

type DiggerExecutorResult struct {
	PlanResult  *DiggerExecutorPlanResult
	ApplyResult *DiggerExecutorApplyResult
}

type DiggerExecutorApplyResult struct {
}

type DiggerExecutorPlanResult struct {
	PlanSummary terraform.PlanSummary
}

type PlanPathProvider interface {
	LocalPlanFilePath() string
	StoredPlanFilePath() string
	ArtifactName() string
}

type ProjectPathProvider struct {
	PRNumber         *int
	ProjectPath      string
	ProjectNamespace string
	ProjectName      string
}

func (d ProjectPathProvider) ArtifactName() string {
	return d.ProjectName
}

func (d ProjectPathProvider) StoredPlanFilePath() string {
	if d.PRNumber != nil {
		prNumber := strconv.Itoa(*d.PRNumber)
		return strings.ReplaceAll(d.ProjectNamespace, "/", "-") + "-" + prNumber + "-" + d.ProjectName + ".tfplan"
	} else {
		return strings.ReplaceAll(d.ProjectNamespace, "/", "-") + "-" + d.ProjectName + ".tfplan"
	}

}

func (d ProjectPathProvider) LocalPlanFilePath() string {
	return path.Join(d.ProjectPath, d.StoredPlanFilePath())
}

func (d DiggerExecutor) RetrievePlanJson() (string, error) {
	executor := d
	planStorage := executor.PlanStorage
	planPathProvider := executor.PlanPathProvider
	storedPlanExists, err := planStorage.PlanExists(planPathProvider.ArtifactName(), planPathProvider.StoredPlanFilePath())
	if err != nil {
		return "", fmt.Errorf("failed to check if stored plan exists. %v", err)
	}
	if storedPlanExists {
		log.Printf("Pre-apply plan retrieval: stored plan exists in artefact, retrieving")
		storedPlanPath, err := planStorage.RetrievePlan(planPathProvider.LocalPlanFilePath(), planPathProvider.ArtifactName(), planPathProvider.StoredPlanFilePath())
		if err != nil {
			return "", fmt.Errorf("failed to retrieve stored plan path. %v", err)
		}

		// Running terraform init to load provider
		for _, step := range executor.PlanStage.Steps {
			if step.Action == "init" {
				executor.TerraformExecutor.Init(step.ExtraArgs, executor.StateEnvVars)
				break
			}
		}

		showArgs := []string{"-no-color", "-json", *storedPlanPath}
		terraformPlanOutput, _, _ := executor.TerraformExecutor.Show(showArgs, executor.CommandEnvVars)
		return terraformPlanOutput, nil

	} else {
		return "", fmt.Errorf("stored plan does not exist")
	}
}

func (d DiggerExecutor) Plan() (*terraform.PlanSummary, bool, bool, string, string, error) {
	plan := ""
	terraformPlanOutput := ""
	planSummary := &terraform.PlanSummary{}
	isEmptyPlan := true
	var planSteps []orchestrator.Step

	if d.PlanStage != nil {
		planSteps = d.PlanStage.Steps
	} else {
		planSteps = []orchestrator.Step{
			{
				Action: "init",
			},
			{
				Action: "plan",
			},
		}
	}
	for _, step := range planSteps {
		if step.Action == "init" {
			_, stderr, err := d.TerraformExecutor.Init(step.ExtraArgs, d.StateEnvVars)
			if err != nil {
				reportError(d.Reporter, stderr)
				return nil, false, false, "", "", fmt.Errorf("error running init: %v", err)
			}
		}
		if step.Action == "plan" {
			planArgs := []string{"-out", d.PlanPathProvider.LocalPlanFilePath(), "-lock-timeout=3m"}
			planArgs = append(planArgs, step.ExtraArgs...)
			_, stdout, stderr, err := d.TerraformExecutor.Plan(planArgs, d.CommandEnvVars)
			if err != nil {
				return nil, false, false, "", "", fmt.Errorf("error executing plan: %v", err)
			}
			showArgs := []string{"-no-color", "-json", d.PlanPathProvider.LocalPlanFilePath()}
			terraformPlanOutput, _, _ = d.TerraformExecutor.Show(showArgs, d.CommandEnvVars)

			isEmptyPlan, planSummary, err = terraform.GetPlanSummary(terraformPlanOutput)
			if err != nil {
				return nil, false, false, "", "", fmt.Errorf("error checking for empty plan: %v", err)
			}

			if !isEmptyPlan {
				nonEmptyPlanFilepath := strings.Replace(d.PlanPathProvider.LocalPlanFilePath(), d.PlanPathProvider.StoredPlanFilePath(), "isNonEmptyPlan.txt", 1)
				file, err := os.Create(nonEmptyPlanFilepath)
				if err != nil {
					return nil, false, false, "", "", fmt.Errorf("unable to create file: %v", err)
				}
				defer file.Close()
			}

			if err != nil {
				return nil, false, false, "", "", fmt.Errorf("error executing plan: %v", err)
			}
			if d.PlanStorage != nil {

				fileBytes, err := os.ReadFile(d.PlanPathProvider.LocalPlanFilePath())
				if err != nil {
					fmt.Println("Error reading file:", err)
					return nil, false, false, "", "", fmt.Errorf("error reading file bytes: %v", err)
				}

				err = d.PlanStorage.StorePlanFile(fileBytes, d.PlanPathProvider.ArtifactName(), d.PlanPathProvider.StoredPlanFilePath())
				if err != nil {
					fmt.Println("Error storing artifact file:", err)
					return nil, false, false, "", "", fmt.Errorf("error storing artifact file: %v", err)
				}
			}
			plan = cleanupTerraformPlan(!isEmptyPlan, err, stdout, stderr)
			if err != nil {
				log.Printf("error publishing comment: %v", err)
			}
		}
		if step.Action == "run" {
			var commands []string
			if os.Getenv("ACTIVATE_VENV") == "true" {
				commands = append(commands, fmt.Sprintf("source %v/.venv/bin/activate", os.Getenv("GITHUB_WORKSPACE")))
			}
			commands = append(commands, step.Value)
			log.Printf("Running %v for **%v**\n", step.Value, d.ProjectNamespace+"#"+d.ProjectName)
			_, _, err := d.CommandRunner.Run(d.ProjectPath, step.Shell, commands, d.RunEnvVars)
			if err != nil {
				return nil, false, false, "", "", fmt.Errorf("error running command: %v", err)
			}
		}
	}
	reportAdditionalOutput(d.Reporter, d.projectId())
	return planSummary, true, !isEmptyPlan, plan, terraformPlanOutput, nil
}

func reportError(r reporting.Reporter, stderr string) {
	if r.SupportsMarkdown() {
		_, _, commentErr := r.Report(stderr, utils.AsCollapsibleComment("Error during init.", false))
		if commentErr != nil {
			log.Printf("error publishing comment: %v", commentErr)
		}
	} else {
		_, _, commentErr := r.Report(stderr, utils.AsComment("Error during init."))
		if commentErr != nil {
			log.Printf("error publishing comment: %v", commentErr)
		}
	}
}

func (d DiggerExecutor) Apply() (bool, string, error) {
	var applyOutput string
	var plansFilename *string
	if d.PlanStorage != nil {
		var err error
		plansFilename, err = d.PlanStorage.RetrievePlan(d.PlanPathProvider.LocalPlanFilePath(), d.PlanPathProvider.ArtifactName(), d.PlanPathProvider.StoredPlanFilePath())
		if err != nil {
			return false, "", fmt.Errorf("error retrieving plan: %v", err)
		}
	}

	var applySteps []orchestrator.Step

	if d.ApplyStage != nil {
		applySteps = d.ApplyStage.Steps
	} else {
		applySteps = []orchestrator.Step{
			{
				Action: "init",
			},
			{
				Action: "apply",
			},
		}
	}

	for _, step := range applySteps {
		if step.Action == "init" {
			stdout, stderr, err := d.TerraformExecutor.Init(step.ExtraArgs, d.StateEnvVars)
			if err != nil {
				reportTerraformError(d.Reporter, stderr)
				return false, stdout, fmt.Errorf("error running init: %v", err)
			}
		}
		if step.Action == "apply" {
			applyArgs := []string{"-lock-timeout=3m"}
			applyArgs = append(applyArgs, step.ExtraArgs...)
			stdout, stderr, err := d.TerraformExecutor.Apply(applyArgs, plansFilename, d.CommandEnvVars)
			applyOutput = cleanupTerraformApply(true, err, stdout, stderr)
			reportTerraformApplyOutput(d.Reporter, d.projectId(), applyOutput)
			if err != nil {
				reportApplyError(d.Reporter, err)
				return false, stdout, fmt.Errorf("error executing apply: %v", err)
			}
		}
		if step.Action == "run" {
			var commands []string
			if os.Getenv("ACTIVATE_VENV") == "true" {
				commands = append(commands, fmt.Sprintf("source %v/.venv/bin/activate", os.Getenv("GITHUB_WORKSPACE")))
			}
			commands = append(commands, step.Value)
			log.Printf("Running %v for **%v**\n", step.Value, d.ProjectNamespace+"#"+d.ProjectName)
			_, stderr, err := d.CommandRunner.Run(d.ProjectPath, step.Shell, commands, d.RunEnvVars)
			if err != nil {
				return false, stderr, fmt.Errorf("error running command: %v", err)
			}
		}
	}
	reportAdditionalOutput(d.Reporter, d.projectId())
	return true, applyOutput, nil
}

func reportApplyError(r reporting.Reporter, err error) {
	if r.SupportsMarkdown() {
		_, _, commentErr := r.Report(err.Error(), utils.AsCollapsibleComment("Error during applying.", false))
		if commentErr != nil {
			log.Printf("error publishing comment: %v", err)
		}
	} else {
		_, _, commentErr := r.Report(err.Error(), utils.AsComment("Error during applying."))
		if commentErr != nil {
			log.Printf("error publishing comment: %v", err)
		}
	}
}

func reportTerraformApplyOutput(r reporting.Reporter, projectId string, applyOutput string) {
	var formatter func(string) string
	if r.SupportsMarkdown() {
		formatter = utils.GetTerraformOutputAsCollapsibleComment("Apply output", false)
	} else {
		formatter = utils.GetTerraformOutputAsComment("Apply output")
	}

	_, _, commentErr := r.Report(applyOutput, formatter)
	if commentErr != nil {
		log.Printf("error publishing comment: %v", commentErr)
	}
}

func reportTerraformError(r reporting.Reporter, stderr string) {
	if r.SupportsMarkdown() {
		_, _, commentErr := r.Report(stderr, utils.GetTerraformOutputAsCollapsibleComment("Error during init.", false))
		if commentErr != nil {
			log.Printf("error publishing comment: %v", commentErr)
		}
	} else {
		_, _, commentErr := r.Report(stderr, utils.GetTerraformOutputAsComment("Error during init."))
		if commentErr != nil {
			log.Printf("error publishing comment: %v", commentErr)
		}
	}
}

func reportAdditionalOutput(r reporting.Reporter, projectId string) {
	var formatter func(string) string
	if r.SupportsMarkdown() {
		formatter = utils.GetTerraformOutputAsCollapsibleComment("Additional output for <b>"+projectId+"</b>", false)
	} else {
		formatter = utils.GetTerraformOutputAsComment("Additional output for " + projectId)
	}
	diggerOutPath := os.Getenv("DIGGER_OUT")
	if _, err := os.Stat(diggerOutPath); err == nil {
		output, _ := os.ReadFile(diggerOutPath)
		outputStr := string(output)
		if len(outputStr) > 0 {
			_, _, commentErr := r.Report(outputStr, formatter)
			if commentErr != nil {
				log.Printf("error publishing comment: %v", commentErr)
			}
		} else {
			log.Printf("empty $DIGGER_OUT file at: %v", diggerOutPath)
		}
		err = os.Remove(diggerOutPath)
		if err != nil {
			log.Printf("error removing $DIGGER_OUT file at: %v, %v", diggerOutPath, err)
		}
	} else {
		log.Printf("no $DIGGER_OUT file at: %v", diggerOutPath)
	}
}

func (d DiggerExecutor) Destroy() (bool, error) {

	destroySteps := []configuration.Step{
		{
			Action: "init",
		},
		{
			Action: "destroy",
		},
	}

	for _, step := range destroySteps {
		if step.Action == "init" {
			_, stderr, err := d.TerraformExecutor.Init(step.ExtraArgs, d.StateEnvVars)
			if err != nil {
				reportError(d.Reporter, stderr)
				return false, fmt.Errorf("error running init: %v", err)
			}
		}
		if step.Action == "destroy" {
			applyArgs := []string{"-lock-timeout=3m"}
			applyArgs = append(applyArgs, step.ExtraArgs...)
			d.TerraformExecutor.Destroy(applyArgs, d.CommandEnvVars)
		}
	}
	return true, nil
}

func cleanupTerraformOutput(nonEmptyOutput bool, planError error, stdout string, stderr string, regexStr *string) string {
	var errorStr string

	// removes output of terraform -version command that terraform-exec executes on every run
	i := strings.Index(stdout, "Initializing the backend...")
	if i != -1 {
		stdout = stdout[i:]
	}
	endPos := len(stdout)

	if planError != nil {
		if stderr != "" {
			errorStr = stderr
		} else if stdout != "" {
			errorStr = stdout
		}
		return errorStr
	}

	delimiters := []string{
		"Terraform will perform the following actions:",
		"OpenTofu will perform the following actions:",
		"No changes. Your infrastructure matches the configuration.",
	}
	indices := lo.FilterMap(delimiters, func(delimiter string, i int) (int, bool) {
		index := strings.Index(stdout, delimiter)
		return index, index > 0
	})
	var startPos int
	if len(indices) > 0 {
		startPos = indices[0]
	} else {
		startPos = 0
	}

	if regexStr != nil {
		regex := regexp.MustCompile(*regexStr)
		matches := regex.FindStringSubmatch(stdout)
		if len(matches) > 0 {
			firstMatch := matches[0]
			endPos = strings.LastIndex(stdout, firstMatch) + len(firstMatch)
		}
	}

	// This should not happen but in case we get here we avoid slice bounds out of range exception by resetting endPos
	if endPos <= startPos {
		endPos = len(stdout)
	}
	return stdout[startPos:endPos]
}

func cleanupTerraformApply(nonEmptyPlan bool, planError error, stdout string, stderr string) string {
	return cleanupTerraformOutput(nonEmptyPlan, planError, stdout, stderr, nil)
}

func cleanupTerraformPlan(nonEmptyPlan bool, planError error, stdout string, stderr string) string {
	regex := `───────────.+`
	return cleanupTerraformOutput(nonEmptyPlan, planError, stdout, stderr, &regex)
}

func (d DiggerExecutor) projectId() string {
	return d.ProjectNamespace + "#" + d.ProjectName
}
