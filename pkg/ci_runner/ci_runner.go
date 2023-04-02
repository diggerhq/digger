package ci_runner

import "digger/pkg/domain"

func Current() domain.CIRunner {
	// For now we only return github actions
	return &GithubActions{}
}
