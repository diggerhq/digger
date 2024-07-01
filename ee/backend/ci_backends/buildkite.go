package ci_backends

import (
	"encoding/json"
	"github.com/buildkite/go-buildkite/v3/buildkite"
	"github.com/diggerhq/digger/libs/spec"
)

type BuildkiteCi struct {
	Client   buildkite.Client
	Org      string
	Pipeline string
}

func (b BuildkiteCi) TriggerWorkflow(spec spec.Spec, runName string, vcsToken string) error {

	specBytes, err := json.Marshal(spec)
	client := b.Client
	_, _, err = client.Builds.Create(b.Org, b.Pipeline, &buildkite.CreateBuild{
		Commit:  spec.Job.Commit,
		Branch:  spec.Job.Branch,
		Message: runName,
		Author:  buildkite.Author{Username: spec.VCS.Actor},
		Env: map[string]string{
			"DIGGER_SPEC":  string(specBytes),
			"GITHUB_TOKEN": vcsToken,
		},
		PullRequestID: int64(*spec.Job.PullRequestNumber),
	})

	return err

}
