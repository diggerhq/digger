package policy

import (
	"fmt"
	"github.com/diggerhq/digger/libs/policy"
	lib_spec "github.com/diggerhq/digger/libs/spec"
	"log"
	"os"
)

type AdvancedPolicyProvider struct{}

func (p AdvancedPolicyProvider) GetPolicyProvider(policySpec lib_spec.PolicySpec, diggerHost string, diggerOrg string, token string, vcsType string) (policy.Checker, error) {
	managementRepo := os.Getenv("DIGGER_MANAGEMENT_REPO")
	if managementRepo != "" {
		log.Printf("info: using management repo policy provider")
		var token = ""
		var tokenName = ""
		switch vcsType {
		case "github":
			token = os.Getenv("GITHUB_TOKEN")
			tokenName = "GITHUB_TOKEN"
		case "gitlab":
			token = os.Getenv("GITLAB_TOKEN")
			tokenName = "GITLAB_TOKEN"
		default:
			token = os.Getenv("GITHUB_TOKEN")
			tokenName = "GITHUB_TOKEN"
		}

		if token == "" {
			return nil, fmt.Errorf("failed to get managent repo policy provider: %v not specified", tokenName)
		}
		return policy.DiggerPolicyChecker{
			PolicyProvider: DiggerRepoPolicyProvider{
				ManagementRepoUrl: managementRepo,
				GitToken:          token,
			},
		}, nil
	}

	checker, err := lib_spec.BasicPolicyProvider{}.GetPolicyProvider(policySpec, diggerHost, diggerOrg, token, "")
	return checker, err
}

type PolicyCheckerProviderAdvanced struct{}

func (p PolicyCheckerProviderAdvanced) Get(hostname string, organisationName string, authToken string) (policy.Checker, error) {
	managementRepo := os.Getenv("DIGGER_MANAGEMENT_REPO")
	if managementRepo != "" {
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("failed to get managent repo policy provider: GITHUB_TOKEN not specified")
		}
		return policy.DiggerPolicyChecker{
			PolicyProvider: DiggerRepoPolicyProvider{
				ManagementRepoUrl: managementRepo,
				GitToken:          token,
			},
		}, nil
	}
	return policy.PolicyCheckerProviderBasic{}.Get(hostname, organisationName, authToken)
}
