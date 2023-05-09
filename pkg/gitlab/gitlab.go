package gitlab

import (
	"digger/pkg/configuration"
	"digger/pkg/digger"
	"fmt"
	"github.com/caarlos0/env/v7"
	go_gitlab "github.com/xanzy/go-gitlab"
	"log"
	"strings"
)

// based on https://docs.gitlab.com/ee/ci/variables/predefined_variables.html

type GitLabContext struct {
	PipelineSource PipelineSourceType `env:"CI_PIPELINE_SOURCE"`

	// this env var should be set by webhook that trigger pipeline
	EventType          GitLabEventType `env:"MERGE_REQUEST_EVENT_NAME"`
	PipelineId         *int            `env:"CI_PIPELINE_ID"`
	PipelineIId        *int            `env:"CI_PIPELINE_IID"`
	MergeRequestId     *int            `env:"CI_MERGE_REQUEST_ID"`
	MergeRequestIId    *int            `env:"CI_MERGE_REQUEST_IID"`
	ProjectName        string          `env:"CI_PROJECT_NAME"`
	ProjectNamespace   string          `env:"CI_PROJECT_NAMESPACE"`
	ProjectId          *int            `env:"CI_PROJECT_ID"`
	ProjectNamespaceId *int            `env:"CI_PROJECT_NAMESPACE_ID"`
	Token              string          `env:"GITLAB_TOKEN"`
	DiggerCommand      string          `env:"DIGGER_COMMAND"`
}

type PipelineSourceType string

func (t PipelineSourceType) String() string {
	return string(t)
}

const (
	Push                     = PipelineSourceType("push")
	Web                      = PipelineSourceType("web")
	Schedule                 = PipelineSourceType("schedule")
	Api                      = PipelineSourceType("api")
	External                 = PipelineSourceType("external")
	Chat                     = PipelineSourceType("chat")
	WebIDE                   = PipelineSourceType("webide")
	ExternalPullRequestEvent = PipelineSourceType("external_pull_request_event")
	ParentPipeline           = PipelineSourceType("parent_pipeline")
	Trigger                  = PipelineSourceType("trigger")
	Pipeline                 = PipelineSourceType("pipeline")
)

func ParseGitLabContext() (*GitLabContext, error) {
	var parsedGitLabContext GitLabContext

	if err := env.Parse(&parsedGitLabContext); err != nil {
		fmt.Printf("%+v\n", err)
	}

	fmt.Printf("%+v\n", parsedGitLabContext)
	return &parsedGitLabContext, nil
}

func NewGitLabService(token string, gitLabContext *GitLabContext) (*GitLabService, error) {
	client, err := go_gitlab.NewClient(token)
	if err != nil {
		log.Fatalf("failed to create gitlab client: %v", err)
	}

	user, _, err := client.Users.CurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get current GitLab user info, %v", err)
	}
	fmt.Printf("current GitLab user: %s\n", user.Name)

	return &GitLabService{
		Client:  client,
		Context: gitLabContext,
	}, nil
}

func ProcessGitLabEvent(gitlabContext *GitLabContext, diggerConfig *configuration.DiggerConfig, service *GitLabService) ([]configuration.Project, error) {
	var impactedProjects []configuration.Project

	if gitlabContext.MergeRequestIId == nil {
		println("Merge Request ID is not found.")
		return nil, nil
	}

	mergeRequestId := gitlabContext.MergeRequestIId
	changedFiles, err := service.GetChangedFiles(*mergeRequestId)

	if err != nil {
		return nil, fmt.Errorf("could not get changed files")
	}

	impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)

	fmt.Println("Impacted projects:")
	for _, v := range impactedProjects {
		fmt.Printf("%s\n", v.Name)
	}

	return impactedProjects, nil
}

type GitLabService struct {
	Client  *go_gitlab.Client
	Context *GitLabContext
}

func (gitlabService GitLabService) GetChangedFiles(mergeRequestId int) ([]string, error) {
	opt := &go_gitlab.GetMergeRequestChangesOptions{}

	log.Printf("projectId: %d", *gitlabService.Context.ProjectId)
	log.Printf("mergeRequestId: %d", mergeRequestId)
	mergeRequestChanges, _, err := gitlabService.Client.MergeRequests.GetMergeRequestChanges(*gitlabService.Context.ProjectId, mergeRequestId, opt)
	if err != nil {
		log.Fatalf("error getting gitlab's merge request: %v", err)
	}

	fileNames := make([]string, len(mergeRequestChanges.Changes))

	for i, change := range mergeRequestChanges.Changes {
		fileNames[i] = change.NewPath
		//fmt.Printf("changed file: %s \n", change.NewPath)
	}
	return fileNames, nil
}

func (gitlabService GitLabService) PublishComment(mergeRequest int, comment string) {
	//TODO implement me
	//panic("implement me")
}

type CIService interface {
	GetChangedFiles(prNumber int) ([]string, error)
	PublishComment(prNumber int, comment string)
}

type GitLabEvent struct {
	EventType GitLabEventType
}

type GitLabEventType string

func (e GitLabEventType) String() string {
	return string(e)
}

const (
	MergeRequestOpened  = GitLabEventType("merge_request_opened")
	MergeRequestUpdated = GitLabEventType("merge_request_updated")
	MergeRequestClosed  = GitLabEventType("merge_request_closed")
	MergeRequestComment = GitLabEventType("merge_request_commented")
)

func ConvertGitLabEventToCommands(event GitLabEvent, gitLabContext *GitLabContext, impactedProjects []configuration.Project, workflows map[string]configuration.Workflow) ([]digger.ProjectCommand, error) {
	commandsPerProject := make([]digger.ProjectCommand, 0)

	switch event.EventType {
	case MergeRequestOpened:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				workflow = workflows["default"]
			}

			commandsPerProject = append(commandsPerProject, digger.ProjectCommand{
				ProjectName:      project.Name,
				ProjectDir:       project.Dir,
				ProjectWorkspace: project.Workspace,
				Terragrunt:       project.Terragrunt,
				Commands:         workflow.Configuration.OnPullRequestPushed,
				ApplyStage:       workflow.Apply,
				PlanStage:        workflow.Plan,
			})
		}
		return commandsPerProject, nil
	case MergeRequestUpdated:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				workflow = workflows["default"]
			}

			commandsPerProject = append(commandsPerProject, digger.ProjectCommand{
				ProjectName:      project.Name,
				ProjectDir:       project.Dir,
				ProjectWorkspace: project.Workspace,
				Terragrunt:       project.Terragrunt,
				Commands:         workflow.Configuration.OnPullRequestPushed,
				ApplyStage:       workflow.Apply,
				PlanStage:        workflow.Plan,
			})
		}
		return commandsPerProject, nil
	case MergeRequestClosed:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				workflow = workflows["default"]
			}
			commandsPerProject = append(commandsPerProject, digger.ProjectCommand{
				ProjectName:      project.Name,
				ProjectDir:       project.Dir,
				ProjectWorkspace: project.Workspace,
				Terragrunt:       project.Terragrunt,
				Commands:         workflow.Configuration.OnPullRequestClosed,
				ApplyStage:       workflow.Apply,
				PlanStage:        workflow.Plan,
			})
		}
		return commandsPerProject, nil
	case MergeRequestComment:
		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}

		for _, command := range supportedCommands {
			if strings.Contains(gitLabContext.DiggerCommand, command) {
				for _, project := range impactedProjects {
					workspace := project.Workspace
					//workspaceOverride, err := parseWorkspace(gitLabContext.DiggerCommand)
					//if err != nil {
					//	return []digger.ProjectCommand{}, err
					//}
					//if workspaceOverride != "" {
					//	workspace = workspaceOverride
					//}
					commandsPerProject = append(commandsPerProject, digger.ProjectCommand{
						ProjectName:      project.Name,
						ProjectDir:       project.Dir,
						ProjectWorkspace: workspace,
						Terragrunt:       project.Terragrunt,
						Commands:         []string{command},
					})
				}
			}
		}
		return commandsPerProject, nil

	default:
		return []digger.ProjectCommand{}, fmt.Errorf("unsupported GitLab event type: %v", event)
	}
}

/*
func RunCommandsPerProject(commandsPerProject []digger.ProjectCommand, gitLabContext GitLabContext, diggerConfig *configuration.DiggerConfig, service CIService, lock utils.Lock, workingDir string) error {

	lockAcquisitionSuccess := true
	for _, projectCommands := range commandsPerProject {
		for _, command := range projectCommands.Commands {
			projectLock := &utils.ProjectLockImpl{
				InternalLock: lock,
				PrManager:    service,
				ProjectName:  projectCommands.ProjectName,
				RepoName:     gitLabContext.ProjectName,
				RepoOwner:    gitLabContext.ProjectNamespace,
			}
			diggerExecutor := digger.DiggerExecutor{
				workingDir,
				projectCommands.ProjectWorkspace,
				gitLabContext.ProjectNamespace,
				projectCommands.ProjectName,
				projectCommands.ProjectDir,
				gitLabContext.ProjectName,
				projectCommands.Terragrunt,
				service,
				projectLock,
				diggerConfig,
			}
			switch command {
			case "digger plan":
				utils.SendUsageRecord(gitLabContext.ProjectNamespace, gitLabContext.EventType.String(), "plan")
				diggerExecutor.Plan(*gitLabContext.MergeRequestIId)
			case "digger apply":
				utils.SendUsageRecord(gitLabContext.ProjectName, gitLabContext.EventType.String(), "apply")
				diggerExecutor.Apply(*gitLabContext.MergeRequestIId)
			case "digger unlock":
				utils.SendUsageRecord(gitLabContext.ProjectNamespace, gitLabContext.EventType.String(), "unlock")
				diggerExecutor.Unlock(*gitLabContext.MergeRequestIId)
			case "digger lock":
				utils.SendUsageRecord(gitLabContext.ProjectNamespace, gitLabContext.EventType.String(), "lock")
				lockAcquisitionSuccess = diggerExecutor.Lock(*gitLabContext.MergeRequestIId)
			}
		}
	}

	if !lockAcquisitionSuccess {
		os.Exit(1)
	}
	return nil
}
*/
