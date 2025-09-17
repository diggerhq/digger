package scheduler

import (
	"fmt"
	"github.com/samber/lo"
	"slices"

	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	configuration "github.com/diggerhq/digger/libs/digger_config"
)

type IacType string

var IacTypeTerraform IacType = "terraform"
var IacTypePulumi IacType = "pulumi"

type Job struct {
	ProjectName        string
	ProjectAlias       string
	ProjectDir         string
	ProjectWorkspace   string
	ProjectWorkflow    string
	Layer              uint
	Terragrunt         bool
	OpenTofu           bool
	Pulumi             bool
	Commands           []string
	ApplyStage         *Stage
	PlanStage          *Stage
	PullRequestNumber  *int
	EventName          string
	RequestedBy        string
	Namespace          string
	RunEnvVars         map[string]string
	StateEnvVars       map[string]string
	CommandEnvVars     map[string]string
	StateEnvProvider   *stscreds.WebIdentityRoleProvider
	StateRoleArn       string
	CommandEnvProvider *stscreds.WebIdentityRoleProvider
	CommandRoleArn     string
	CognitoOidcConfig  *configuration.AwsCognitoOidcConfig
	SkipMergeCheck     bool
}

type Step struct {
	Action    string
	Value     string
	ExtraArgs []string
	Shell     string
}

type Stage struct {
	FilterRegex *string
	Steps       []Step
}

func ToConfigStep(configState configuration.Step) Step {
	return Step{
		Action:    configState.Action,
		Value:     configState.Value,
		ExtraArgs: configState.ExtraArgs,
		Shell:     configState.Shell,
	}

}

func ToConfigStage(configStage *configuration.Stage) *Stage {
	if configStage == nil {
		return nil
	}
	steps := make([]Step, 0)
	for _, step := range configStage.Steps {
		steps = append(steps, ToConfigStep(step))
	}
	return &Stage{
		Steps:       steps,
		FilterRegex: configStage.FilterRegex,
	}
}

func (j *Job) GetProjectAlias() string {
	if j.ProjectAlias != "" {
		return j.ProjectAlias
	}
	return j.ProjectName
}

func JobForProjectName(jobs []Job, projectName string) (*Job, error) {
	filteredJobs := lo.Filter(jobs, func(item Job, index int) bool {
		return item.ProjectName == projectName
	})
	if len(filteredJobs) == 0 {
		return nil, fmt.Errorf("job not found for project name %v", projectName)
	}
	if len(filteredJobs) > 1 {
		return nil, fmt.Errorf("more than one job found for project name, duplicate? %v", projectName)
	}
	return &filteredJobs[0], nil
}

func (j *Job) IsPlan() bool {
	return slices.Contains(j.Commands, "digger plan")
}

func (j *Job) IsApply() bool {
	return slices.Contains(j.Commands, "digger apply")
}

func (j *Job) IacType() IacType {
	if j.Pulumi {
		return IacTypePulumi
	} else {
		return IacTypeTerraform
	}
}

func IsPlanJobs(jobs []Job) bool {
	isPlan := true
	for _, job := range jobs {
		isPlan = isPlan && job.IsPlan()
	}
	return isPlan
}

func IsApplyJobs(jobs []JobJson) bool {
	isApply := true
	for _, job := range jobs {
		isApply = isApply && job.IsApply()
	}
	return isApply
}

func CountUniqueLayers(jobs []Job) (uint, []uint) {
	layerOnly := lo.Map(jobs, func(job Job, _ int) uint {
		return job.Layer
	})

	uniqueLayers := lo.Uniq(layerOnly)

	return uint(len(uniqueLayers)), uniqueLayers
}
