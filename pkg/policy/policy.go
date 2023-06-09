package policy

import (
	"context"
	"fmt"
	"github.com/open-policy-agent/opa/rego"
	"io/ioutil"
	"net/http"
)

type PolicyProvider interface {
	GetPolicy() (string, error)
}

type DiggerHttpPolicyProvider struct {
	DiggerHost string
	AuthToken  string
	HttpClient *http.Client
}

func (p *DiggerHttpPolicyProvider) GetPolicy() (string, error) {
	// fetch RBAC policies from Digger API
	req, err := http.NewRequest("GET", p.DiggerHost+"/policies", nil)
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

func (p *DiggerPolicyChecker) Check(input interface{}) (bool, []string, error) {
	policy, err := p.PolicyProvider.GetPolicy()
	if err != nil {
		return false, nil, err
	}
	ctx := context.Background()
	// Create a new query that uses the compiled policy from above.
	query, err := rego.New(
		rego.Query("data.digger.allow"),
		rego.Module("digger", policy),
	).PrepareForEval(ctx)

	if err != nil {
		// handle error
		return false, nil, err
	}

	results, err := query.Eval(ctx, rego.EvalInput(input))
	// Process result
	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return false, nil, fmt.Errorf("no result found")
	}

	// Assuming the decision is a boolean
	expressions := results[0].Expressions

	allTrue := true
	messages := make([]string, 0)
	for _, expression := range expressions {
		decision, ok := expression.Value.(bool)
		if !ok {
			return false, nil, fmt.Errorf("decision is not a boolean")
		}
		if !decision {
			messages = append(messages, expression.Text)
		}
		allTrue = allTrue && decision
	}

	return allTrue, messages, nil
}
