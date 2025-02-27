package bitbucket

import (
	"fmt"
	"github.com/caarlos0/env/v11"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"log"
	"strings"
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
		log.Printf("Error parsing Bitbucket context: %+v\n", err)
		return nil, err
	}

	log.Printf("Parsed Bitbucket context: %+v\n", parsedContext)
	return &parsedContext, nil
}

func ProcessBitbucketEvent(context *BitbucketContext, diggerConfig *digger_config.DiggerConfig, api BitbucketAPI) ([]digger_config.Project, *digger_config.Project, error) {
	var impactedProjects []digger_config.Project

	if context.PullRequestID == nil {
		return nil, nil, fmt.Errorf("pull request ID not found")
	}

	changedFiles, err := api.GetChangedFiles(*context.PullRequestID)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get changed files: %v", err)
	}

	impactedProjects, _ = diggerConfig.GetModifiedProjects(changedFiles)

	switch context.EventType {
	case PullRequestComment:
		diggerCommand := strings.ToLower(context.DiggerCommand)
		diggerCommand = strings.TrimSpace(diggerCommand)
		requestedProject := ci.ParseProjectName(diggerCommand)

		if requestedProject == "" {
			return impactedProjects, nil, nil
		}

		for _, project := range impactedProjects {
			if project.Name == requestedProject {
				return impactedProjects, &project, nil
			}
		}
		return nil, nil, fmt.Errorf("requested project not found in modified projects")
	default:
		return impactedProjects, nil, nil
	}
}

func ConvertBitbucketEventToCommands(event BitbucketEventType, context *BitbucketContext, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow) ([]scheduler.Job, bool, error) {
	jobs := make([]scheduler.Job, 0)

	log.Printf("Converting Bitbucket event to commands, event type: %s\n", event)

	switch event {
	case PullRequestOpened, PullRequestUpdated:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
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
		}
		return jobs, true, nil

	case PullRequestComment:
		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}
		coversAllImpactedProjects := true
		runForProjects := impactedProjects

		if requestedProject != nil {
			if len(impactedProjects) > 1 {
				coversAllImpactedProjects = false
				runForProjects = []digger_config.Project{*requestedProject}
			} else if len(impactedProjects) == 1 && impactedProjects[0].Name != requestedProject.Name {
				return jobs, false, fmt.Errorf("requested project %v is not impacted by this PR", requestedProject.Name)
			}
		}

		diggerCommand := strings.ToLower(context.DiggerCommand)
		diggerCommand = strings.TrimSpace(diggerCommand)

		for _, command := range supportedCommands {
			if strings.Contains(diggerCommand, command) {
				for _, project := range runForProjects {
					workflow, ok := workflows[project.Workflow]
					if !ok {
						workflow = workflows["default"]
					}

					workspace := project.Workspace
					workspaceOverride, err := ci.ParseWorkspace(diggerCommand)
					if err != nil {
						return []scheduler.Job{}, false, err
					}
					if workspaceOverride != "" {
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
				}
			}
		}
		return jobs, coversAllImpactedProjects, nil

	default:
		return []scheduler.Job{}, false, fmt.Errorf("unsupported Bitbucket event type: %v", event)
	}
}
