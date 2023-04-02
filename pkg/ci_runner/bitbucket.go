package ci_runner

import (
	"digger/pkg/domain"
	"fmt"
)

type Bitbucket struct{}

func (bb *Bitbucket) CurrentEvent() (*domain.Event, error) {
	return nil, fmt.Errorf("not implemented yet")
}
