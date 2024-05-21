package orchestrator

import (
	"slices"

	"github.com/diggerhq/digger/libs/digger_config"
)

type StepJson struct {
	Action    string   `json:"action"`
	Value     string   `json:"value"`
	ExtraArgs []string `json:"extraArgs"`
	Shell     string   `json:"shell"`
}

type StageJson struct {
	Steps []StepJson `json:"steps"`
}

type JobJson struct {
	ProjectName             string            `json:"projectName"`
	ProjectDir              string            `json:"projectDir"`
	ProjectWorkspace        string            `json:"projectWorkspace"`
	Terragrunt              bool              `json:"terragrunt"`
	Commands                []string          `json:"commands"`
	ApplyStage              StageJson         `json:"applyStage"`
	PlanStage               StageJson         `json:"planStage"`
	PullRequestNumber       *int              `json:"pullRequestNumber"`
	EventName               string            `json:"eventName"`
	RequestedBy             string            `json:"requestedBy"`
	Namespace               string            `json:"namespace"`
	RunEnvVars              map[string]string `json:"runEnvVars"`
	StateEnvVars            map[string]string `json:"stateEnvVars"`
	CommandEnvVars          map[string]string `json:"commandEnvVars"`
	Region                  string            `json:"region"`
	StateRoleName           string            `json:"state_role_name"`
	CommandRoleName         string            `json:"command_role_name"`
	BackendHostname         string            `json:"backend_hostname"`
	BackendOrganisationName string            `json:"backend_organisation_hostname"`
	BackendJobToken         string            `json:"backend_job_token"`
}

func (j *JobJson) IsPlan() bool {
	return slices.Contains(j.Commands, "digger plan")
}

func (j *JobJson) IsApply() bool {
	return slices.Contains(j.Commands, "digger apply")
}

func JobToJson(job Job, organisationName string, jobToken string, backendHostname string, project digger_config.Project) JobJson {
	stateRole, commandRole, region := "", "", ""
	if project.AwsRoleToAssume != nil {
		region = project.AwsRoleToAssume.Region
		stateRole = project.AwsRoleToAssume.State
		commandRole = project.AwsRoleToAssume.Command

	}
	return JobJson{
		ProjectName:             job.ProjectName,
		ProjectDir:              job.ProjectDir,
		ProjectWorkspace:        job.ProjectWorkspace,
		Terragrunt:              job.Terragrunt,
		Commands:                job.Commands,
		ApplyStage:              stageToJson(job.ApplyStage),
		PlanStage:               stageToJson(job.PlanStage),
		PullRequestNumber:       job.PullRequestNumber,
		EventName:               job.EventName,
		RequestedBy:             job.RequestedBy,
		Namespace:               job.Namespace,
		RunEnvVars:              job.RunEnvVars,
		StateEnvVars:            job.StateEnvVars,
		CommandEnvVars:          job.CommandEnvVars,
		Region:                  region,
		StateRoleName:           stateRole,
		CommandRoleName:         commandRole,
		BackendHostname:         backendHostname,
		BackendJobToken:         jobToken,
		BackendOrganisationName: organisationName,
	}
}

func JsonToJob(jobJson JobJson) Job {
	return Job{
		ProjectName:        jobJson.ProjectName,
		ProjectDir:         jobJson.ProjectDir,
		ProjectWorkspace:   jobJson.ProjectWorkspace,
		Terragrunt:         jobJson.Terragrunt,
		Commands:           jobJson.Commands,
		ApplyStage:         jsonToStage(jobJson.ApplyStage),
		PlanStage:          jsonToStage(jobJson.PlanStage),
		PullRequestNumber:  jobJson.PullRequestNumber,
		EventName:          jobJson.EventName,
		RequestedBy:        jobJson.RequestedBy,
		Namespace:          jobJson.Namespace,
		RunEnvVars:         jobJson.RunEnvVars,
		StateEnvVars:       jobJson.StateEnvVars,
		CommandEnvVars:     jobJson.CommandEnvVars,
		StateEnvProvider:   GetProviderFromRole(jobJson.StateRoleName),
		CommandEnvProvider: GetProviderFromRole(jobJson.CommandRoleName),
	}
}

func jsonToStage(stageJson StageJson) *Stage {
	if len(stageJson.Steps) == 0 {
		return nil
	}
	steps := make([]Step, len(stageJson.Steps))
	for i, step := range stageJson.Steps {
		steps[i] = Step{
			Action:    step.Action,
			Value:     step.Value,
			ExtraArgs: step.ExtraArgs,
			Shell:     step.Shell,
		}
	}
	return &Stage{
		Steps: steps,
	}
}

func stageToJson(stage *Stage) StageJson {
	if stage == nil {
		return StageJson{}
	}
	steps := make([]StepJson, len(stage.Steps))
	for i, step := range stage.Steps {
		steps[i] = StepJson{
			Action:    step.Action,
			Value:     step.Value,
			ExtraArgs: step.ExtraArgs,
			Shell:     step.Shell,
		}
	}
	return StageJson{
		Steps: steps,
	}
}

func IsPlanJobSpecs(jobs []JobJson) bool {
	isPlan := true
	for _, job := range jobs {
		isPlan = isPlan && job.IsPlan()
	}
	return isPlan
}

func IsApplyJobSpecs(jobs []JobJson) bool {
	isApply := true
	for _, job := range jobs {
		isApply = isApply && job.IsApply()
	}
	return isApply
}
