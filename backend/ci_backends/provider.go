package ci_backends

import (
	"fmt"
	"github.com/diggerhq/digger/backend/utils"
	"log"
)

type CiBackendProvider interface {
	GetCiBackend(options CiBackendOptions) (CiBackend, error)
}

type DefaultBackendProvider struct{}

func (d DefaultBackendProvider) GetCiBackend(options CiBackendOptions) (CiBackend, error) {
	client, _, err := utils.GetGithubClient(options.GithubClientProvider, options.GithubInstallationId, options.RepoFullName)
	if err != nil {
		log.Printf("GetCiBackend: could not get github client: %v", err)
		return nil, fmt.Errorf("could not get github client: %v", err)
	}
	backend := &GithubActionCi{
		Client: client,
	}
	return backend, nil
}
