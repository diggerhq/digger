package policy

import (
	core_policy "github.com/diggerhq/digger/cli/pkg/core/policy"
	"log"
	"net/http"
	"os"
)

type PolicyCheckerProviderBasic struct{}

func (p PolicyCheckerProviderBasic) Get(hostname string, organisationName string, authToken string) (core_policy.Checker, error) {
	var policyChecker core_policy.Checker
	if os.Getenv("NO_BACKEND") == "true" {
		log.Println("WARNING: running in 'backendless' mode. No policies will be supported.")
		policyChecker = NoOpPolicyChecker{}
	} else {
		policyChecker = DiggerPolicyChecker{
			PolicyProvider: &DiggerHttpPolicyProvider{
				DiggerHost:         hostname,
				DiggerOrganisation: organisationName,
				AuthToken:          authToken,
				HttpClient:         http.DefaultClient,
			}}
	}
	return policyChecker, nil
}
