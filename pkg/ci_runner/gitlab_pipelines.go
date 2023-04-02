package ci_runner

import (
	"digger/pkg/domain"
	"fmt"
)

type GitlabPipelines struct{}

func (gp *GitlabPipelines) CurrentEvent() (*domain.Event, error) {
	return nil, fmt.Errorf("not implemented yet")
}
