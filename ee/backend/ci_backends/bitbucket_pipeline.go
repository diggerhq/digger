package ci_backends

import (
	"encoding/json"
	"fmt"
	orchestrator_bitbucket "github.com/diggerhq/digger/libs/ci/bitbucket"
	"github.com/diggerhq/digger/libs/spec"
)

type BitbucketPipelineCI struct {
	Client    *orchestrator_bitbucket.BitbucketAPI
	RepoOwner string
	RepoName  string
	Branch    string
}

func (bbp BitbucketPipelineCI) TriggerWorkflow(spec spec.Spec, runName string, vcsToken string) error {
	specBytes, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not serialize spec: %v", err)
	}

	variables := []interface{}{
		map[string]string{
			"key":   "ENVIRONMENT",
			"value": "production",
		},
		map[string]string{
			"key":   "DIGGER_RUN_SPEC",
			"value": string(specBytes),
		},
		map[string]string{
			"key":   "DIGGER_BITBUCKET_ACCESS_TOKEN",
			"value": vcsToken,
		},
	}
	_, err = bbp.Client.TriggerPipeline(bbp.Branch, variables)
	return err
}

// GetWorkflowUrl fetch workflow url after triggering a job
// since some CI don't return url automatically we split it out to become a
// followup method
func (bbp BitbucketPipelineCI) GetWorkflowUrl(spec spec.Spec) (string, error) {
	return "", nil
}
