package ci_runner

import (
	"digger/pkg/domain"
	"fmt"
)

type Jenkins struct{}

func (j *Jenkins) CurrentEvent() (*domain.Event, error) {
	return nil, fmt.Errorf("not implemented yet")
}
