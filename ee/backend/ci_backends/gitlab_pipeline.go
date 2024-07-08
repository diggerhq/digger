package ci_backends

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/xanzy/go-gitlab"
	"strconv"
)

type GitlabPipelineCI struct {
	Client                      *gitlab.Client
	GitlabProjectId             int
	GitlabmergeRequestEventName string
	GitlabCIPipelineID          string
	GitlabCIPipelineIID         int
	GitlabCIMergeRequestID      int
	GitlabCIMergeRequestIID     int
	GitlabCIProjectName         string
	GitlabciprojectNamespace    string
	GitlabciprojectId           int
	GitlabciprojectNamespaceId  int
	GitlabDiscussionId          string
}

func (gl GitlabPipelineCI) TriggerWorkflow(spec spec.Spec, runName string, vcsToken string) error {
	specBytes, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not serialize spec: %v", err)
	}
	vars := map[string]string{
		"DIGGER_RUN_NAME":          runName,
		"DIGGER_RUN_SPEC":          string(specBytes),
		"GITLAB_TOKEN":             vcsToken,
		"MERGE_REQUEST_EVENT_NAME": gl.GitlabmergeRequestEventName,
		"CI_PIPELINE_ID":           gl.GitlabCIPipelineID,
		"CI_PIPELINE_IID":          strconv.Itoa(gl.GitlabCIPipelineIID),
		"CI_MERGE_REQUEST_ID":      strconv.Itoa(gl.GitlabCIMergeRequestID),
		"CI_MERGE_REQUEST_IID":     strconv.Itoa(gl.GitlabCIMergeRequestIID),
		"CI_PROJECT_NAME":          spec.VCS.RepoName,
		"CI_PROJECT_NAMESPACE":     spec.VCS.RepoFullname,
		"CI_PROJECT_ID":            strconv.Itoa(gl.GitlabProjectId),
		"CI_PROJECT_NAMESPACE_ID":  gl.GitlabciprojectNamespace,
		"DISCUSSION_ID":            gl.GitlabDiscussionId,
	}
	variables := []*gitlab.PipelineVariableOptions{}
	for k, v := range vars {
		variables = append(variables, &gitlab.PipelineVariableOptions{
			Key:          &k,
			Value:        gitlab.String(v),
			VariableType: gitlab.String("env_var"),
		})
	}
	client := gl.Client
	_, _, err = client.Pipelines.CreatePipeline(spec.VCS.RepoFullname, &gitlab.CreatePipelineOptions{
		Ref:       &spec.Job.Branch,
		Variables: &variables,
	}, nil)

	return err
}
