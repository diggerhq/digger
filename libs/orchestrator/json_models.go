package orchestrator

type StepJson struct {
	Action    string   `json:"action"`
	ExtraArgs []string `json:"extraArgs"`
}

type StageJson struct {
	Steps []StepJson `json:"steps"`
}

type JobJson struct {
	ProjectName       string            `json:"projectName"`
	ProjectDir        string            `json:"projectDir"`
	ProjectWorkspace  string            `json:"projectWorkspace"`
	Terragrunt        bool              `json:"terragrunt"`
	Commands          []string          `json:"commands"`
	ApplyStage        StageJson         `json:"applyStage"`
	PlanStage         StageJson         `json:"planStage"`
	PullRequestNumber *int              `json:"pullRequestNumber"`
	EventName         string            `json:"eventName"`
	RequestedBy       string            `json:"requestedBy"`
	Namespace         string            `json:"namespace"`
	StateEnvVars      map[string]string `json:"stateEnvVars"`
	CommandEnvVars    map[string]string `json:"commandEnvVars"`
}

func JobToJson(job Job) JobJson {
	return JobJson{
		ProjectName:       job.ProjectName,
		ProjectDir:        job.ProjectDir,
		ProjectWorkspace:  job.ProjectWorkspace,
		Terragrunt:        job.Terragrunt,
		Commands:          job.Commands,
		ApplyStage:        stageToJson(job.ApplyStage),
		PlanStage:         stageToJson(job.PlanStage),
		PullRequestNumber: job.PullRequestNumber,
		EventName:         job.EventName,
		RequestedBy:       job.RequestedBy,
		Namespace:         job.Namespace,
		StateEnvVars:      job.StateEnvVars,
		CommandEnvVars:    job.CommandEnvVars,
	}
}

func JsonToJob(jobJson JobJson) Job {
	return Job{
		ProjectName:       jobJson.ProjectName,
		ProjectDir:        jobJson.ProjectDir,
		ProjectWorkspace:  jobJson.ProjectWorkspace,
		Terragrunt:        jobJson.Terragrunt,
		Commands:          jobJson.Commands,
		ApplyStage:        jsonToStage(jobJson.ApplyStage),
		PlanStage:         jsonToStage(jobJson.PlanStage),
		PullRequestNumber: jobJson.PullRequestNumber,
		EventName:         jobJson.EventName,
		RequestedBy:       jobJson.RequestedBy,
		Namespace:         jobJson.Namespace,
		StateEnvVars:      jobJson.StateEnvVars,
		CommandEnvVars:    jobJson.CommandEnvVars,
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
			ExtraArgs: step.ExtraArgs,
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
			ExtraArgs: step.ExtraArgs,
		}
	}
	return StageJson{
		Steps: steps,
	}
}
