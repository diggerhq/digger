package policy

import (
	"fmt"
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	"github.com/diggerhq/digger/cli/pkg/policy"
	"os"
)

type PolicyCheckerProviderAdvanced struct{}

func (p PolicyCheckerProviderAdvanced) Get(hostname string, organisationName string, authToken string) (core_policy.Checker, error) {
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
