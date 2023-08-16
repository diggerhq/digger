package policy

import (
	"context"
	"digger/pkg/ci"
	"digger/pkg/core/policy"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-policy-agent/opa/rego"
	"io"
	"log"
	"net/http"
	"net/url"
)

type DiggerHttpPolicyProvider struct {
	DiggerHost         string
	DiggerOrganisation string
	AuthToken          string
	HttpClient         *http.Client
}

type NoOpPolicyChecker struct {
}

func (p NoOpPolicyChecker) CheckAccessPolicy(_ ci.OrgService, _ string, _ string, _ string, _ string, _ string) (bool, error) {
	return true, nil
}

func (p NoOpPolicyChecker) CheckPlanPolicy(_ string, _ string, _ string) (bool, []string, error) {
	return true, nil, nil
}

func (p NoOpPolicyChecker) CheckDriftPolicy(SCMOrganisation string, SCMrepository string, projectname string) (bool, error) {
	return true, nil
}

func getAccessPolicyForOrganisation(p *DiggerHttpPolicyProvider) (string, *http.Response, error) {
	organisation := p.DiggerOrganisation
	u, err := url.Parse(p.DiggerHost)
	if err != nil {
		log.Fatalf("Not able to parse digger cloud url: %v", err)
	}
	u.Path = "/orgs/" + organisation + "/access-policy"
	req, err := http.NewRequest("GET", u.String(), nil)
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
		log.Fatalf("Not able to parse digger cloud url: %v", err)
	}
	u.Path = "/orgs/" + organisation + "/plan-policy"
	req, err := http.NewRequest("GET", u.String(), nil)
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
		log.Fatalf("Not able to parse digger cloud url: %v", err)
	}
	u.Path = "/orgs/" + organisation + "/drift-policy"
	req, err := http.NewRequest("GET", u.String(), nil)
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

func getAccessPolicyForNamespace(p *DiggerHttpPolicyProvider, namespace string, projectName string) (string, *http.Response, error) {
	// fetch RBAC policies for project from Digger API
	u, err := url.Parse(p.DiggerHost)
	if err != nil {
		log.Fatalf("Not able to parse digger cloud url: %v", err)
	}
	u.Path = "/repos/" + namespace + "/projects/" + projectName + "/access-policy"
	req, err := http.NewRequest("GET", u.String(), nil)

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

func getPlanPolicyForNamespace(p *DiggerHttpPolicyProvider, namespace string, projectName string) (string, *http.Response, error) {
	u, err := url.Parse(p.DiggerHost)
	if err != nil {
		log.Fatalf("Not able to parse digger cloud url: %v", err)
	}
	u.Path = "/repos/" + namespace + "/projects/" + projectName + "/plan-policy"
	req, err := http.NewRequest("GET", u.String(), nil)

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
func (p *DiggerHttpPolicyProvider) GetAccessPolicy(organisation string, repo string, projectName string) (string, error) {
	namespace := fmt.Sprintf("%v-%v", organisation, repo)
	content, resp, err := getAccessPolicyForNamespace(p, namespace, projectName)
	if err != nil {
		return "", err
	}

	// project policy found
	if resp.StatusCode == 200 && content != "" {
		return content, nil
	}

	// check if project policy was empty or not found (retrieve org policy if so)
	if (resp.StatusCode == 200 && content == "") || resp.StatusCode == 404 {
		content, resp, err := getAccessPolicyForOrganisation(p)
		if err != nil {
			return "", err
		}
		if resp.StatusCode == 200 {
			return content, nil
		} else if resp.StatusCode == 404 {
			return "", nil
		} else {
			return "", errors.New(fmt.Sprintf("unexpected response while fetching organisation policy: %v, code %v", content, resp.StatusCode))
		}
	} else {
		return "", errors.New(fmt.Sprintf("unexpected response while fetching project policy: %v code %v", content, resp.StatusCode))
	}
}

func (p *DiggerHttpPolicyProvider) GetPlanPolicy(organisation string, repo string, projectName string) (string, error) {
	namespace := fmt.Sprintf("%v-%v", organisation, repo)
	content, resp, err := getPlanPolicyForNamespace(p, namespace, projectName)
	if err != nil {
		return "", err
	}

	// project policy found
	if resp.StatusCode == 200 && content != "" {
		return content, nil
	}

	// check if project policy was empty or not found (retrieve org policy if so)
	if (resp.StatusCode == 200 && content == "") || resp.StatusCode == 404 {
		content, resp, err := getPlanPolicyForOrganisation(p)
		if err != nil {
			return "", err
		}
		if resp.StatusCode == 200 {
			return content, nil
		} else if resp.StatusCode == 404 {
			return "", nil
		} else {
			return "", errors.New(fmt.Sprintf("unexpected response while fetching organisation policy: %v, code %v", content, resp.StatusCode))
		}
	} else {
		return "", errors.New(fmt.Sprintf("unexpected response while fetching project policy: %v code %v", content, resp.StatusCode))
	}
}

func (p *DiggerHttpPolicyProvider) GetDriftPolicy() (string, error) {
	content, resp, err := getDriftPolicyForOrganisation(p)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == 200 {
		return content, nil
	} else if resp.StatusCode == 404 {
		return "", nil
	} else {
		return "", errors.New(fmt.Sprintf("unexpected response while fetching organisation policy: %v, code %v", content, resp.StatusCode))
	}
}

func (p *DiggerHttpPolicyProvider) GetOrganisation() string {
	return p.DiggerOrganisation
}

type DiggerPolicyChecker struct {
	PolicyProvider policy.Provider
}

func (p DiggerPolicyChecker) CheckAccessPolicy(ciService ci.OrgService, SCMOrganisation string, SCMrepository string, projectName string, command string, requestedBy string) (bool, error) {

	policy, err := p.PolicyProvider.GetAccessPolicy(SCMOrganisation, SCMrepository, projectName)

	if err != nil {
		fmt.Printf("Error while fetching policy: %v", err)
		return false, err
	}

	teams, err := ciService.GetUserTeams(SCMOrganisation, requestedBy)
	if err != nil {
		fmt.Printf("Error while fetching user teams for CI service: %v", err)
		return false, err
	}

	input := map[string]interface{}{
		"user":         requestedBy,
		"organisation": SCMOrganisation,
		"teams":        teams,
		"action":       command,
		"project":      projectName,
	}

	if policy == "" {
		return true, nil
	}

	ctx := context.Background()
	fmt.Printf("DEBUG: passing the following input policy: %v ||| text: %v", input, policy)
	query, err := rego.New(
		rego.Query("data.digger.allow"),
		rego.Module("digger", policy),
	).PrepareForEval(ctx)

	if err != nil {
		return false, err
	}

	results, err := query.Eval(ctx, rego.EvalInput(input))
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return false, fmt.Errorf("no result found")
	}

	expressions := results[0].Expressions

	for _, expression := range expressions {
		decision, ok := expression.Value.(bool)
		if !ok {
			return false, fmt.Errorf("decision is not a boolean")
		}
		if !decision {
			return false, nil
		}
	}

	return true, nil
}

func (p DiggerPolicyChecker) CheckPlanPolicy(SCMrepository string, projectName string, planOutput string) (bool, []string, error) {
	// TODO: Get rid of organisation if its not needed
	organisation := p.PolicyProvider.GetOrganisation()
	policy, err := p.PolicyProvider.GetPlanPolicy(organisation, SCMrepository, projectName)
	if err != nil {
		return false, nil, fmt.Errorf("failed get plan policy: %v", err)
	}
	var parsedPlanOutput map[string]interface{}
	err = json.Unmarshal([]byte(planOutput), &parsedPlanOutput)
	if err != nil {
		return false, nil, fmt.Errorf("failed to parse json terraform output to map: %v", err)
	}

	input := map[string]interface{}{
		"terraform": parsedPlanOutput,
	}

	if policy == "" {
		fmt.Printf("No plan policies found, succeeding")
		return true, nil, nil
	}

	ctx := context.Background()
	fmt.Printf("DEBUG: passing the following input policy: %v ||| text: %v", input, policy)
	query, err := rego.New(
		rego.Query("data.digger.deny"),
		rego.Module("digger", policy),
	).PrepareForEval(ctx)

	if err != nil {
		return false, nil, err
	}

	results, err := query.Eval(ctx, rego.EvalInput(input))
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return false, nil, fmt.Errorf("no result found")
	}

	expressions := results[0].Expressions

	decisionsResult := make([]string, 0)
	for _, expression := range expressions {
		decisions, ok := expression.Value.([]interface{})

		if !ok {
			return false, nil, fmt.Errorf("decision is not a slice of interfaces")
		}
		if len(decisions) > 0 {
			for _, d := range decisions {
				decisionsResult = append(decisionsResult, d.(string))
				fmt.Printf("denied: %v\n", d)
			}

		}

	}

	if len(decisionsResult) > 0 {
		return false, decisionsResult, nil
	}

	return true, []string{}, nil
}

func (p DiggerPolicyChecker) CheckDriftPolicy(SCMOrganisation string, SCMrepository string, projectName string) (bool, error) {
	// TODO: Get rid of organisation if its not needed
	//organisation := p.PolicyProvider.GetOrganisation()
	policy, err := p.PolicyProvider.GetDriftPolicy()
	if err != nil {
		fmt.Printf("Error while fetching drift policy: %v", err)
		return false, err
	}

	input := map[string]interface{}{
		"organisation": SCMOrganisation,
		"project":      projectName,
	}

	if policy == "" {
		return true, nil
	}

	ctx := context.Background()
	fmt.Printf("DEBUG: passing the following input policy: %v ||| text: %v", input, policy)
	query, err := rego.New(
		rego.Query("data.digger.enable"),
		rego.Module("digger", policy),
	).PrepareForEval(ctx)

	if err != nil {
		return false, err
	}

	results, err := query.Eval(ctx, rego.EvalInput(input))
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return false, fmt.Errorf("no result found")
	}

	expressions := results[0].Expressions

	for _, expression := range expressions {
		decision, ok := expression.Value.(bool)
		if !ok {
			return false, fmt.Errorf("decision is not a boolean")
		}
		if !decision {
			return false, nil
		}
	}

	return true, nil
}
