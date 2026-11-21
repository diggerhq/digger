package ci_backends

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/diggerhq/digger/backend/utils"
)

type CiBackendProvider interface {
	GetCiBackend(options CiBackendOptions) (CiBackend, error)
}

type DefaultBackendProvider struct{}

func (d DefaultBackendProvider) GetCiBackend(options CiBackendOptions) (CiBackend, error) {
	backendType := os.Getenv("DIGGER_CI_BACKEND")
	if backendType == "azure_devops" {
		slog.Info("Using Azure DevOps CI backend")
		ci, err := NewAzureDevOpsCi()
		if err != nil {
			slog.Error("GetCiBackend: could not initialize Azure DevOps client", "error", err)
			return nil, fmt.Errorf("could not initialize Azure DevOps client: %v", err)
		}
		return ci, nil
	}

	client, _, err := utils.GetGithubClientFromAppId(options.GithubClientProvider, options.GithubInstallationId, options.GithubAppId, options.RepoFullName)
	if err != nil {
		slog.Error("GetCiBackend: could not get github client", "error", err)
		return nil, fmt.Errorf("could not get github client: %v", err)
	}
	backend := &GithubActionCi{
		Client: client,
	}
	return backend, nil
}
