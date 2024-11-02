package execution

import (
	"fmt"
	"github.com/diggerhq/digger/libs/comment_utils/utils"
	"github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/storage"
	"github.com/diggerhq/digger/libs/terraform_utils"
	"github.com/samber/lo"
	"log"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	configuration "github.com/diggerhq/digger/libs/digger_config"
)

type Executor interface {
	Plan() (*terraform_utils.TerraformSummary, bool, bool, string, string, error)
	Apply() (*terraform_utils.TerraformSummary, bool, string, error)
	Destroy() (bool, error)
}

type LockingExecutorWrapper struct {
	ProjectLock locking.ProjectLock
	Executor    Executor
}

func (l LockingExecutorWrapper) Plan() (*terraform_utils.TerraformSummary, bool, bool, string, string, error) {
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

func (l LockingExecutorWrapper) Apply() (*terraform_utils.TerraformSummary, bool, string, error) {
	locked, err := l.ProjectLock.Lock()
	if err != nil {
		msg := fmt.Sprintf("digger apply, error locking project: %v", err)
		return nil, false, msg, fmt.Errorf(msg)
	}
	log.Printf("Lock result: %t\n", locked)
	if locked {
		return l.Executor.Apply()
	} else {
		return nil, false, "couldn't lock ", nil
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
		return fmt.Errorf("failed to acquire lock: %s, %v", l.ProjectLock.LockId(), err)
	}
	return nil
}

func (l LockingExecutorWrapper) Lock() error {
	_, err := l.ProjectLock.Lock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %s, %v", l.ProjectLock.LockId(), err)
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
	ApplyStage        *scheduler.Stage
	PlanStage         *scheduler.Stage
	CommandRunner     CommandRun
	TerraformExecutor TerraformExecutor
	Reporter          reporting.Reporter
	PlanStorage       storage.PlanStorage
	PlanPathProvider  PlanPathProvider
}

type DiggerOperationType string

var DiggerOparationTypePlan DiggerOperationType = "plan"
var DiggerOparationTypeApply DiggerOperationType = "apply"

type DiggerExecutorResult struct {
	OperationType   DiggerOperationType
	TerraformOutput string
	PlanResult      *DiggerExecutorPlanResult
	ApplyResult     *DiggerExecutorApplyResult
}

type DiggerExecutorApplyResult struct {
	ApplySummary terraform_utils.TerraformSummary
}

type DiggerExecutorPlanResult struct {
	PlanSummary   terraform_utils.TerraformSummary
	TerraformJson string
}

func (d DiggerExecutorResult) GetTerraformSummary() terraform_utils.TerraformSummary {
	var summary terraform_utils.TerraformSummary
	if d.OperationType == DiggerOparationTypePlan && d.PlanResult != nil {
		summary = d.PlanResult.PlanSummary
	} else if d.OperationType == DiggerOparationTypeApply && d.ApplyResult != nil {
		summary = d.ApplyResult.ApplySummary
	}
	return summary
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

func (d DiggerExecutor) Plan() (*terraform_utils.TerraformSummary, bool, bool, string, string, error) {
	plan := ""
	terraformPlanOutput := ""
	planSummary := &terraform_utils.TerraformSummary{}
	isEmptyPlan := true
	var planSteps []scheduler.Step

	if d.PlanStage != nil {
		planSteps = d.PlanStage.Steps
	} else {
		planSteps = []scheduler.Step{
			{
				Action: "init",
			},
			{
				Action: "plan",
			},
		}
	}
	for _, step := range planSteps {
		log.Printf(" Running step: %v\n", step.Action)
		if step.Action == "init" {
			_, stderr, err := d.TerraformExecutor.Init(step.ExtraArgs, d.StateEnvVars)
			if err != nil {
				reportError(d.Reporter, stderr)
				return nil, false, false, "", "", fmt.Errorf("error running init: %v", err)
			}
		}
		if step.Action == "plan" {
			planArgs := []string{}

			// TODO remove those only for pulumi project
			//planArgs = append(planArgs, []string{"-out", d.PlanPathProvider.LocalPlanFilePath(), "-lock-timeout=3m"}...)

			planArgs = append(planArgs, step.ExtraArgs...)

			_, stdout, stderr, err := d.TerraformExecutor.Plan(planArgs, d.CommandEnvVars)
			if err != nil {
				return nil, false, false, "", "", fmt.Errorf("error executing plan: %v", err)
			}
			showArgs := []string{"-no-color", "-json", d.PlanPathProvider.LocalPlanFilePath()}
			terraformPlanOutput, _, _ = d.TerraformExecutor.Show(showArgs, d.CommandEnvVars)

			// TODO: removal of this if statement
			if terraformPlanOutput == "" {
				isEmptyPlan = false
			} else {
				isEmptyPlan, planSummary, err = terraform_utils.GetSummaryFromPlanJson(terraformPlanOutput)
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

func (d DiggerExecutor) Apply() (*terraform_utils.TerraformSummary, bool, string, error) {
	var applyOutput string
	var plansFilename *string
	summary := terraform_utils.TerraformSummary{}
	if d.PlanStorage != nil {
		var err error
		plansFilename, err = d.PlanStorage.RetrievePlan(d.PlanPathProvider.LocalPlanFilePath(), d.PlanPathProvider.ArtifactName(), d.PlanPathProvider.StoredPlanFilePath())
		if err != nil {
			return nil, false, "", fmt.Errorf("error retrieving plan: %v", err)
		}
	}

	var applySteps []scheduler.Step

	if d.ApplyStage != nil {
		applySteps = d.ApplyStage.Steps
	} else {
		applySteps = []scheduler.Step{
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
				return nil, false, stdout, fmt.Errorf("error running init: %v", err)
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
				return nil, false, stdout, fmt.Errorf("error executing apply: %v", err)
			}

			summary, err = terraform_utils.GetSummaryFromTerraformApplyOutput(stdout)
			if err != nil {
				log.Printf("Warning: get summary from apply output failed: %v", err)
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
				return nil, false, stderr, fmt.Errorf("error running command: %v", err)
			}
		}
	}
	reportAdditionalOutput(d.Reporter, d.projectId())
	return &summary, true, applyOutput, nil
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

// this will log an exit code and error based on the executor of the executor drivers are by filename
func logCommandFail(exitCode int, err error) {

	_, filename, _, ok := runtime.Caller(1)
	if ok {
		executor := strings.TrimSuffix(path.Base(filename), path.Ext(filename))
		log.Printf("Command failed in %v with exit code %v and error %v", executor, exitCode, err)
	} else {
		log.Printf("Command failed in unknown executor with exit code %v and error %v", exitCode, err)
	}
}
