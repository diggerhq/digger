package gitlab

import (
	"digger/pkg/utils"
	"fmt"
	"log"
	"strings"

	configuration "github.com/diggerhq/lib-digger-config"
	orchestrator "github.com/diggerhq/lib-orchestrator"

	"github.com/caarlos0/env/v8"
	go_gitlab "github.com/xanzy/go-gitlab"
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
	OpenMergeRequests  []string        `env:"CI_OPEN_MERGE_REQUESTS"`
	Token              string          `env:"GITLAB_TOKEN"`
	GitlabUserName     string          `env:"GITLAB_USER_NAME"`
	DiggerCommand      string          `env:"DIGGER_COMMAND"`
	DiscussionID       string          `env:"DISCUSSION_ID"`
	IsMeargeable       bool            `env:"IS_MERGEABLE"`
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
		log.Printf("%+v\n", err)
	}

	log.Printf("%+v\n", parsedGitLabContext)
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
	log.Printf("current GitLab user: %s\n", user.Name)

	return &GitLabService{
		Client:  client,
		Context: gitLabContext,
	}, nil
}

func ProcessGitLabEvent(gitlabContext *GitLabContext, diggerConfig *configuration.DiggerConfig, service *GitLabService) ([]configuration.Project, *configuration.Project, error) {
	var impactedProjects []configuration.Project

	if gitlabContext.MergeRequestIId == nil {
		return nil, nil, fmt.Errorf("value for 'Merge Request ID' parameter is not found")
	}

	mergeRequestId := gitlabContext.MergeRequestIId
	changedFiles, err := service.GetChangedFiles(*mergeRequestId)

	if err != nil {
		return nil, nil, fmt.Errorf("could not get changed files")
	}

	impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)

	switch gitlabContext.EventType {
	case MergeRequestComment:
		diggerCommand := strings.ToLower(gitlabContext.DiggerCommand)
		diggerCommand = strings.TrimSpace(diggerCommand)
		requestedProject := utils.ParseProjectName(diggerCommand)

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
		//log.Printf("changed file: %s \n", change.NewPath)
	}
	return fileNames, nil
}

func (gitlabService GitLabService) GetUserTeams(organisation string, user string) ([]string, error) {
	return make([]string, 0), nil
}

func (gitlabService GitLabService) PublishComment(mergeRequestID int, comment string) error {
	discussionId := gitlabService.Context.DiscussionID
	projectId := *gitlabService.Context.ProjectId
	mergeRequestIID := *gitlabService.Context.MergeRequestIId
	commentOpt := &go_gitlab.AddMergeRequestDiscussionNoteOptions{Body: &comment}

	log.Printf("PublishComment mergeRequestID : %d, projectId: %d, mergeRequestIID: %d, discussionId: %s \n", mergeRequestID, projectId, mergeRequestIID, discussionId)

	if discussionId == "" {
		commentOpt := &go_gitlab.CreateMergeRequestDiscussionOptions{Body: &comment}
		discussion, _, err := gitlabService.Client.Discussions.CreateMergeRequestDiscussion(projectId, mergeRequestIID, commentOpt)
		if err != nil {
			log.Printf("Failed to publish a comment. %v\n", err)
			print(err.Error())
		}
		discussionId = discussion.ID
		return err
	} else {
		_, _, err := gitlabService.Client.Discussions.AddMergeRequestDiscussionNote(projectId, mergeRequestIID, discussionId, commentOpt)
		if err != nil {
			log.Printf("Failed to publish a comment. %v\n", err)
			print(err.Error())
		}
		return err
	}
}

// SetStatus GitLab implementation is using https://docs.gitlab.com/15.11/ee/api/status_checks.html (external status checks)
// https://docs.gitlab.com/ee/user/project/merge_requests/status_checks.html#add-a-status-check-service
// only supported by 'Ultimate' plan
func (gitlabService GitLabService) SetStatus(mergeRequestID int, status string, statusContext string) error {
	//TODO implement me
	log.Printf("SetStatus: mergeRequest: %d, status: %s, statusContext: %s\n", mergeRequestID, status, statusContext)
	return nil
}

func (gitlabService GitLabService) GetCombinedPullRequestStatus(mergeRequestID int) (string, error) {
	//TODO implement me

	return "success", nil
}

func (gitlabService GitLabService) MergePullRequest(mergeRequestID int) error {
	projectId := *gitlabService.Context.ProjectId
	mergeRequestIID := *gitlabService.Context.MergeRequestIId
	mergeWhenPipelineSucceeds := true
	opt := &go_gitlab.AcceptMergeRequestOptions{MergeWhenPipelineSucceeds: &mergeWhenPipelineSucceeds}

	log.Printf("MergePullRequest mergeRequestID : %d, projectId: %d, mergeRequestIID: %d, \n", mergeRequestID, projectId, mergeRequestIID)

	_, _, err := gitlabService.Client.MergeRequests.AcceptMergeRequest(projectId, mergeRequestIID, opt)
	if err != nil {
		log.Printf("Failed to merge Merge Request. %v\n", err)
		return fmt.Errorf("Failed to merge Merge Request. %v\n", err)
	}
	return nil
}

func (gitlabService GitLabService) IsMergeable(mergeRequestID int) (bool, error) {
	return gitlabService.Context.IsMeargeable, nil
}

func (gitlabService GitLabService) IsClosed(mergeRequestID int) (bool, error) {
	mergeRequest := getMergeRequest(gitlabService)
	if mergeRequest.State == "closed" {
		return true, nil
	}
	return false, nil
}

func (gitlabService GitLabService) IsMerged(mergeRequestID int) (bool, error) {
	mergeRequest := getMergeRequest(gitlabService)
	if mergeRequest.State == "merged" {
		return true, nil
	}
	return false, nil
}

func (gitlabService GitLabService) EditComment(id interface{}, comment string) error {
	//TODO implement me
	return nil
}

func (gitlabService GitLabService) GetComments(prNumber int) ([]orchestrator.Comment, error) {
	//TODO implement me
	return nil, nil
}

func (gitlabService GitLabService) GetBranchName(prNumber int) (string, error) {
	//TODO implement me
	return "", nil
}

func getMergeRequest(gitlabService GitLabService) *go_gitlab.MergeRequest {
	projectId := *gitlabService.Context.ProjectId
	mergeRequestIID := *gitlabService.Context.MergeRequestIId
	log.Printf("getMergeRequest mergeRequestIID : %d, projectId: %d \n", mergeRequestIID, projectId)
	opt := &go_gitlab.GetMergeRequestsOptions{}
	mergeRequest, _, err := gitlabService.Client.MergeRequests.GetMergeRequest(projectId, mergeRequestIID, opt)
	if err != nil {
		log.Printf("Failed to get a MergeRequest: %d, %v \n", mergeRequestIID, err)
		print(err.Error())
	}
	return mergeRequest
}

type GitLabEvent struct {
	EventType GitLabEventType
}

type GitLabEventType string

func (e GitLabEventType) String() string {
	return string(e)
}

const (
	MergeRequestOpened     = GitLabEventType("merge_request_opened")
	MergeRequestClosed     = GitLabEventType("merge_request_closed")
	MergeRequestReopened   = GitLabEventType("merge_request_reopened")
	MergeRequestUpdated    = GitLabEventType("merge_request_updated")
	MergeRequestApproved   = GitLabEventType("merge_request_approved")
	MergeRequestUnapproved = GitLabEventType("merge_request_unapproved")
	MergeRequestApproval   = GitLabEventType("merge_request_approval")
	MergeRequestUnapproval = GitLabEventType("merge_request_unapproval")
	MergeRequestMerged     = GitLabEventType("merge_request_merge")

	MergeRequestComment = GitLabEventType("merge_request_commented")
)

func ConvertGitLabEventToCommands(event GitLabEvent, gitLabContext *GitLabContext, impactedProjects []configuration.Project, requestedProject *configuration.Project, workflows map[string]configuration.Workflow) ([]orchestrator.Job, bool, error) {
	jobs := make([]orchestrator.Job, 0)

	log.Printf("ConvertGitLabEventToCommands, event.EventType: %s\n", event.EventType)
	switch event.EventType {
	case MergeRequestOpened, MergeRequestReopened, MergeRequestUpdated:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				return nil, true, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
			}

			stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)
			jobs = append(jobs, orchestrator.Job{
				ProjectName:       project.Name,
				ProjectDir:        project.Dir,
				ProjectWorkspace:  project.Workspace,
				Terragrunt:        project.Terragrunt,
				Commands:          workflow.Configuration.OnPullRequestPushed,
				ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
				PullRequestNumber: gitLabContext.MergeRequestIId,
				EventName:         gitLabContext.EventType.String(),
				RequestedBy:       gitLabContext.GitlabUserName,
				Namespace:         gitLabContext.ProjectNamespace,
				StateEnvVars:      stateEnvVars,
				CommandEnvVars:    commandEnvVars,
			})
		}
		return jobs, true, nil
	case MergeRequestClosed, MergeRequestMerged:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				return nil, true, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
			}
			stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)
			jobs = append(jobs, orchestrator.Job{
				ProjectName:       project.Name,
				ProjectDir:        project.Dir,
				ProjectWorkspace:  project.Workspace,
				Terragrunt:        project.Terragrunt,
				Commands:          workflow.Configuration.OnPullRequestClosed,
				ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
				PullRequestNumber: gitLabContext.MergeRequestIId,
				EventName:         gitLabContext.EventType.String(),
				RequestedBy:       gitLabContext.GitlabUserName,
				Namespace:         gitLabContext.ProjectNamespace,
				StateEnvVars:      stateEnvVars,
				CommandEnvVars:    commandEnvVars,
			})
		}
		return jobs, true, nil
	case MergeRequestComment:
		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}

		coversAllImpactedProjects := true

		runForProjects := impactedProjects

		if requestedProject != nil {
			if len(impactedProjects) > 1 {
				coversAllImpactedProjects = false
				runForProjects = []configuration.Project{*requestedProject}
			} else if len(impactedProjects) == 1 && impactedProjects[0].Name != requestedProject.Name {
				return jobs, false, fmt.Errorf("requested project %v is not impacted by this PR", requestedProject.Name)
			}
		}

		diggerCommand := strings.ToLower(gitLabContext.DiggerCommand)
		diggerCommand = strings.TrimSpace(diggerCommand)
		for _, command := range supportedCommands {
			if strings.Contains(diggerCommand, command) {
				for _, project := range runForProjects {
					workflow, ok := workflows[project.Workflow]
					if !ok {
						workflow = workflows["default"]
					}
					workspace := project.Workspace
					workspaceOverride, err := utils.ParseWorkspace(diggerCommand)
					if err != nil {
						return []orchestrator.Job{}, false, err
					}
					if workspaceOverride != "" {
						workspace = workspaceOverride
					}
					stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)
					jobs = append(jobs, orchestrator.Job{
						ProjectName:       project.Name,
						ProjectDir:        project.Dir,
						ProjectWorkspace:  workspace,
						Terragrunt:        project.Terragrunt,
						Commands:          []string{command},
						ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
						PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
						PullRequestNumber: gitLabContext.MergeRequestIId,
						EventName:         gitLabContext.EventType.String(),
						RequestedBy:       gitLabContext.GitlabUserName,
						Namespace:         gitLabContext.ProjectNamespace,
						StateEnvVars:      stateEnvVars,
						CommandEnvVars:    commandEnvVars,
					})
				}
			}
		}
		return jobs, coversAllImpactedProjects, nil

	default:
		return []orchestrator.Job{}, false, fmt.Errorf("unsupported GitLab event type: %v", event)
	}
}
