package ci_backends

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/xanzy/go-gitlab"
	"strconv"
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
		"DIGGER_RUN_SPEC":          string(specBytes),
		"GITLAB_TOKEN":             vcsToken,
		"MERGE_REQUEST_EVENT_NAME": "",
		"CI_PIPELINE_ID":           "",
		"CI_PIPELINE_IID":          "",
		"CI_MERGE_REQUEST_ID":      "",
		"CI_MERGE_REQUEST_IID":     strconv.Itoa(*spec.Job.PullRequestNumber),
		"CI_PROJECT_NAME":          spec.VCS.RepoName,
		"CI_PROJECT_NAMESPACE":     spec.VCS.RepoFullname,
		"CI_PROJECT_ID":            "",
		"CI_PROJECT_NAMESPACE_ID":  "",
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
