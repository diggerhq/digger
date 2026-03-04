package ci_backends

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"os"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/diggerhq/digger/libs/spec"
)

var (
	azureCred     *azidentity.DefaultAzureCredential
	azureCredOnce sync.Once
	azureCredErr  error
)

func getAzureCredential() (*azidentity.DefaultAzureCredential, error) {
	azureCredOnce.Do(func() {
		azureCred, azureCredErr = azidentity.NewDefaultAzureCredential(nil)
	})
	return azureCred, azureCredErr
}

type AzureDevOpsCi struct {
	Client      *http.Client
	AuthToken   string
	IsBasicAuth bool
}

var (
	pipelineIDCache   = make(map[string]string)
	pipelineIDCacheMu sync.Mutex
)

// resolvePipelineID resolves a workflow_file (pipeline name) to its numeric
// Azure DevOps pipeline ID by querying the Build Definitions API.  Results are
// cached for the lifetime of the process.  Falls back to AZURE_DEVOPS_PIPELINE_ID
// for simple single-pipeline deployments.
func (a AzureDevOpsCi) resolvePipelineID(pipelineName string) (string, error) {
	pipelineIDCacheMu.Lock()
	if id, ok := pipelineIDCache[pipelineName]; ok {
		pipelineIDCacheMu.Unlock()
		return id, nil
	}
	pipelineIDCacheMu.Unlock()

	org := os.Getenv("AZURE_DEVOPS_ORG")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")
	if org == "" || project == "" {
		return "", fmt.Errorf("AZURE_DEVOPS_ORG and AZURE_DEVOPS_PROJECT must be set")
	}

	apiURL := fmt.Sprintf(
		"https://dev.azure.com/%s/%s/_apis/build/definitions?name=%s&api-version=7.1",
		neturl.PathEscape(org), neturl.PathEscape(project), neturl.QueryEscape(pipelineName),
	)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create pipeline lookup request: %v", err)
	}
	if a.IsBasicAuth {
		req.SetBasicAuth("", a.AuthToken)
	} else {
		req.Header.Set("Authorization", "Bearer "+a.AuthToken)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return a.pipelineIDFallback(pipelineName, fmt.Errorf("API request failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return a.pipelineIDFallback(pipelineName, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body)))
	}

	var result struct {
		Count int `json:"count"`
		Value []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return a.pipelineIDFallback(pipelineName, fmt.Errorf("failed to decode response: %v", err))
	}

	if result.Count == 0 {
		return a.pipelineIDFallback(pipelineName, fmt.Errorf("no pipeline found with name %q", pipelineName))
	}

	id := fmt.Sprintf("%d", result.Value[0].ID)
	slog.Info("Resolved ADO pipeline name to ID", "name", pipelineName, "id", id)

	pipelineIDCacheMu.Lock()
	pipelineIDCache[pipelineName] = id
	pipelineIDCacheMu.Unlock()

	return id, nil
}

// pipelineIDFallback returns AZURE_DEVOPS_PIPELINE_ID when the API-based
// lookup cannot be performed.  If the env var is unset it surfaces the
// original error.
func (a AzureDevOpsCi) pipelineIDFallback(pipelineName string, lookupErr error) (string, error) {
	if fallback := os.Getenv("AZURE_DEVOPS_PIPELINE_ID"); fallback != "" {
		slog.Warn("Pipeline name lookup failed, falling back to AZURE_DEVOPS_PIPELINE_ID",
			"name", pipelineName, "error", lookupErr)
		return fallback, nil
	}
	return "", fmt.Errorf("failed to resolve pipeline %q: %v", pipelineName, lookupErr)
}

func NewAzureDevOpsCi() (*AzureDevOpsCi, error) {
	pat := os.Getenv("AZURE_DEVOPS_PAT")

	ci := &AzureDevOpsCi{
		Client: &http.Client{},
	}

	if pat != "" {
		ci.AuthToken = pat
		ci.IsBasicAuth = true
	} else {
		cred, err := getAzureCredential()
		if err != nil {
			return nil, fmt.Errorf("failed to obtain Azure credential: %v", err)
		}
		token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{
			Scopes: []string{"499b84ac-1181-4cfa-b6f4-a0fa2c36415b/.default"},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get Azure DevOps token: %v", err)
		}
		ci.AuthToken = token.Token
	}

	return ci, nil
}

func (a AzureDevOpsCi) TriggerWorkflow(spec spec.Spec, runName string, vcsToken string) error {
	slog.Info("TriggerAzureDevOpsWorkflow", "repoOwner", spec.VCS.RepoOwner, "repoName", spec.VCS.RepoName, "commentId", spec.CommentId)

	org := os.Getenv("AZURE_DEVOPS_ORG")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")

	if org == "" || project == "" {
		return fmt.Errorf("AZURE_DEVOPS_ORG and AZURE_DEVOPS_PROJECT must be set")
	}

	pipelineId, err := a.resolvePipelineID(spec.VCS.WorkflowFile)
	if err != nil {
		return fmt.Errorf("failed to resolve pipeline ID: %v", err)
	}

	specBytes, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("failed to marshal spec: %v", err)
	}

	payload := map[string]interface{}{
		"resources": map[string]interface{}{
			"repositories": map[string]interface{}{
				"self": map[string]string{
					"refName": "refs/heads/" + spec.Job.Branch,
				},
			},
		},
		"templateParameters": map[string]string{
			"DIGGER_SPEC":     string(specBytes),
			"DIGGER_RUN_NAME": runName,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	url := fmt.Sprintf("https://dev.azure.com/%s/%s/_apis/pipelines/%s/runs?api-version=7.1-preview.1", org, project, pipelineId)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if a.IsBasicAuth {
		req.SetBasicAuth("", a.AuthToken)
	} else {
		req.Header.Set("Authorization", "Bearer "+a.AuthToken)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to Azure DevOps: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Azure DevOps API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (a AzureDevOpsCi) GetWorkflowUrl(spec spec.Spec) (string, error) {
	org := os.Getenv("AZURE_DEVOPS_ORG")
	project := os.Getenv("AZURE_DEVOPS_PROJECT")

	if org == "" || project == "" {
		return "", fmt.Errorf("AZURE_DEVOPS_ORG and AZURE_DEVOPS_PROJECT must be set")
	}

	pipelineId, err := a.resolvePipelineID(spec.VCS.WorkflowFile)
	if err != nil {
		return "", fmt.Errorf("failed to resolve pipeline ID: %v", err)
	}

	// Returns the pipeline overview page as a fallback since the exact run ID isn't persisted locally
	return fmt.Sprintf("https://dev.azure.com/%s/%s/_build?definitionId=%s", org, project, pipelineId), nil
}
