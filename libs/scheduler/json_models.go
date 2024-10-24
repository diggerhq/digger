package scheduler

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
	JobType                 string            `json:"job_type"`
	ProjectName             string            `json:"projectName"`
	ProjectDir              string            `json:"projectDir"`
	ProjectWorkspace        string            `json:"projectWorkspace"`
	Terragrunt              bool              `json:"terragrunt"`
	OpenTofu                bool              `json:"opentofu"`
	Commands                []string          `json:"commands"`
	ApplyStage              StageJson         `json:"applyStage"`
	PlanStage               StageJson         `json:"planStage"`
	PullRequestNumber       *int              `json:"pullRequestNumber"`
	Commit                  string            `json:"commit"`
	Branch                  string            `json:"branch"`
	EventName               string            `json:"eventName"`
	RequestedBy             string            `json:"requestedBy"`
	Namespace               string            `json:"namespace"`
	RunEnvVars              map[string]string `json:"runEnvVars"`
	StateEnvVars            map[string]string `json:"stateEnvVars"`
	CommandEnvVars          map[string]string `json:"commandEnvVars"`
	AwsRoleRegion           string            `json:"aws_role_region"`
	StateRoleName           string            `json:"state_role_name"`
	CommandRoleName         string            `json:"command_role_name"`
	BackendHostname         string            `json:"backend_hostname"`
	BackendOrganisationName string            `json:"backend_organisation_hostname"`
	BackendJobToken         string            `json:"backend_job_token"`
	SkipMergeCheck          bool              `json:"skip_merge_check"`
}

func (j *JobJson) IsPlan() bool {
	return slices.Contains(j.Commands, "digger plan")
}

func (j *JobJson) IsApply() bool {
	return slices.Contains(j.Commands, "digger apply")
}

func JobToJson(job Job, jobType DiggerCommand, organisationName string, branch string, commitSha string, jobToken string, backendHostname string, project digger_config.Project) JobJson {
	stateRole, commandRole, region := "", "", ""

	if project.AwsRoleToAssume != nil {
		region = project.AwsRoleToAssume.AwsRoleRegion
		stateRole = project.AwsRoleToAssume.State
		commandRole = project.AwsRoleToAssume.Command
	}
	return JobJson{
		JobType:                 string(jobType),
		ProjectName:             job.ProjectName,
		ProjectDir:              job.ProjectDir,
		ProjectWorkspace:        job.ProjectWorkspace,
		OpenTofu:                job.OpenTofu,
		Terragrunt:              job.Terragrunt,
		Commands:                job.Commands,
		ApplyStage:              stageToJson(job.ApplyStage),
		PlanStage:               stageToJson(job.PlanStage),
		PullRequestNumber:       job.PullRequestNumber,
		Commit:                  commitSha,
		Branch:                  branch,
		EventName:               job.EventName,
		RequestedBy:             job.RequestedBy,
		Namespace:               job.Namespace,
		RunEnvVars:              job.RunEnvVars,
		StateEnvVars:            job.StateEnvVars,
		CommandEnvVars:          job.CommandEnvVars,
		AwsRoleRegion:           region,
		StateRoleName:           stateRole,
		CommandRoleName:         commandRole,
		BackendHostname:         backendHostname,
		BackendJobToken:         jobToken,
		BackendOrganisationName: organisationName,
		SkipMergeCheck:          job.SkipMergeCheck,
	}
}

func JsonToJob(jobJson JobJson) Job {
	return Job{
		ProjectName:        jobJson.ProjectName,
		ProjectDir:         jobJson.ProjectDir,
		ProjectWorkspace:   jobJson.ProjectWorkspace,
		OpenTofu:           jobJson.OpenTofu,
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
		StateEnvProvider:   GetProviderFromRole(jobJson.StateRoleName, jobJson.AwsRoleRegion),
		CommandEnvProvider: GetProviderFromRole(jobJson.CommandRoleName, jobJson.AwsRoleRegion),
		SkipMergeCheck:     jobJson.SkipMergeCheck,
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

func JobsSpecsToProjectMap(jobSpecs []JobJson) (map[string]JobJson, error) {
	res := make(map[string]JobJson)
	for _, jobSpec := range jobSpecs {
		res[jobSpec.ProjectName] = jobSpec
	}
	return res, nil
}
