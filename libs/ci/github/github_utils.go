package github

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v61/github"
)

func ListGithubRepos(client *github.Client) ([]*github.Repository, error) {
	allRepos := make([]*github.Repository, 0)
	//err := c.BindJSON(&request)
	opts := &github.ListOptions{PerPage: 100}

	countLimit := 0
	for {
		listRepos, resp, err := client.Apps.ListRepos(context.Background(), opts)
		if err != nil {
			// Check specifically for rate limit errors
			if _, ok := err.(*github.RateLimitError); ok {
				slog.Error("GitHub API rate limit exceeded",
					"error", err,
				)
				// Wait and retry after a delay or return a specific error
				// For now, we'll just return with the rate limit error
				return nil, err
			}
			slog.Error("Failed to list existing repositories",
				"error", err,
			)
			return nil, err
		}
		allRepos = append(allRepos, listRepos.Repositories...)
		countLimit++
		if countLimit == 20 {
			slog.Error("Exceeded maximum number of existing repositories")
			return nil, fmt.Errorf("exceeded maximum number of existing repositories")
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allRepos, nil
}
