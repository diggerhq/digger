package execution

import (
	"digger/pkg/core/locking"
	"digger/pkg/core/reporting"
	"digger/pkg/core/runners"
	"digger/pkg/core/storage"
	"digger/pkg/core/terraform"
	"digger/pkg/core/utils"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	configuration "github.com/diggerhq/lib-digger-config"
	orchestrator "github.com/diggerhq/lib-orchestrator"
)

type Executor interface {
	Plan() (bool, bool, string, string, error)
	Apply() (bool, string, error)
	Destroy() (bool, error)
}

type LockingExecutorWrapper struct {
	ProjectLock locking.ProjectLock
	Executor    Executor
}

func (l LockingExecutorWrapper) Plan() (bool, bool, string, string, error) {
	plan := ""
	locked, err := l.ProjectLock.Lock()
	if err != nil {
		return false, false, "", "", fmt.Errorf("error locking project: %v", err)
	}
	log.Printf("Lock result: %t\n", locked)
	if locked {
		return l.Executor.Plan()
	} else {
		return false, false, plan, "", nil
	}
}

func (l LockingExecutorWrapper) Apply() (bool, string, error) {
	locked, err := l.ProjectLock.Lock()
	if err != nil {
		msg := fmt.Sprintf("error locking project: %v", err)
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
		return false, fmt.Errorf("error locking project: %v", err)
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
	ApplyStage        *orchestrator.Stage
	PlanStage         *orchestrator.Stage
	CommandRunner     runners.CommandRun
	TerraformExecutor terraform.TerraformExecutor
	Reporter          reporting.Reporter
	PlanStorage       storage.PlanStorage
	PlanPathProvider  PlanPathProvider
}

type PlanPathProvider interface {
	LocalPlanFilePath() string
	StoredPlanFilePath() string
	PlanFileName() string
}

type ProjectPathProvider struct {
	ProjectPath      string
	ProjectNamespace string
	ProjectName      string
}

func (d ProjectPathProvider) PlanFileName() string {
	return strings.ReplaceAll(d.ProjectNamespace, "/", "-") + "#" + d.ProjectName + ".tfplan"
}

func (d ProjectPathProvider) LocalPlanFilePath() string {
	return path.Join(d.ProjectPath, d.PlanFileName())
}

func (d ProjectPathProvider) StoredPlanFilePath() string {
	return path.Join(d.ProjectNamespace, d.PlanFileName())
}

func (d DiggerExecutor) Plan() (bool, bool, string, string, error) {
	plan := ""
	terraformPlanOutput := ""
	isNonEmptyPlan := false
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
				commentErr := d.Reporter.Report(stderr, utils.GetTerraformOutputAsCollapsibleComment("Error during init."))
				if commentErr != nil {
					fmt.Printf("error publishing comment: %v", err)
				}
				return false, false, "", "", fmt.Errorf("error running init: %v", err)
			}
		}
		if step.Action == "plan" {
			planArgs := []string{"-out", d.PlanPathProvider.LocalPlanFilePath(), "-lock-timeout=3m"}
			planArgs = append(planArgs, step.ExtraArgs...)
			nonEmptyPlan, stdout, stderr, err := d.TerraformExecutor.Plan(planArgs, d.CommandEnvVars)
			isNonEmptyPlan = nonEmptyPlan
			if isNonEmptyPlan {
				nonEmptyPlanFilepath := strings.Replace(d.PlanPathProvider.LocalPlanFilePath(), d.PlanPathProvider.PlanFileName(), "isNonEmptyPlan.txt", 1)
				file, err := os.Create(nonEmptyPlanFilepath)
				if err != nil {
					return false, false, "", "", fmt.Errorf("unable to create file: %v", err)
				}
				defer file.Close()
			}

			if err != nil {
				return false, false, "", "", fmt.Errorf("error executing plan: %v", err)
			}
			if d.PlanStorage != nil {
				planExists, err := d.PlanStorage.PlanExists(d.PlanPathProvider.StoredPlanFilePath())
				if err != nil {
					return false, false, "", "", fmt.Errorf("error checking if plan exists: %v", err)
				}

				if planExists {
					err = d.PlanStorage.DeleteStoredPlan(d.PlanPathProvider.StoredPlanFilePath())
					if err != nil {
						return false, false, "", "", fmt.Errorf("error deleting plan: %v", err)
					}
				}

				err = d.PlanStorage.StorePlan(d.PlanPathProvider.LocalPlanFilePath(), d.PlanPathProvider.StoredPlanFilePath())
				if err != nil {
					return false, false, "", "", fmt.Errorf("error storing plan: %v", err)
				}
			}
			plan = cleanupTerraformPlan(isNonEmptyPlan, err, stdout, stderr)
			if err != nil {
				fmt.Printf("error publishing comment: %v", err)
			}

			showArgs := []string{"-no-color", "-json", d.PlanPathProvider.LocalPlanFilePath()}
			terraformPlanOutput, _, _ = d.TerraformExecutor.Show(showArgs, d.CommandEnvVars)
			// perform a rego check of plan policy and terraform json output

		}
		if step.Action == "run" {
			var commands []string
			if os.Getenv("ACTIVATE_VENV") == "true" {
				commands = append(commands, fmt.Sprintf("source %v/.venv/bin/activate", os.Getenv("GITHUB_WORKSPACE")))
			}
			commands = append(commands, step.Value)
			log.Printf("Running %v for **%v**\n", step.Value, d.ProjectNamespace+"#"+d.ProjectName)
			_, _, err := d.CommandRunner.Run(d.ProjectPath, step.Shell, commands)
			if err != nil {
				return false, false, "", "", fmt.Errorf("error running command: %v", err)
			}
		}
	}
	return true, isNonEmptyPlan, plan, terraformPlanOutput, nil
}

func (d DiggerExecutor) Apply() (bool, string, error) {
	var applyOutput string
	var plansFilename *string
	if d.PlanStorage != nil {
		var err error
		plansFilename, err = d.PlanStorage.RetrievePlan(d.PlanPathProvider.LocalPlanFilePath(), d.PlanPathProvider.StoredPlanFilePath())
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
				commentErr := d.Reporter.Report(stderr, utils.GetTerraformOutputAsCollapsibleComment("Error during init."))
				if commentErr != nil {
					fmt.Printf("error publishing comment: %v", err)
				}
				return false, stdout, fmt.Errorf("error running init: %v", err)
			}
		}
		if step.Action == "apply" {
			applyArgs := []string{"-lock-timeout=3m"}
			applyArgs = append(applyArgs, step.ExtraArgs...)
			stdout, stderr, err := d.TerraformExecutor.Apply(applyArgs, plansFilename, d.CommandEnvVars)
			applyOutput = cleanupTerraformApply(true, err, stdout, stderr)
			formatter := utils.GetTerraformOutputAsCollapsibleComment("Apply for <b>" + d.ProjectNamespace + "#" + d.ProjectName + "</b>")

			commentErr := d.Reporter.Report(applyOutput, formatter)
			if commentErr != nil {
				fmt.Printf("error publishing comment: %v", err)
			}
			if err != nil {
				commentErr = d.Reporter.Report(err.Error(), utils.AsCollapsibleComment("Error during applying."))
				if commentErr != nil {
					fmt.Printf("error publishing comment: %v", err)
				}
				return false, stdout, fmt.Errorf("error executing apply: %v", err)
			}
		}
		if step.Action == "run" {
			var commands []string
			if os.Getenv("ACTIVATE_VENV") == "true" {
				commands = append(commands, fmt.Sprintf("source %v/.venv/bin/activate", d.ProjectPath))
			}
			commands = append(commands, step.Value)
			log.Printf("Running %v for **%v**\n", step.Value, d.ProjectNamespace+"#"+d.ProjectName)
			_, stderr, err := d.CommandRunner.Run(d.ProjectPath, step.Shell, commands)
			if err != nil {
				return false, stderr, fmt.Errorf("error running command: %v", err)
			}
		}
	}
	return true, applyOutput, nil
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
				commentErr := d.Reporter.Report(stderr, utils.GetTerraformOutputAsCollapsibleComment("Error during init."))
				if commentErr != nil {
					fmt.Printf("error publishing comment: %v", err)
				}
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
	var errorStr, start string

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
	} else if nonEmptyOutput {
		start = "Terraform will perform the following actions:"
	} else {
		start = "No changes. Your infrastructure matches the configuration."
	}

	startPos := strings.Index(stdout, start)
	if startPos == -1 {
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
