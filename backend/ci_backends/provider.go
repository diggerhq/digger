package ci_backends

import (
	"fmt"
	"log"

	"github.com/go-substrate/strate/backend/utils"
)

type CiBackendProvider interface {
	GetCiBackend(options CiBackendOptions) (CiBackend, error)
}

type DefaultBackendProvider struct{}

func (d DefaultBackendProvider) GetCiBackend(options CiBackendOptions) (CiBackend, error) {
	client, _, err := utils.GetGithubClientFromAppId(options.GithubClientProvider, options.GithubInstallationId, options.GithubAppId, options.RepoFullName)
	if err != nil {
		log.Printf("GetCiBackend: could not get github client: %v", err)
		return nil, fmt.Errorf("could not get github client: %v", err)
	}
	backend := &GithubActionCi{
		Client: client,
	}
	return backend, nil
}
