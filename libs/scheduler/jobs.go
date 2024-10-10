package scheduler

import (
	"slices"

	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	configuration "github.com/diggerhq/digger/libs/digger_config"
)

type Job struct {
	ProjectName        string
	ProjectDir         string
	ProjectWorkspace   string
	ProjectWorkflow    string
	Terragrunt         bool
	OpenTofu           bool
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
	StateRoleArn	   string
	CommandEnvProvider *stscreds.WebIdentityRoleProvider
	CommandRoleArn	   string
	CognitoOidcConfig  *configuration.AwsCognitoOidcConfig
}

type Step struct {
	Action    string
	Value     string
	ExtraArgs []string
	Shell     string
}

type Stage struct {
	Steps []Step
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
		Steps: steps,
	}
}

func (j *Job) IsPlan() bool {
	return slices.Contains(j.Commands, "digger plan")
}

func (j *Job) IsApply() bool {
	return slices.Contains(j.Commands, "digger apply")
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
