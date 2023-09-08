package azure

import (
	"context"
	"digger/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	configuration "github.com/diggerhq/lib-digger-config"
	orchestrator "github.com/diggerhq/lib-orchestrator"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

const (
	AzurePrUpdated   = "git.pullrequest.updated"
	AzurePrCreated   = "git.pullrequest.created"
	AzurePrMerged    = "git.pullrequest.merged"
	AzurePrClosed    = "git.pullrequest.closed"
	AzurePrReopened  = "git.pullrequest.reopened"
	AzurePrCommented = "ms.vss-code.git-pullrequest-comment-event"
)

type AzurePrEvent struct {
	EventType          string             `json:"eventType"`
	Resource           Resource           `json:"resource"`
	ResourceContainers ResourceContainers `json:"resourceContainers"`
}

type Repository struct {
	Id      string `json:"id"`
	Name    string `json:"name"`
	Project struct {
		Name string `json:"name"`
	} `json:"project"`
	Status string `json:"status"`
}

type Resource struct {
	Repository    Repository `json:"repository"`
	PullRequestId int        `json:"pullRequestId"`
}

type ResourceContainers struct {
	Account struct {
		BaseUrl string `json:"baseUrl"`
	} `json:"account"`
}
type AzureCommentEvent struct {
	EventType string `json:"eventType"`
	Resource  struct {
		Comment struct {
			Content string `json:"content"`
		} `json:"comment"`
		PullRequest struct {
			Repository    Repository `json:"repository"`
			PullRequestId int        `json:"pullRequestId"`
		} `json:"pullRequest"`
	} `json:"resource"`
	ResourceContainers ResourceContainers `json:"resourceContainers"`
}

type Azure struct {
	EventType    string `json:"eventType"`
	Event        interface{}
	ProjectName  string
	BaseUrl      string
	RepositoryId string
}

func GetAzureReposContext(azureContext string) (Azure, error) {
	var parsedAzureContext Azure
	err := json.Unmarshal([]byte(azureContext), &parsedAzureContext)
	if err != nil {
		return Azure{}, fmt.Errorf("error parsing Azure context JSON: %v", err)
	}
	return parsedAzureContext, nil
}

func (a *Azure) UnmarshalJSON(data []byte) error {
	type Alias Azure
	aux := struct {
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	switch a.EventType {
	case "git.pullrequest.updated", "git.pullrequest.created", "git.pullrequest.merged", "git.pullrequest.closed", "git.pullrequest.reopened":
		var event AzurePrEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		a.Event = event
		a.ProjectName = event.Resource.Repository.Project.Name
		a.BaseUrl = event.ResourceContainers.Account.BaseUrl
		a.RepositoryId = event.Resource.Repository.Id
	case "ms.vss-code.git-pullrequest-comment-event":
		var event AzureCommentEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		a.Event = event
		a.ProjectName = event.Resource.PullRequest.Repository.Project.Name
		a.BaseUrl = event.ResourceContainers.Account.BaseUrl
		a.RepositoryId = event.Resource.PullRequest.Repository.Id
	default:
		return errors.New("unknown Azure event: " + a.EventType)
	}

	return nil
}

func NewAzureReposService(patToken string, baseUrl string, projectName string, repositoryId string) (*AzureReposService, error) {
	client, err := git.NewClient(context.Background(), azuredevops.NewPatConnection(baseUrl, patToken))

	if err != nil {
		return nil, err
	}
	return &AzureReposService{
		Client:       client,
		ProjectName:  projectName,
		RepositoryId: repositoryId,
	}, nil
}

type AzureReposService struct {
	Client       git.Client
	ProjectName  string
	RepositoryId string
}

func (a *AzureReposService) GetUserTeams(organisation string, user string) ([]string, error) {
	return make([]string, 0), nil
}

func (a *AzureReposService) GetChangedFiles(prNumber int) ([]string, error) {

	pullRequest, err := a.Client.GetPullRequestById(context.Background(), git.GetPullRequestByIdArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
	})
	if err != nil {
		return nil, err
	}
	sourceCommitId := pullRequest.LastMergeSourceCommit.CommitId
	targetCommitId := pullRequest.LastMergeTargetCommit.CommitId

	repositoryId := pullRequest.Repository.Id.String()
	changes, err := a.Client.GetCommitDiffs(context.Background(), git.GetCommitDiffsArgs{
		Project:                 &a.ProjectName,
		RepositoryId:            &repositoryId,
		BaseVersionDescriptor:   &git.GitBaseVersionDescriptor{BaseVersion: targetCommitId, BaseVersionType: &git.GitVersionTypeValues.Commit},
		TargetVersionDescriptor: &git.GitTargetVersionDescriptor{TargetVersion: sourceCommitId, TargetVersionType: &git.GitVersionTypeValues.Commit},
	})
	if err != nil {
		return nil, err
	}

	var changedFiles []string
	for _, change := range *changes.Changes {
		if item, ok := change.(map[string]interface{})["item"].(map[string]interface{}); ok {
			if p, ok := item["path"].(string); ok {
				changedFiles = append(changedFiles, p)
			}
		}
	}
	return changedFiles, nil
}

func (a *AzureReposService) PublishComment(prNumber int, comment string) error {
	_, err := a.Client.CreateThread(context.Background(), git.CreateThreadArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
		RepositoryId:  &a.RepositoryId,
		CommentThread: &git.GitPullRequestCommentThread{
			Comments: &[]git.Comment{{
				Content: &comment,
			}},
		},
	})
	return err
}

func (a *AzureReposService) SetStatus(prNumber int, status string, statusContext string) error {
	var gitStatusState git.GitStatusState
	if status == "success" {
		gitStatusState = git.GitStatusStateValues.Succeeded
	} else if status == "failure" {
		gitStatusState = git.GitStatusStateValues.Failed
	} else if status == "pending" {
		gitStatusState = git.GitStatusStateValues.Pending
	} else {
		gitStatusState = git.GitStatusStateValues.NotSet
	}

	gitStatusContext := git.GitStatusContext{Name: &statusContext}
	_, err := a.Client.CreatePullRequestStatus(context.Background(), git.CreatePullRequestStatusArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
		RepositoryId:  &a.RepositoryId,
		Status: &git.GitPullRequestStatus{
			State:       &gitStatusState,
			Context:     &gitStatusContext,
			Description: &statusContext,
		},
	})
	return err
}

func (a *AzureReposService) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	pullRequestStatuses, err := a.Client.GetPullRequestStatuses(context.Background(), git.GetPullRequestStatusesArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
		RepositoryId:  &a.RepositoryId,
	})
	if err != nil {
		return "", err
	}

	latestUniqueRequestStatuses := make(map[string]*git.GitPullRequestStatus)

	for _, status := range *pullRequestStatuses {
		if status.Context == nil || status.Context.Name == nil || status.Context.Genre == nil {
			continue
		}
		key := fmt.Sprintf("%s/%s", *status.Context.Name, *status.Context.Genre)

		if res, ok := latestUniqueRequestStatuses[key]; !ok {
			latestUniqueRequestStatuses[key] = &status
		} else {
			if status.CreationDate.Time.After(res.CreationDate.Time) {
				latestUniqueRequestStatuses[key] = &status
			}
		}
	}

	for _, status := range latestUniqueRequestStatuses {
		if status.State != nil && (*status.State == git.GitStatusStateValues.Failed || *status.State == git.GitStatusStateValues.Error) {
			return "failure", nil
		}
	}

	var allSuccess = true
	for _, status := range latestUniqueRequestStatuses {
		if status.State != nil || *status.State != git.GitStatusStateValues.Succeeded {
			allSuccess = false
			break
		}
	}
	if allSuccess {
		return "success", nil
	}

	return "pending", nil
}

func (a *AzureReposService) MergePullRequest(prNumber int) error {
	pullRequest, err := a.Client.GetPullRequestById(context.Background(), git.GetPullRequestByIdArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
	})
	if err != nil {
		return err
	}
	_, err = a.Client.UpdatePullRequest(context.Background(), git.UpdatePullRequestArgs{
		Project:       &a.ProjectName,
		RepositoryId:  &a.RepositoryId,
		PullRequestId: &prNumber,
		GitPullRequestToUpdate: &git.GitPullRequest{
			LastMergeSourceCommit: pullRequest.LastMergeSourceCommit,
			Status:                &git.PullRequestStatusValues.Completed,
		},
	})
	return err
}

func (a *AzureReposService) IsMergeable(prNumber int) (bool, error) {
	pullRequest, err := a.Client.GetPullRequestById(context.Background(), git.GetPullRequestByIdArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
	})
	if err != nil {
		return false, err
	}
	return *pullRequest.MergeStatus == git.PullRequestAsyncStatusValues.Succeeded, nil
}

func (a *AzureReposService) IsClosed(prNumber int) (bool, error) {
	pullRequest, err := a.Client.GetPullRequestById(context.Background(), git.GetPullRequestByIdArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
	})
	if err != nil {
		return false, err
	}
	return *pullRequest.Status == git.PullRequestStatusValues.Abandoned, nil
}

func (a *AzureReposService) IsMerged(prNumber int) (bool, error) {
	pullRequest, err := a.Client.GetPullRequestById(context.Background(), git.GetPullRequestByIdArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
	})
	if err != nil {
		return false, err
	}
	return *pullRequest.Status == git.PullRequestStatusValues.Completed, nil
}

func (a *AzureReposService) EditComment(id interface{}, comment string) error {
	threadId := id.(int)
	comments := []git.Comment{
		{
			Content: &comment,
		},
	}
	_, err := a.Client.UpdateThread(context.Background(), git.UpdateThreadArgs{
		Project:      &a.ProjectName,
		RepositoryId: &a.RepositoryId,
		ThreadId:     &threadId,
		CommentThread: &git.GitPullRequestCommentThread{
			Comments: &comments,
		},
	})
	return err
}

func (a *AzureReposService) GetBranchName(prNumber int) (string, error) {
	//TODO implement me
	return "", nil
}

func (a *AzureReposService) GetComments(prNumber int) ([]orchestrator.Comment, error) {
	comments, err := a.Client.GetComments(context.Background(), git.GetCommentsArgs{
		Project:       &a.ProjectName,
		RepositoryId:  &a.RepositoryId,
		PullRequestId: &prNumber,
	})
	if err != nil {
		return nil, err
	}
	var result []orchestrator.Comment
	for _, comment := range *comments {
		result = append(result, orchestrator.Comment{
			Id:   *comment.Id,
			Body: comment.Content,
		})
	}
	return result, nil

}

func ProcessAzureReposEvent(azureEvent interface{}, diggerConfig *configuration.DiggerConfig, ciService orchestrator.PullRequestService) ([]configuration.Project, *configuration.Project, int, error) {
	var impactedProjects []configuration.Project
	var prNumber int

	switch azureEvent.(type) {
	case AzurePrEvent:
		prNumber = azureEvent.(AzurePrEvent).Resource.PullRequestId
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files: %v", err)
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case AzureCommentEvent:
		prNumber = azureEvent.(AzureCommentEvent).Resource.PullRequest.PullRequestId
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files: %v", err)
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
		requestedProject := utils.ParseProjectName(azureEvent.(AzureCommentEvent).Resource.Comment.Content)

		if requestedProject == "" {
			return impactedProjects, nil, prNumber, nil
		}

		for _, project := range impactedProjects {
			if project.Name == requestedProject {
				return impactedProjects, &project, prNumber, nil
			}
		}
		return nil, nil, 0, fmt.Errorf("requested project not found in modified projects")

	default:
		return nil, nil, 0, fmt.Errorf("unsupported event type")
	}
	return impactedProjects, nil, prNumber, nil
}

func ConvertAzureEventToCommands(parseAzureContext Azure, impactedProjects []configuration.Project, requestedProject *configuration.Project, workflows map[string]configuration.Workflow) ([]orchestrator.Job, bool, error) {
	jobs := make([]orchestrator.Job, 0)
	//&dependencyGraph, diggerProjectNamespace, parsedAzureContext.BaseUrl, parsedAzureContext.EventType, prNumber,
	switch parseAzureContext.EventType {
	case AzurePrCreated, AzurePrUpdated, AzurePrReopened:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
			}

			prNumber := parseAzureContext.Event.(AzurePrEvent).Resource.PullRequestId
			stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)
			jobs = append(jobs, orchestrator.Job{
				ProjectName:       project.Name,
				ProjectDir:        project.Dir,
				ProjectWorkspace:  project.Workspace,
				Terragrunt:        project.Terragrunt,
				Commands:          workflow.Configuration.OnPullRequestPushed,
				ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
				PullRequestNumber: &prNumber,
				EventName:         parseAzureContext.EventType,
				RequestedBy:       parseAzureContext.BaseUrl,
				Namespace:         parseAzureContext.BaseUrl + "/" + parseAzureContext.ProjectName,
				StateEnvVars:      stateEnvVars,
				CommandEnvVars:    commandEnvVars,
			})
		}
		return jobs, true, nil
	case AzurePrClosed:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
			}

			prNumber := parseAzureContext.Event.(AzurePrEvent).Resource.PullRequestId
			stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)
			jobs = append(jobs, orchestrator.Job{
				ProjectName:       project.Name,
				ProjectDir:        project.Dir,
				ProjectWorkspace:  project.Workspace,
				Terragrunt:        project.Terragrunt,
				Commands:          workflow.Configuration.OnPullRequestClosed,
				ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
				PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
				PullRequestNumber: &prNumber,
				EventName:         parseAzureContext.EventType,
				RequestedBy:       parseAzureContext.BaseUrl,
				Namespace:         parseAzureContext.BaseUrl + "/" + parseAzureContext.ProjectName,
				StateEnvVars:      stateEnvVars,
				CommandEnvVars:    commandEnvVars,
			})
		}
		return jobs, true, nil
	case AzurePrMerged:
		prNumber := parseAzureContext.Event.(AzurePrEvent).Resource.PullRequestId
		if parseAzureContext.Event.(AzurePrEvent).Resource.Repository.Status == "completed" {
			for _, project := range impactedProjects {
				workflow, ok := workflows[project.Workflow]
				if !ok {
					return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
				}
				stateEnvVars, commandEnvVars := configuration.CollectTerraformEnvConfig(workflow.EnvVars)
				jobs = append(jobs, orchestrator.Job{
					ProjectName:       project.Name,
					ProjectDir:        project.Dir,
					ProjectWorkspace:  project.Workspace,
					Terragrunt:        project.Terragrunt,
					Commands:          workflow.Configuration.OnCommitToDefault,
					ApplyStage:        orchestrator.ToConfigStage(workflow.Apply),
					PlanStage:         orchestrator.ToConfigStage(workflow.Plan),
					PullRequestNumber: &prNumber,
					EventName:         parseAzureContext.EventType,
					RequestedBy:       parseAzureContext.BaseUrl,
					Namespace:         parseAzureContext.BaseUrl + "/" + parseAzureContext.ProjectName,
					StateEnvVars:      stateEnvVars,
					CommandEnvVars:    commandEnvVars,
				})
			}
			return jobs, true, nil
		}
		return jobs, true, nil
	case AzurePrCommented:
		diggerCommand := strings.ToLower(parseAzureContext.Event.(AzureCommentEvent).Resource.Comment.Content)
		coversAllImpactedProjects := true
		runForProjects := impactedProjects

		prNumber := parseAzureContext.Event.(AzureCommentEvent).Resource.PullRequest.PullRequestId
		if requestedProject != nil {
			if len(impactedProjects) > 1 {
				coversAllImpactedProjects = false
				runForProjects = []configuration.Project{*requestedProject}
			} else if len(impactedProjects) == 1 && impactedProjects[0].Name != requestedProject.Name {
				return jobs, false, fmt.Errorf("requested project %v is not impacted by this PR", requestedProject.Name)
			}
		}

		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}
		for _, command := range supportedCommands {
			if strings.Contains(diggerCommand, command) {
				for _, project := range runForProjects {
					workspace := project.Workspace
					workspaceOverride, err := utils.ParseWorkspace(diggerCommand)
					if err != nil {
						return []orchestrator.Job{}, coversAllImpactedProjects, err
					}
					if workspaceOverride != "" {
						workspace = workspaceOverride
					}
					workflow, ok := workflows[project.Workflow]
					if !ok {
						return nil, false, fmt.Errorf("failed to find workflow config '%s' for project '%s'", project.Workflow, project.Name)
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
						PullRequestNumber: &prNumber,
						EventName:         parseAzureContext.EventType,
						RequestedBy:       parseAzureContext.BaseUrl,
						Namespace:         parseAzureContext.BaseUrl + "/" + parseAzureContext.ProjectName,
						StateEnvVars:      stateEnvVars,
						CommandEnvVars:    commandEnvVars,
					})
				}
			}
		}
		return jobs, coversAllImpactedProjects, nil

	default:
		return []orchestrator.Job{}, true, fmt.Errorf("unsupported Azure event type: %v", parseAzureContext.EventType)
	}
}
