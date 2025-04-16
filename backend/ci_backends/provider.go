package ci_backends

import (
	"fmt"
	"log/slog"

	"github.com/diggerhq/digger/backend/utils"
)

type CiBackendProvider interface {
	GetCiBackend(options CiBackendOptions) (CiBackend, error)
}

type DefaultBackendProvider struct{}

func (d DefaultBackendProvider) GetCiBackend(options CiBackendOptions) (CiBackend, error) {
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
