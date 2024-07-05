package policy

import (
	"fmt"
	"github.com/diggerhq/digger/libs/policy"
	lib_spec "github.com/diggerhq/digger/libs/spec"
	"os"
)

type AdvancedPolicyProvider struct{}

func (p AdvancedPolicyProvider) GetPolicyProvider(policySpec lib_spec.PolicySpec, diggerHost string, diggerOrg string, token string) (policy.Checker, error) {
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

	checker, err := lib_spec.BasicPolicyProvider{}.GetPolicyProvider(policySpec, diggerHost, diggerOrg, token)
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
