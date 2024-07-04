package ci_backends

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/xanzy/go-gitlab"
)

type GitlabPipelineCI struct {
	Client *gitlab.Client
}

func (gl GitlabPipelineCI) TriggerWorkflow(spec spec.Spec, runName string, vcsToken string) error {
	specBytes, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not serialize spec: %v", err)
	}
	vars := map[string]string{
		"DIGGER_SPEC":  string(specBytes),
		"GITLAB_TOKEN": vcsToken,
	}
	variables := []*gitlab.PipelineVariableOptions{}
	for k, v := range vars {
		variables = append(variables, &gitlab.PipelineVariableOptions{
			Key:   &k,
			Value: &v,
		})
	}
	client := gl.Client
	_, _, err = client.Pipelines.CreatePipeline(spec.VCS.RepoFullname, &gitlab.CreatePipelineOptions{
		Ref:       &spec.Job.Branch,
		Variables: &variables,
	}, nil)

	return err
}
