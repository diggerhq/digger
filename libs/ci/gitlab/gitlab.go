package gitlab

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"

	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/caarlos0/env/v11"
	"github.com/diggerhq/digger/libs/digger_config"
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
		slog.Error("failed to parse GitLab context", "error", err)
	}

	slog.Info("parsed GitLab context",
		"pipelineSource", parsedGitLabContext.PipelineSource,
		"eventType", parsedGitLabContext.EventType,
		"projectName", parsedGitLabContext.ProjectName,
		"projectNamespace", parsedGitLabContext.ProjectNamespace)
	return &parsedGitLabContext, nil
}

func NewGitLabService(token string, gitLabContext *GitLabContext, gitlabBaseUrl string) (*GitLabService, error) {
	var client *go_gitlab.Client
	var err error
	if gitlabBaseUrl != "" {
		client, err = go_gitlab.NewClient(token, go_gitlab.WithBaseURL(gitlabBaseUrl))
	} else {
		client, err = go_gitlab.NewClient(token)
	}

	if err != nil {
		slog.Error("failed to create gitlab client", "error", err)
		return nil, fmt.Errorf("failed to create gitlab client: %v", err)
	}

	user, _, err := client.Users.CurrentUser()
	if err != nil {
		slog.Error("failed to get current GitLab user info", "error", err)
		return nil, fmt.Errorf("failed to get current GitLab user info, %v", err)
	}
	slog.Info("current GitLab user", "name", user.Name)

	return &GitLabService{
		Client:  client,
		Context: gitLabContext,
	}, nil
}

func ProcessGitLabEvent(gitlabContext *GitLabContext, diggerConfig *digger_config.DiggerConfig, service *GitLabService) ([]digger_config.Project, *digger_config.Project, error) {
	var impactedProjects []digger_config.Project

	if gitlabContext.MergeRequestIId == nil {
		slog.Error("merge request ID not found")
		return nil, nil, fmt.Errorf("value for 'Merge Request ID' parameter is not found")
	}

	mergeRequestId := gitlabContext.MergeRequestIId
	slog.Info("processing GitLab event",
		"eventType", gitlabContext.EventType,
		"mergeRequestId", *mergeRequestId)

	changedFiles, err := service.GetChangedFiles(*mergeRequestId)
	if err != nil {
		slog.Error("could not get changed files", "error", err, "mergeRequestId", *mergeRequestId)
		return nil, nil, fmt.Errorf("could not get changed files")
	}

	impactedProjects, _ = diggerConfig.GetModifiedProjects(changedFiles)
	slog.Info("identified impacted projects", "count", len(impactedProjects))

	switch gitlabContext.EventType {
	case MergeRequestComment:
		diggerCommand := strings.ToLower(gitlabContext.DiggerCommand)
		diggerCommand = strings.TrimSpace(diggerCommand)
		requestedProject := ci.ParseProjectName(diggerCommand)

		slog.Debug("processing merge request comment",
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

type GitLabService struct {
	Client  *go_gitlab.Client
	Context *GitLabContext
}

func (gitlabService GitLabService) GetChangedFiles(mergeRequestId int) ([]string, error) {
	opt := &go_gitlab.ListMergeRequestDiffsOptions{}

	slog.Debug("getting changed files",
		"projectId", *gitlabService.Context.ProjectId,
		"mergeRequestId", mergeRequestId)

	mergeRequestChanges, _, err := gitlabService.Client.MergeRequests.ListMergeRequestDiffs(*gitlabService.Context.ProjectId, mergeRequestId, opt)
	if err != nil {
		slog.Error("error getting GitLab merge request diffs", "error", err, "mergeRequestId", mergeRequestId)
		return nil, fmt.Errorf("error getting gitlab's merge request: %v", err)
	}

	fileNames := make([]string, len(mergeRequestChanges))

	for i, change := range mergeRequestChanges {
		fileNames[i] = change.NewPath
	}

	slog.Debug("found changed files", "count", len(fileNames), "mergeRequestId", mergeRequestId)
	return fileNames, nil
}

func (gitlabService GitLabService) GetUserTeams(organisation, user string) ([]string, error) {
	return make([]string, 0), nil
}

func (gitlabService GitLabService) PublishComment(prNumber int, comment string) (*ci.Comment, error) {
	discussionId := gitlabService.Context.DiscussionID
	projectId := *gitlabService.Context.ProjectId
	mergeRequestIID := *gitlabService.Context.MergeRequestIId
	commentOpt := &go_gitlab.AddMergeRequestDiscussionNoteOptions{Body: &comment}

	slog.Info("publishing comment",
		"mergeRequestIID", mergeRequestIID,
		"projectId", projectId,
		"discussionId", discussionId)

	if discussionId == "" {
		commentOpt := &go_gitlab.CreateMergeRequestDiscussionOptions{Body: &comment}
		discussion, _, err := gitlabService.Client.Discussions.CreateMergeRequestDiscussion(projectId, mergeRequestIID, commentOpt)
		if err != nil {
			slog.Error("failed to publish comment", "error", err, "mergeRequestIID", mergeRequestIID)
			print(err.Error())
		}
		discussionId = discussion.ID
		note := discussion.Notes[0]
		return &ci.Comment{Id: strconv.Itoa(note.ID), DiscussionId: discussionId, Body: &note.Body}, err
	} else {
		note, _, err := gitlabService.Client.Discussions.AddMergeRequestDiscussionNote(projectId, mergeRequestIID, discussionId, commentOpt)
		if err != nil {
			slog.Error("failed to publish comment", "error", err, "mergeRequestIID", mergeRequestIID, "discussionId", discussionId)
			print(err.Error())
		}
		return &ci.Comment{Id: strconv.Itoa(note.ID), DiscussionId: discussionId, Body: &note.Body}, err
	}
}

func (svc GitLabService) ListIssues() ([]*ci.Issue, error) {
	return nil, fmt.Errorf("implement me")
}

func (svc GitLabService) PublishIssue(title, body string, labels *[]string) (int64, error) {
	return 0, fmt.Errorf("implement me")
}

func (svc GitLabService) UpdateIssue(id int64, title, body string) (int64, error) {
	return 0, fmt.Errorf("implement me")
}

// SetStatus GitLab implementation is using https://docs.gitlab.com/15.11/ee/api/status_checks.html (external status checks)
// https://docs.gitlab.com/ee/user/project/merge_requests/status_checks.html#add-a-status-check-service
// only supported by 'Ultimate' plan
func (gitlabService GitLabService) SetStatus(mergeRequestID int, status, statusContext string) error {
	// TODO implement me
	slog.Debug("setting status (not implemented)",
		"mergeRequestID", mergeRequestID,
		"status", status,
		"statusContext", statusContext)
	return nil
}

func (gitlabService GitLabService) GetCombinedPullRequestStatus(mergeRequestID int) (string, error) {
	// TODO implement me
	return "success", nil
}

func (gitlabService GitLabService) MergePullRequest(mergeRequestID int, mergeStrategy string) error {
	projectId := *gitlabService.Context.ProjectId
	mergeRequestIID := *gitlabService.Context.MergeRequestIId
	mergeWhenPipelineSucceeds := true
	opt := &go_gitlab.AcceptMergeRequestOptions{MergeWhenPipelineSucceeds: &mergeWhenPipelineSucceeds}

	slog.Info("merging pull request",
		"mergeRequestID", mergeRequestID,
		"projectId", projectId,
		"mergeRequestIID", mergeRequestIID,
		"strategy", mergeStrategy)

	_, _, err := gitlabService.Client.MergeRequests.AcceptMergeRequest(projectId, mergeRequestIID, opt)
	if err != nil {
		slog.Error("failed to merge merge request", "error", err, "mergeRequestIID", mergeRequestIID)
		return fmt.Errorf("Failed to merge Merge Request. %v\n", err)
	}
	return nil
}

func (gitlabService GitLabService) IsMergeable(mergeRequestID int) (bool, error) {
	opt := &go_gitlab.GetMergeRequestsOptions{}
	mergeRequest, _, err := gitlabService.Client.MergeRequests.GetMergeRequest(*gitlabService.Context.ProjectId, *gitlabService.Context.MergeRequestIId, opt)
	if err != nil {
		slog.Error("could not get GitLab mergeability status", "error", err, "mergeRequestID", mergeRequestID)
		return false, fmt.Errorf("could not get gitlab mergability status: %v", err)
	}
	if mergeRequest.DetailedMergeStatus == "mergeable" {
		return true, nil
	}
	slog.Debug("merge request not mergeable",
		"mergeRequestID", mergeRequestID,
		"detailedMergeStatus", mergeRequest.DetailedMergeStatus)
	return false, nil
}

func (gitlabService GitLabService) IsClosed(mergeRequestIID int) (bool, error) {
	mergeRequest := getMergeRequest(gitlabService, mergeRequestIID)
	slog.Debug("checking if merge request is closed",
		"mergeRequestIID", mergeRequestIID,
		"state", mergeRequest.State)

	if mergeRequest.State == "closed" || mergeRequest.State == "merged" {
		return true, nil
	}
	return false, nil
}

func (gitlabService GitLabService) IsMerged(mergeRequestIID int) (bool, error) {
	mergeRequest := getMergeRequest(gitlabService, mergeRequestIID)
	slog.Debug("checking if merge request is merged",
		"mergeRequestIID", mergeRequestIID,
		"state", mergeRequest.State)

	if mergeRequest.State == "merged" {
		return true, nil
	}
	return false, nil
}

func (gitlabService GitLabService) EditComment(prNumber int, id, comment string) error {
	discussionId := gitlabService.Context.DiscussionID
	projectId := *gitlabService.Context.ProjectId
	mergeRequestIID := *gitlabService.Context.MergeRequestIId
	commentOpt := &go_gitlab.UpdateMergeRequestDiscussionNoteOptions{Body: &comment}

	slog.Info("editing comment",
		"mergeRequestIID", mergeRequestIID,
		"projectId", projectId,
		"discussionId", discussionId,
		"commentId", id)

	id32, err := strconv.Atoi(id)
	if err != nil {
		slog.Error("could not convert comment ID to int", "error", err, "id", id)
		return fmt.Errorf("could not convert to int: %v", err)
	}
	_, _, err = gitlabService.Client.Discussions.UpdateMergeRequestDiscussionNote(projectId, mergeRequestIID, discussionId, id32, commentOpt)
	if err != nil {
		slog.Error("failed to edit comment", "error", err, "mergeRequestIID", mergeRequestIID, "commentId", id)
		print(err.Error())
	}

	return err
}

func (gitlabService GitLabService) DeleteComment(id string) error {
	return nil
}

func (gitlabService GitLabService) CreateCommentReaction(id, reaction string) error {
	// TODO implement me
	return nil
}

func (gitlabService GitLabService) GetComments(prNumber int) ([]ci.Comment, error) {
	// TODO implement me
	return nil, nil
}

func (gitlabService GitLabService) GetApprovals(prNumber int) ([]string, error) {
	approvals := make([]string, 0)
	// TODO: implement me
	return approvals, nil
}

func (gitlabService GitLabService) GetBranchName(prNumber int) (string, string, error) {
	// TODO implement me
	projectId := *gitlabService.Context.ProjectId
	slog.Debug("getting branch name", "prNumber", prNumber, "projectId", projectId)

	options := go_gitlab.GetMergeRequestsOptions{}
	pr, _, err := gitlabService.Client.MergeRequests.GetMergeRequest(projectId, prNumber, &options)
	if err != nil {
		slog.Error("error getting branch name for PR", "error", err, "prNumber", prNumber)
		return "", "", err
	}
	return pr.SourceBranch, pr.SHA, nil
}

func (gitlabService GitLabService) CheckBranchExists(branchName string) (bool, error) {
	projectId := *gitlabService.Context.ProjectId
	slog.Debug("checking if branch exists", "branchName", branchName, "projectId", projectId)

	_, resp, err := gitlabService.Client.Branches.GetBranch(projectId, branchName)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		slog.Error("error checking if branch exists", "error", err, "branchName", branchName)
		return false, err
	}
	return true, nil
}

func (svc GitLabService) SetOutput(prNumber int, key, value string) error {
	// TODO implement me
	return nil
}

func getMergeRequest(gitlabService GitLabService, mergeRequestIID int) *go_gitlab.MergeRequest {
	projectId := *gitlabService.Context.ProjectId
	slog.Debug("getting merge request", "mergeRequestIID", mergeRequestIID, "projectId", projectId)

	opt := &go_gitlab.GetMergeRequestsOptions{}
	mergeRequest, _, err := gitlabService.Client.MergeRequests.GetMergeRequest(projectId, mergeRequestIID, opt)
	if err != nil {
		slog.Error("failed to get merge request", "error", err, "mergeRequestIID", mergeRequestIID)
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

func ConvertGitLabEventToCommands(event GitLabEvent, gitLabContext *GitLabContext, impactedProjects []digger_config.Project, requestedProject *digger_config.Project, workflows map[string]digger_config.Workflow) ([]scheduler.Job, bool, error) {
	jobs := make([]scheduler.Job, 0)

	slog.Info("converting GitLab event to commands", "eventType", event.EventType)

	switch event.EventType {
	case MergeRequestOpened, MergeRequestReopened, MergeRequestUpdated:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				slog.Error("failed to find workflow config",
					"workflow", project.Workflow,
					"project", project.Name)
				return nil, true, fmt.Errorf("failed to find workflow digger_config '%s' for project '%s'", project.Workflow, project.Name)
			}

			var skipMerge bool
			if workflow.Configuration != nil {
				skipMerge = workflow.Configuration.SkipMergeCheck
			} else {
				skipMerge = false
			}

			stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, true)
			StateEnvProvider, CommandEnvProvider := scheduler.GetStateAndCommandProviders(project)

			slog.Debug("creating job for MR update event",
				"project", project.Name,
				"eventType", event.EventType)

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
				PullRequestNumber:  gitLabContext.MergeRequestIId,
				EventName:          gitLabContext.EventType.String(),
				RequestedBy:        gitLabContext.GitlabUserName,
				Namespace:          gitLabContext.ProjectNamespace,
				StateEnvVars:       stateEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvProvider:   StateEnvProvider,
				CommandEnvProvider: CommandEnvProvider,
				SkipMergeCheck:     skipMerge,
			})
		}
		return jobs, true, nil

	case MergeRequestClosed, MergeRequestMerged:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				slog.Error("failed to find workflow config",
					"workflow", project.Workflow,
					"project", project.Name)
				return nil, true, fmt.Errorf("failed to find workflow digger_config '%s' for project '%s'", project.Workflow, project.Name)
			}
			stateEnvVars, commandEnvVars := digger_config.CollectTerraformEnvConfig(workflow.EnvVars, true)
			var StateEnvProvider *stscreds.WebIdentityRoleProvider
			var CommandEnvProvider *stscreds.WebIdentityRoleProvider
			if project.AwsRoleToAssume != nil {
				if project.AwsRoleToAssume.Command != "" {
					StateEnvProvider = scheduler.GetProviderFromRole(project.AwsRoleToAssume.State, project.AwsRoleToAssume.AwsRoleRegion)
				} else {
					StateEnvProvider = nil
				}

				if project.AwsRoleToAssume.Command != "" {
					CommandEnvProvider = scheduler.GetProviderFromRole(project.AwsRoleToAssume.Command, project.AwsRoleToAssume.AwsRoleRegion)
				} else {
					CommandEnvProvider = nil
				}
			}

			slog.Debug("creating job for MR close/merge event",
				"project", project.Name,
				"eventType", event.EventType)

			jobs = append(jobs, scheduler.Job{
				ProjectName:        project.Name,
				ProjectDir:         project.Dir,
				ProjectWorkspace:   project.Workspace,
				Terragrunt:         project.Terragrunt,
				OpenTofu:           project.OpenTofu,
				Pulumi:             project.Pulumi,
				Commands:           workflow.Configuration.OnPullRequestClosed,
				ApplyStage:         scheduler.ToConfigStage(workflow.Apply),
				PlanStage:          scheduler.ToConfigStage(workflow.Plan),
				PullRequestNumber:  gitLabContext.MergeRequestIId,
				EventName:          gitLabContext.EventType.String(),
				RequestedBy:        gitLabContext.GitlabUserName,
				Namespace:          gitLabContext.ProjectNamespace,
				StateEnvVars:       stateEnvVars,
				CommandEnvVars:     commandEnvVars,
				StateEnvProvider:   StateEnvProvider,
				CommandEnvProvider: CommandEnvProvider,
			})
		}
		return jobs, true, nil

	case MergeRequestComment:
		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}
		coversAllImpactedProjects := true
		runForProjects := impactedProjects

		if requestedProject != nil {
			if len(impactedProjects) > 1 {
				slog.Info("comment specifies one project out of many impacted",
					"requestedProject", requestedProject.Name,
					"impactedCount", len(impactedProjects))

				coversAllImpactedProjects = false
				runForProjects = []digger_config.Project{*requestedProject}
			} else if len(impactedProjects) == 1 && impactedProjects[0].Name != requestedProject.Name {
				slog.Error("requested project not impacted by MR",
					"requestedProject", requestedProject.Name,
					"impactedProject", impactedProjects[0].Name)
				return jobs, false, fmt.Errorf("requested project %v is not impacted by this PR", requestedProject.Name)
			}
		}

		diggerCommand := strings.ToLower(gitLabContext.DiggerCommand)
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

					slog.Debug("creating job from comment",
						"project", project.Name,
						"command", command,
						"workspace", workspace)

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
						PullRequestNumber:  gitLabContext.MergeRequestIId,
						EventName:          gitLabContext.EventType.String(),
						RequestedBy:        gitLabContext.GitlabUserName,
						Namespace:          gitLabContext.ProjectNamespace,
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
		slog.Error("unsupported GitLab event type", "eventType", event.EventType)
		return []scheduler.Job{}, false, fmt.Errorf("unsupported GitLab event type: %v", event)
	}
}
