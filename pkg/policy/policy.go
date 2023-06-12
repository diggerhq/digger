package policy

import (
	"context"
	"fmt"
	"github.com/open-policy-agent/opa/rego"
	"io/ioutil"
	"net/http"
)

type PolicyProvider interface {
	GetPolicy(namespace string, projectname string) (string, error)
}

type DiggerHttpPolicyProvider struct {
	DiggerHost string
	AuthToken  string
	HttpClient *http.Client
}

type NoOpPolicyChecker struct {
}

func (p NoOpPolicyChecker) Check(_ string, _ string, _ interface{}) (bool, error) {
	return true, nil
}

func (p *DiggerHttpPolicyProvider) GetPolicy(namespace string, projectName string) (string, error) {
	// fetch RBAC policies from Digger API
	req, err := http.NewRequest("GET", p.DiggerHost+"/repos/"+namespace+"/projects/"+projectName+"/policies", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", "Bearer "+p.AuthToken)

	resp, err := p.HttpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

type DiggerPolicyChecker struct {
	PolicyProvider PolicyProvider
}

func (p DiggerPolicyChecker) Check(namespace string, projectName string, input interface{}) (bool, error) {
	policy, err := p.PolicyProvider.GetPolicy(namespace, projectName)
	if err != nil {
		return false, err
	}
	ctx := context.Background()
	// Create a new query that uses the compiled policy from above.
	query, err := rego.New(
		rego.Query("data.digger.allow"),
		rego.Module("digger", policy),
	).PrepareForEval(ctx)

	if err != nil {
		// handle error
		return false, err
	}

	results, err := query.Eval(ctx, rego.EvalInput(input))
	// Process result
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return false, fmt.Errorf("no result found")
	}

	// Assuming the decision is a boolean
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
