package bitbucket

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
)

type BitbucketContext struct {
	PullRequestID      *int               `env:"BITBUCKET_PR_ID"`
	RepositoryOWNER    string             `env:"BITBUCKET_REPO_OWNER"`
	RepositoryName     string             `env:"BITBUCKET_REPO_NAME"`
	RepositoryFullName string             `env:"BITBUCKET_REPO_FULL_NAME"`
	Token              string             `env:"BITBUCKET_TOKEN"`
	BitbucketUserName  string             `env:"BITBUCKET_USER_NAME"`
	DiggerCommand      string             `env:"DIGGER_COMMAND"`
	EventType          BitbucketEventType `env:"BITBUCKET_EVENT_TYPE"`
}

type BitbucketEventType string

func (e BitbucketEventType) String() string {
	return string(e)
}

const (
	PullRequestOpened   = BitbucketEventType("pullrequest:created")
	PullRequestUpdated  = BitbucketEventType("pullrequest:updated")
	PullRequestMerged   = BitbucketEventType("pullrequest:merged")
	PullRequestDeclined = BitbucketEventType("pullrequest:declined")
	PullRequestComment  = BitbucketEventType("pullrequest:comment")
)

func ParseBitbucketContext() (*BitbucketContext, error) {
	var parsedContext BitbucketContext

	if err := env.Parse(&parsedContext); err != nil {
		slog.Error("error parsing Bitbucket context", "error", err)
		return nil, err
	}

	slog.Info("parsed Bitbucket context",
		"repoOwner", parsedContext.RepositoryOWNER,
		"repoName", parsedContext.RepositoryName,
		"eventType", parsedContext.EventType,
		"prId", parsedContext.PullRequestID)
	return &parsedContext, nil
}

func ProcessBitbucketEvent(context *BitbucketContext, diggerConfig *digger_config.DiggerConfig, api BitbucketAPI) ([]digger_config.Project, *digger_config.Project, error) {
	var impactedProjects []digger_config.Project

	if context.PullRequestID == nil {
		slog.Error("pull request ID not found")
		return nil, nil, fmt.Errorf("pull request ID not found")
	}

	slog.Info("processing Bitbucket event",
		"eventType", context.EventType,
		"prId", *context.PullRequestID)

	changedFiles, err := api.GetChangedFiles(*context.PullRequestID)
	if err != nil {
		slog.Error("could not get changed files", "error", err, "prId", *context.PullRequestID)
		return nil, nil, fmt.Errorf("could not get changed files: %v", err)
	}

	impactedProjects, _ = diggerConfig.GetModifiedProjects(changedFiles)
	slog.Info("identified impacted projects", "count", len(impactedProjects))

	switch context.EventType {
	case PullRequestComment:
		diggerCommand := strings.ToLower(context.DiggerCommand)
		diggerCommand = strings.TrimSpace(diggerCommand)
		requestedProject := ci.ParseProjectName(diggerCommand)

		slog.Debug("processing PR comment",
			"command", diggerCommand,
			"requestedProject", requestedProject)

		if requestedProject == "" {
			return impactedProjects, nil, nil
		}

		for _, project := range impactedProjects {
			if project.Name == requestedProject {
				slog.Debug("found requested project in impacted projects", "project", requestedProject)
				return impactedProjects, &project, nil
			}
		}
		slog.Error("requested project not found in modified projects", "requestedProject", requestedProject)
		return nil, nil, fmt.Errorf("requested project not found in modified projects")
	default:
		return impactedProjects, nil, nil
	}
}

func ConvertBitbucketEventToCommands(event BitbucketEventType, context *BitbucketContext, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow) ([]scheduler.Job, bool, error) {
	jobs := make([]scheduler.Job, 0)

	slog.Info("converting Bitbucket event to commands",
		"eventType", event,
		"impactedProjects", len(impactedProjects))

	switch event {
	case PullRequestOpened, PullRequestUpdated:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				slog.Error("failed to find workflow config",
					"workflow", project.Workflow,
					"project", project.Name)
				return nil, true, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
			}

			var skipMerge bool
			if workflow.Configuration != nil {
				skipMerge = workflow.Configuration.SkipMergeCheck
			}

			stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, true)
			StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(project)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Pulumi:             project.Pulumi,
				Commands:           workflow.Configuration.OnPullRequestPushed,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				PullRequestNumber:  context.PullRequestID,
				EventName:          string(event),
				RequestedBy:        context.BitbucketUserName,
				Namespace:          context.RepositoryFullName,
				StateEnvVars:       stateEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvProvider:   StateEnvProvider,
				CommandEnvProvider: CommandEnvProvider,
				SkipMergeCheck:     skipMerge,
			})

			slog.Debug("created job for PR",
				"project", project.Name,
				"prId", *context.PullRequestID,
				"eventType", event)
		}
		return jobs, true, nil

	case PullRequestComment:
		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}
		coversAllImpactedProjects := true
		runForProjects := impactedProjects

		if requestedProject != nil {
			if len(impactedProjects) > 1 {
				slog.Info("comment specifies one project out of many impacted",
					"requestedProject", requestedProject.Name,
					"impactedProjectCount", len(impactedProjects))

				coversAllImpactedProjects = false
				runForProjects = []digger_config.Project{*requestedProject}
			} else if len(impactedProjects) == 1 && impactedProjects[0].Name != requestedProject.Name {
				slog.Error("requested project not impacted by PR",
					"requestedProject", requestedProject.Name,
					"impactedProject", impactedProjects[0].Name)
				return jobs, false, fmt.Errorf("requested project %v is not impacted by this PR", requestedProject.Name)
			}
		}

		diggerCommand := strings.ToLower(context.DiggerCommand)
		diggerCommand = strings.TrimSpace(diggerCommand)

		slog.Debug("processing digger command", "command", diggerCommand)

		for _, command := range supportedCommands {
			if strings.Contains(diggerCommand, command) {
				slog.Info("matched command", "command", command)

				for _, project := range runForProjects {
					workflow, ok := workflows[project.Workflow]
					if !ok {
						slog.Debug("workflow not found, using default",
							"requestedWorkflow", project.Workflow,
							"project", project.Name)
						workflow = workflows["default"]
					}

					workspace := project.Workspace
					workspaceOverride, err := ci.ParseWorkspace(diggerCommand)
					if err != nil {
						slog.Error("failed to parse workspace", "error", err, "command", diggerCommand)
						return []scheduler.Job{}, false, err
					}
					if workspaceOverride != "" {
						slog.Debug("using workspace override",
							"original", workspace,
							"override", workspaceOverride)
						workspace = workspaceOverride
					}

					stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, true)
					StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(project)

					jobs = append(jobs, scheduler.Job{
						ProjectName:        project.Name,
						ProjectDir:         project.Dir,
						ProjectWorkspace:   workspace,
						Terragrunt:         project.Terragrunt,
						OpenTofu:           project.OpenTofu,
						Pulumi:             project.Pulumi,
						Commands:           []string{command},
						ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
						PlanStage:          scheduler.ToConfigStage(workflow.Plan),
						PullRequestNumber:  context.PullRequestID,
						EventName:          string(event),
						RequestedBy:        context.BitbucketUserName,
						Namespace:          context.RepositoryFullName,
						StateEnvVars:       stateEnvVars,
						CommandEnvVars:     commandEnvVars,
						StateEnvProvider:   StateEnvProvider,
						CommandEnvProvider: CommandEnvProvider,
					})

					slog.Debug("created job from comment",
						"project", project.Name,
						"command", command,
						"workspace", workspace)
				}
			}
		}
		return jobs, coversAllImpactedProjects, nil

	default:
		slog.Error("unsupported Bitbucket event type", "eventType", event)
		return []scheduler.Job{}, false, fmt.Errorf("unsupported Bitbucket event type: %v", event)
	}
}
