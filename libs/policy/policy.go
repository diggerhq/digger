package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/open-policy-agent/opa/rego"
)

const DefaultAccessPolicy = `
package digger
default allow = true
allow = (count(input.planPolicyViolations) == 0)
`

type DiggerHttpPolicyProvider struct {
	DiggerHost         string
	DiggerOrganisation string
	AuthToken          string
	HttpClient         *http.Client
}

type NoOpPolicyChecker struct{}

func (p NoOpPolicyChecker) CheckAccessPolicy(ciService ci.OrgService, prService *ci.PullRequestService, scmOrganisation, scmRepository, projectName, projectDir, command string, prNumber *int, requestedBy string, planPolicyViolations []string) (bool, error) {
	return true, nil
}

func (p NoOpPolicyChecker) CheckPlanPolicy(scmRepository, scmOrganisation, projectname, projectDir, planOutput string) (bool, []string, error) {
	return true, nil, nil
}

func (p NoOpPolicyChecker) CheckDriftPolicy(scmOrganisation, scmRepository, projectname string) (bool, error) {
	return true, nil
}

func getAccessPolicyForOrganisation(p *DiggerHttpPolicyProvider) (string, *http.Response, error) {
	organisation := p.DiggerOrganisation
	u, err := url.Parse(p.DiggerHost)
	if err != nil {
		slog.Error("Failed to parse digger cloud URL", "url", p.DiggerHost, "error", err)
		return "", nil, fmt.Errorf("not able to parse digger cloud url: %v", err)
	}
	u.Path = "/orgs/" + organisation + "/access-policy"

	slog.Debug("Fetching org access policy", "organisation", organisation, "url", u.String())

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Add("Authorization", "Bearer "+p.AuthToken)

	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp, nil
	}
	return string(body), resp, nil
}

func getPlanPolicyForOrganisation(p *DiggerHttpPolicyProvider) (string, *http.Response, error) {
	organisation := p.DiggerOrganisation
	u, err := url.Parse(p.DiggerHost)
	if err != nil {
		slog.Error("Failed to parse digger cloud URL", "url", p.DiggerHost, "error", err)
		return "", nil, fmt.Errorf("not able to parse digger cloud url: %v", err)
	}
	u.Path = "/orgs/" + organisation + "/plan-policy"

	slog.Debug("Fetching org plan policy", "organisation", organisation, "url", u.String())

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Add("Authorization", "Bearer "+p.AuthToken)

	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp, nil
	}
	return string(body), resp, nil
}

func getDriftPolicyForOrganisation(p *DiggerHttpPolicyProvider) (string, *http.Response, error) {
	organisation := p.DiggerOrganisation
	u, err := url.Parse(p.DiggerHost)
	if err != nil {
		slog.Error("Failed to parse digger cloud URL", "url", p.DiggerHost, "error", err)
		return "", nil, fmt.Errorf("not able to parse digger cloud url: %v", err)
	}
	u.Path = "/orgs/" + organisation + "/drift-policy"

	slog.Debug("Fetching org drift policy", "organisation", organisation, "url", u.String())

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Add("Authorization", "Bearer "+p.AuthToken)

	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp, nil
	}
	return string(body), resp, nil
}

func getAccessPolicyForNamespace(p *DiggerHttpPolicyProvider, namespace, projectName string) (string, *http.Response, error) {
	// fetch RBAC policies for project from Digger API
	u, err := url.Parse(p.DiggerHost)
	if err != nil {
		slog.Error("Failed to parse digger cloud URL", "url", p.DiggerHost, "error", err)
		return "", nil, fmt.Errorf("not able to parse digger cloud url: %v", err)
	}
	u.Path = "/repos/" + namespace + "/projects/" + projectName + "/access-policy"

	slog.Debug("Fetching namespace access policy",
		"namespace", namespace,
		"projectName", projectName,
		"url", u.String())

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Add("Authorization", "Bearer "+p.AuthToken)

	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return "", resp, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp, nil
	}
	return string(body), resp, nil
}

func getPlanPolicyForNamespace(p *DiggerHttpPolicyProvider, namespace, projectName string) (string, *http.Response, error) {
	u, err := url.Parse(p.DiggerHost)
	if err != nil {
		slog.Error("Failed to parse digger cloud URL", "url", p.DiggerHost, "error", err)
		return "", nil, fmt.Errorf("not able to parse digger cloud url: %v", err)
	}
	u.Path = "/repos/" + namespace + "/projects/" + projectName + "/plan-policy"

	slog.Debug("Fetching namespace plan policy",
		"namespace", namespace,
		"projectName", projectName,
		"url", u.String())

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Add("Authorization", "Bearer "+p.AuthToken)

	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp, nil
	}
	return string(body), resp, nil
}

// GetPolicy fetches policy for particular project,  if not found then it will fallback to org level policy
func (p DiggerHttpPolicyProvider) GetAccessPolicy(organisation, repo, projectName, projectDir string) (string, error) {
	namespace := fmt.Sprintf("%v-%v", organisation, repo)

	slog.Debug("Getting access policy",
		"organisation", organisation,
		"repo", repo,
		"projectName", projectName,
		"projectDir", projectDir)

	content, resp, err := getAccessPolicyForNamespace(&p, namespace, projectName)
	if err != nil {
		slog.Error("Failed to fetch access policy for namespace",
			"namespace", namespace,
			"error", err)
		return "", fmt.Errorf("error while fetching access policy for namespace: %v", err)
	}

	// project policy found
	if resp.StatusCode == http.StatusOK && content != "" {
		slog.Debug("Found project access policy", "namespace", namespace, "projectName", projectName)
		return content, nil
	}

	// check if project policy was empty or not found (retrieve org policy if so)
	if (resp.StatusCode == http.StatusOK && content == "") || resp.StatusCode == http.StatusNotFound {
		slog.Debug("Project access policy not found, falling back to org policy",
			"organisation", organisation)

		content, resp, err := getAccessPolicyForOrganisation(&p)
		if err != nil {
			slog.Error("Failed to fetch access policy for organisation",
				"organisation", organisation,
				"error", err)
			return "", fmt.Errorf("error while fetching access policy for organisation: %v", err)
		}
		switch resp.StatusCode {
		case http.StatusOK:
			slog.Debug("Found organisation access policy", "organisation", organisation)
			return content, nil
		case http.StatusNotFound:
			slog.Debug("Organisation access policy not found, using default", "organisation", organisation)
			return DefaultAccessPolicy, nil
		default:
			slog.Error("Unexpected response for organisation policy",
				"statusCode", resp.StatusCode,
				"response", content)
			return "", fmt.Errorf("unexpected response while fetching organisation policy: %v, code %v", content, resp.StatusCode)
		}
	} else {
		slog.Error("Unexpected response for project policy",
			"statusCode", resp.StatusCode,
			"response", content)
		return "", fmt.Errorf("unexpected response while fetching project policy: %v code %v", content, resp.StatusCode)
	}
}

func (p DiggerHttpPolicyProvider) GetPlanPolicy(organisation, repo, projectName, projectDir string) (string, error) {
	namespace := fmt.Sprintf("%v-%v", organisation, repo)

	slog.Debug("Getting plan policy",
		"organisation", organisation,
		"repo", repo,
		"projectName", projectName,
		"projectDir", projectDir)

	content, resp, err := getPlanPolicyForNamespace(&p, namespace, projectName)
	if err != nil {
		slog.Error("Failed to fetch plan policy for namespace",
			"namespace", namespace,
			"error", err)
		return "", err
	}

	// project policy found
	if resp.StatusCode == http.StatusOK && content != "" {
		slog.Debug("Found project plan policy", "namespace", namespace, "projectName", projectName)
		return content, nil
	}

	// check if project policy was empty or not found (retrieve org policy if so)
	if (resp.StatusCode == http.StatusOK && content == "") || resp.StatusCode == http.StatusNotFound {
		slog.Debug("Project plan policy not found, falling back to org policy",
			"organisation", organisation)

		content, resp, err := getPlanPolicyForOrganisation(&p)
		if err != nil {
			slog.Error("Failed to fetch plan policy for organisation",
				"organisation", organisation,
				"error", err)
			return "", err
		}
		switch resp.StatusCode {
		case http.StatusOK:
			slog.Debug("Found organisation plan policy", "organisation", organisation)
			return content, nil
		case http.StatusNotFound:
			slog.Debug("Organisation plan policy not found", "organisation", organisation)
			return "", nil
		default:
			slog.Error("Unexpected response for organisation policy",
				"statusCode", resp.StatusCode,
				"response", content)
			return "", fmt.Errorf("unexpected response while fetching organisation policy: %v, code %v", content, resp.StatusCode)
		}
	} else {
		slog.Error("Unexpected response for project policy",
			"statusCode", resp.StatusCode,
			"response", content)
		return "", fmt.Errorf("unexpected response while fetching project policy: %v code %v", content, resp.StatusCode)
	}
}

func (p DiggerHttpPolicyProvider) GetDriftPolicy() (string, error) {
	slog.Debug("Getting drift policy", "organisation", p.DiggerOrganisation)

	content, resp, err := getDriftPolicyForOrganisation(&p)
	if err != nil {
		slog.Error("Failed to fetch drift policy",
			"organisation", p.DiggerOrganisation,
			"error", err)
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		slog.Debug("Found drift policy", "organisation", p.DiggerOrganisation)
		return content, nil
	case http.StatusNotFound:
		slog.Debug("Drift policy not found", "organisation", p.DiggerOrganisation)
		return "", nil
	default:
		slog.Error("Unexpected response for drift policy",
			"statusCode", resp.StatusCode,
			"response", content)
		return "", fmt.Errorf("unexpected response while fetching organisation policy: %v, code %v", content, resp.StatusCode)
	}
}

func (p DiggerHttpPolicyProvider) GetOrganisation() string {
	return p.DiggerOrganisation
}

type DiggerPolicyChecker struct {
	PolicyProvider Provider
}

// TODO refactor to use AccessPolicyContext - too many arguments
func (p DiggerPolicyChecker) CheckAccessPolicy(ciService ci.OrgService, prService *ci.PullRequestService, scmOrganisation, scmRepository, projectName, projectDir, command string, prNumber *int, requestedBy string, planPolicyViolations []string) (bool, error) {
	slog.Debug("Checking access policy",
		"organisation", scmOrganisation,
		"repository", scmRepository,
		"project", projectName,
		"command", command,
		"requestedBy", requestedBy)

	policy, err := p.PolicyProvider.GetAccessPolicy(scmOrganisation, scmRepository, projectName, projectDir)
	if err != nil {
		slog.Error("Error fetching policy", "error", err)
		return false, err
	}

	teams, err := ciService.GetUserTeams(scmOrganisation, requestedBy)
	if err != nil {
		slog.Error("Error fetching user teams",
			"organisation", scmOrganisation,
			"user", requestedBy,
			"error", err)
		slog.Warn("Teams failed to be fetched, using empty list for access policy checks")
		teams = []string{}
	}

	// list of pull request approvals (if applicable)
	approvals := make([]string, 0)
	if prService != nil && prNumber != nil {
		approvals, err = (*prService).GetApprovals(*prNumber)
		if err != nil {
			slog.Warn("Failed to get PR approvals",
				"prNumber", *prNumber,
				"error", err)
		}
	}

	input := map[string]interface{}{
		"user":                 requestedBy,
		"organisation":         scmOrganisation,
		"teams":                teams,
		"approvals":            approvals,
		"planPolicyViolations": planPolicyViolations,
		"action":               command,
		"project":              projectName,
	}

	if policy == "" {
		slog.Debug("No access policy found, allowing action")
		return true, nil
	}

	ctx := context.Background()
	slog.Debug("Evaluating access policy",
		"input", input,
		"policy", policy)

	query, err := rego.New(
		rego.Query("data.digger.allow"),
		rego.Module("digger", policy),
	).PrepareForEval(ctx)
	if err != nil {
		slog.Error("Failed to prepare policy evaluation", "error", err)
		return false, err
	}

	results, err := query.Eval(ctx, rego.EvalInput(input))
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		slog.Error("No result found from policy evaluation")
		return false, fmt.Errorf("no result found")
	}

	expressions := results[0].Expressions

	for _, expression := range expressions {
		decision, ok := expression.Value.(bool)
		if !ok {
			slog.Error("Policy decision is not a boolean")
			return false, fmt.Errorf("decision is not a boolean")
		}
		if !decision {
			slog.Info("Access policy denied action",
				"user", requestedBy,
				"action", command,
				"project", projectName)
			return false, nil
		}
	}

	slog.Info("Access policy allowed action",
		"user", requestedBy,
		"action", command,
		"project", projectName)
	return true, nil
}

func (p DiggerPolicyChecker) CheckPlanPolicy(scmRepository, scmOrganisation, projectname, projectDir, planOutput string) (bool, []string, error) {
	slog.Debug("Checking plan policy",
		"organisation", scmOrganisation,
		"repository", scmRepository,
		"project", projectname)

	policy, err := p.PolicyProvider.GetPlanPolicy(scmOrganisation, scmRepository, projectname, projectDir)
	if err != nil {
		slog.Error("Failed to get plan policy", "error", err)
		return false, nil, fmt.Errorf("failed get plan policy: %v", err)
	}
	var parsedPlanOutput map[string]interface{}

	err = json.Unmarshal([]byte(planOutput), &parsedPlanOutput)
	if err != nil {
		slog.Error("Failed to parse terraform plan output", "error", err)
		return false, nil, fmt.Errorf("failed to parse json terraform output to map: %v", err)
	}

	input := map[string]interface{}{
		"terraform": parsedPlanOutput,
	}

	if policy == "" {
		slog.Info("No plan policies found, succeeding")
		return true, nil, nil
	}

	ctx := context.Background()
	slog.Debug("Evaluating plan policy", "policy", policy)

	query, err := rego.New(
		rego.Query("data.digger.deny"),
		rego.Module("digger", policy),
	).PrepareForEval(ctx)
	if err != nil {
		slog.Error("Failed to prepare plan policy evaluation", "error", err)
		return false, nil, err
	}

	results, err := query.Eval(ctx, rego.EvalInput(input))
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		slog.Error("No result found from plan policy evaluation")
		return false, nil, fmt.Errorf("no result found")
	}

	expressions := results[0].Expressions

	decisionsResult := make([]string, 0)
	for _, expression := range expressions {
		decisions, ok := expression.Value.([]interface{})

		if !ok {
			slog.Error("Plan policy decision is not a slice of interfaces")
			return false, nil, fmt.Errorf("decision is not a slice of interfaces")
		}
		if len(decisions) > 0 {
			for _, d := range decisions {
				decisionsResult = append(decisionsResult, d.(string))
				slog.Info("Plan policy violation", "reason", d)
			}
		}
	}

	if len(decisionsResult) > 0 {
		slog.Info("Plan policy check failed",
			"violations", len(decisionsResult),
			"organisation", scmOrganisation,
			"repository", scmRepository,
			"project", projectname)
		return false, decisionsResult, nil
	}

	slog.Info("Plan policy check passed",
		"organisation", scmOrganisation,
		"repository", scmRepository,
		"project", projectname)
	return true, []string{}, nil
}

func (p DiggerPolicyChecker) CheckDriftPolicy(scmOrganisation, scmRepository, projectName string) (bool, error) {
	slog.Debug("Checking drift policy",
		"organisation", scmOrganisation,
		"repository", scmRepository,
		"project", projectName)

	// TODO: Get rid of organisation if its not needed
	// organisation := p.PolicyProvider.GetOrganisation()
	policy, err := p.PolicyProvider.GetDriftPolicy()
	if err != nil {
		slog.Error("Error fetching drift policy", "error", err)
		return false, err
	}

	input := map[string]interface{}{
		"organisation": scmOrganisation,
		"project":      projectName,
	}

	if policy == "" {
		slog.Debug("No drift policy found, allowing drift detection")
		return true, nil
	}

	ctx := context.Background()
	slog.Debug("Evaluating drift policy",
		"input", input,
		"policy", policy)

	query, err := rego.New(
		rego.Query("data.digger.enable"),
		rego.Module("digger", policy),
	).PrepareForEval(ctx)
	if err != nil {
		slog.Error("Failed to prepare drift policy evaluation", "error", err)
		return false, err
	}

	results, err := query.Eval(ctx, rego.EvalInput(input))
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		slog.Error("No result found from drift policy evaluation")
		return false, fmt.Errorf("no result found")
	}

	expressions := results[0].Expressions

	for _, expression := range expressions {
		decision, ok := expression.Value.(bool)
		if !ok {
			slog.Error("Drift policy decision is not a boolean")
			return false, fmt.Errorf("decision is not a boolean")
		}
		if !decision {
			slog.Info("Drift detection disabled by policy",
				"organisation", scmOrganisation,
				"project", projectName)
			return false, nil
		}
	}

	slog.Info("Drift detection enabled by policy",
		"organisation", scmOrganisation,
		"project", projectName)
	return true, nil
}

func NewPolicyChecker(hostname, organisationName, authToken string) Checker {
	var policyChecker Checker
	if os.Getenv("NO_BACKEND") == "true" {
		slog.Warn("Running in 'backendless' mode. Features that require backend will not be available.")
		policyChecker = NoOpPolicyChecker{}
	} else {
		slog.Info("Initializing policy checker",
			"hostname", hostname,
			"organisation", organisationName)

		policyChecker = DiggerPolicyChecker{
			PolicyProvider: &DiggerHttpPolicyProvider{
				DiggerHost:         hostname,
				DiggerOrganisation: organisationName,
				AuthToken:          authToken,
				HttpClient:         http.DefaultClient,
			},
		}
	}
	return policyChecker
}
