package policy

import (
	"log"
	"net/http"
	"os"
)

type PolicyCheckerProviderBasic struct{}

func (p PolicyCheckerProviderBasic) Get(hostname string, organisationName string, authToken string) (Checker, error) {
	var policyChecker Checker
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
