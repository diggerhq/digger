package azure

import (
	"context"
	"digger/pkg/ci"
	"digger/pkg/configuration"
	"digger/pkg/models"
	"digger/pkg/utils"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"strings"
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
type Resource struct {
	Repository struct {
		Id      string `json:"id"`
		Name    string `json:"name"`
		Project struct {
			Name string `json:"name"`
		}
		Status        string `json:"status"`
		PullRequestId int    `json:"pullRequestId"`
	}
}

type ResourceContainers struct {
	Account struct {
		BaseUrl string `json:"baseUrl"`
	}
}
type AzureCommentEvent struct {
	EventType string `json:"eventType"`
	Comment   struct {
		Content string `json:"content"`
	}
	PullRequest struct {
		Resource Resource `json:"resource"`
	}
	ResourceContainers ResourceContainers `json:"resourceContainers"`
}

type Azure struct {
	EventType   string `json:"eventType"`
	Event       interface{}
	ProjectName string
	BaseUrl     string
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
	var rawEvent json.RawMessage
	auxEvent := struct {
		Event *json.RawMessage `json:"eventType"`
	}{
		Event: &rawEvent,
	}
	if err := json.Unmarshal(data, &auxEvent); err != nil {
		return err
	}

	switch a.Event {
	case "git.pullrequest.updated", "git.pullrequest.created", "git.pullrequest.merged", "git.pullrequest.closed", "git.pullrequest.reopened":
		var event AzurePrEvent
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			return err
		}
		a.Event = event
		a.ProjectName = event.Resource.Repository.Project.Name
		a.BaseUrl = event.ResourceContainers.Account.BaseUrl
	case "ms.vss-code.git-pullrequest-comment-event":
		var event AzureCommentEvent
		if err := json.Unmarshal(rawEvent, &event); err != nil {
			return err
		}
		a.Event = event
		a.ProjectName = event.PullRequest.Resource.Repository.Project.Name
		a.BaseUrl = event.ResourceContainers.Account.BaseUrl
	default:
		return errors.New("unknown Azure event: " + a.EventType)
	}

	return nil
}

func NewAzureReposService(patToken string, baseUrl string, projectName string) (*AzureReposService, error) {
	client, err := git.NewClient(context.Background(), &azuredevops.Connection{
		AuthorizationString: patToken,
		BaseUrl:             baseUrl,
	})

	if err != nil {
		return nil, err
	}
	return &AzureReposService{
		Client:      client,
		ProjectName: projectName,
	}, nil
}

type AzureReposService struct {
	Client      git.Client
	ProjectName string
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
		BaseVersionDescriptor:   &git.GitBaseVersionDescriptor{Version: sourceCommitId, VersionType: &git.GitVersionTypeValues.Commit},
		TargetVersionDescriptor: &git.GitTargetVersionDescriptor{Version: targetCommitId, VersionType: &git.GitVersionTypeValues.Commit},
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
		CommentThread: &git.GitPullRequestCommentThread{
			Comments: &[]git.Comment{{
				Content: &comment,
			}},
		},
	})
	return err
}

func (a *AzureReposService) SetStatus(prNumber int, status string, statusContext string) error {
	gitStatusState := git.GitStatusState(status)
	gitStatusContext := git.GitStatusContext{Name: &statusContext}
	_, err := a.Client.CreatePullRequestStatus(context.Background(), git.CreatePullRequestStatusArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
		Status: &git.GitPullRequestStatus{
			State:       &gitStatusState,
			Context:     &gitStatusContext,
			Description: &status,
		},
	})
	return err
}

func (a *AzureReposService) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	pullRequestStatuses, err := a.Client.GetPullRequestStatuses(context.Background(), git.GetPullRequestStatusesArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
	})
	if err != nil {
		return "", err
	}
	for _, status := range *pullRequestStatuses {
		if *status.State == git.GitStatusStateValues.Failed || *status.State == git.GitStatusStateValues.Error {
			return "failure", nil
		}
	}

	var allSuccess = true
	for _, status := range *pullRequestStatuses {
		if *status.State != git.GitStatusStateValues.Succeeded {
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
	_, err := a.Client.UpdatePullRequest(context.Background(), git.UpdatePullRequestArgs{
		Project:       &a.ProjectName,
		PullRequestId: &prNumber,
		GitPullRequestToUpdate: &git.GitPullRequest{
			Status: &git.PullRequestStatusValues.Completed,
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
	return *pullRequest.Status == git.PullRequestStatusValues.Completed, nil
}

func ProcessAzureReposEvent(azureEvent interface{}, diggerConfig *configuration.DiggerConfig, ciService ci.CIService) ([]configuration.Project, *configuration.Project, int, error) {
	var impactedProjects []configuration.Project
	var prNumber int

	switch azureEvent.(type) {
	case AzurePrEvent:
		prNumber = azureEvent.(AzurePrEvent).Resource.Repository.PullRequestId
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
	case AzureCommentEvent:
		prNumber = azureEvent.(AzureCommentEvent).PullRequest.Resource.Repository.PullRequestId
		changedFiles, err := ciService.GetChangedFiles(prNumber)

		if err != nil {
			return nil, nil, 0, fmt.Errorf("could not get changed files")
		}

		impactedProjects = diggerConfig.GetModifiedProjects(changedFiles)
		requestedProject := utils.ParseProjectName(azureEvent.(AzureCommentEvent).Comment.Content)

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

func ConvertAzureEventToCommands(parseAzureContext Azure, impactedProjects []configuration.Project, workflows map[string]configuration.Workflow) ([]models.ProjectCommand, error) {
	commandsPerProject := make([]models.ProjectCommand, 0)

	switch parseAzureContext.EventType {
	case AzurePrCreated, AzurePrUpdated, AzurePrReopened:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				workflow = workflows["default"]
			}

			commandsPerProject = append(commandsPerProject, models.ProjectCommand{
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
	case AzurePrClosed:
		for _, project := range impactedProjects {
			workflow, ok := workflows[project.Workflow]
			if !ok {
				workflow = workflows["default"]
			}

			commandsPerProject = append(commandsPerProject, models.ProjectCommand{
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
	case AzurePrMerged:
		if parseAzureContext.Event.(AzurePrEvent).Resource.Repository.Status == "completed" {
			for _, project := range impactedProjects {
				workflow, ok := workflows[project.Workflow]
				if !ok {
					workflow = workflows["default"]
				}

				commandsPerProject = append(commandsPerProject, models.ProjectCommand{
					ProjectName:      project.Name,
					ProjectDir:       project.Dir,
					ProjectWorkspace: project.Workspace,
					Terragrunt:       project.Terragrunt,
					Commands:         workflow.Configuration.OnCommitToDefault,
					ApplyStage:       workflow.Apply,
					PlanStage:        workflow.Plan,
				})
			}
			return commandsPerProject, nil
		}
		return commandsPerProject, nil
	case AzurePrCommented:

		diggerCommand := parseAzureContext.Event.(AzureCommentEvent).Comment.Content

		supportedCommands := []string{"digger plan", "digger apply", "digger unlock", "digger lock"}
		for _, command := range supportedCommands {
			if strings.Contains(diggerCommand, command) {
				for _, project := range impactedProjects {
					workspace := project.Workspace
					workspaceOverride, err := utils.ParseWorkspace(diggerCommand)
					if err != nil {
						return []models.ProjectCommand{}, err
					}
					if workspaceOverride != "" {
						workspace = workspaceOverride
					}
					commandsPerProject = append(commandsPerProject, models.ProjectCommand{
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
		return []models.ProjectCommand{}, fmt.Errorf("unsupported Azure event type: %v", parseAzureContext.EventType)
	}
}
