package scm

import "digger/pkg/domain"

func GetProvider() (domain.SCMProvider, error) {
	return &Github{}, nil
}
