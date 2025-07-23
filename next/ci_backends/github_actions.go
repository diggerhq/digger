package ci_backends

import (
	"context"
	"encoding/json"
	"log"

	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/google/go-github/v61/github"
)

type GithubActionCi struct {
	Client *github.Client
}

func (g GithubActionCi) TriggerWorkflow(spec spec.Spec, runName, vcsToken string) error {
	log.Printf("TriggerGithubWorkflow: repoOwner: %v, repoName: %v, commentId: %v", spec.VCS.RepoOwner, spec.VCS.RepoName, spec.CommentId)
	client := g.Client
	specBytes, err := json.Marshal(spec)

	inputs := orchestrator_scheduler.WorkflowInput{
		Spec:    string(specBytes),
		RunName: runName,
	}

	_, err = client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), spec.VCS.RepoOwner, spec.VCS.RepoName, spec.VCS.WorkflowFile, github.CreateWorkflowDispatchEventRequest{
		Ref:    spec.Job.Branch,
		Inputs: inputs.ToMap(),
	})

	return err
}

func (g GithubActionCi) GetWorkflowUrl(spec spec.Spec) (string, error) {
	return "", nil
}
