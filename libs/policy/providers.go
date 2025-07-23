package policy

import (
	"log/slog"
	"net/http"
	"os"
)

type PolicyCheckerProviderBasic struct{}

func (p PolicyCheckerProviderBasic) Get(hostname, organisationName, authToken string) (Checker, error) {
	var policyChecker Checker
	if os.Getenv("NO_BACKEND") == "true" {
		slog.Warn("Running in 'backendless' mode. No policies will be supported.")
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
	return policyChecker, nil
}
