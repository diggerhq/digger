package ci_backends

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/utils"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/google/go-github/v61/github"
)

type GithubActionCi struct {
	Client *github.Client
}

func (g GithubActionCi) TriggerWorkflow(spec spec.Spec, runName string, vcsToken string) error {
	slog.Info("TriggerGithubWorkflow", "repoOwner", spec.VCS.RepoOwner, "repoName", spec.VCS.RepoName, "commentId", spec.CommentId)
	client := g.Client
	specBytes, err := json.Marshal(spec)

	inputs := orchestrator_scheduler.WorkflowInput{
		Spec:    string(specBytes),
		RunName: runName,
	}

	ref, err := g.resolveWorkflowRef(context.Background(), spec)
	if err != nil {
		return err
	}

	_, err = client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), spec.VCS.RepoOwner, spec.VCS.RepoName, spec.VCS.WorkflowFile, github.CreateWorkflowDispatchEventRequest{
		Ref:    ref,
		Inputs: inputs.ToMap(),
	})

	return err
}

// resolveWorkflowRef returns the git ref that should be used when triggering
// the workflow. When the `force_trigger_from_default_branch` flag is enabled
// we query GitHub for the repository's default branch; otherwise, we use the
// branch present in the job spec.
func (g GithubActionCi) resolveWorkflowRef(ctx context.Context, spec spec.Spec) (string, error) {
	client := g.Client
	ref := spec.Job.Branch

	if config.DiggerConfig.GetBool("force_trigger_from_default_branch") {
		repo, _, rErr := client.Repositories.Get(ctx, spec.VCS.RepoOwner, spec.VCS.RepoName)
		if rErr != nil {
			slog.Error("Failed to fetch repository info to determine default branch", "owner", spec.VCS.RepoOwner, "repo", spec.VCS.RepoName, "error", rErr)
			return "", fmt.Errorf("failed to fetch repo info to get default branch: %v", rErr)
		}
		if repo.DefaultBranch != nil && *repo.DefaultBranch != "" {
			ref = *repo.DefaultBranch
			slog.Info("Forcing workflow ref to repository default branch", "repo", spec.VCS.RepoFullname, "defaultBranch", ref)
		} else {
			// If GitHub doesn't return a default branch, fall back to 'main'.
			ref = "main"
			slog.Info("Repository default branch unknown — falling back to 'main'", "repo", spec.VCS.RepoFullname)
		}
	}

	return ref, nil
}

func (g GithubActionCi) GetWorkflowUrl(spec spec.Spec) (string, error) {
	if spec.JobId == "" {
		slog.Error("Cannot get workflow URL: JobId is empty")
		return "", fmt.Errorf("job ID is required to fetch workflow URL")
	}

	_, workflowRunUrl, err := utils.GetWorkflowIdAndUrlFromDiggerJobId(g.Client, spec.VCS.RepoOwner, spec.VCS.RepoName, spec.JobId)
	if err != nil {
		return "", err
	} else {
		return workflowRunUrl, nil
	}
}
