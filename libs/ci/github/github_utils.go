package github

import (
	"context"
	"log/slog"

	"github.com/google/go-github/v61/github"
)

func ListGithubRepos(client *github.Client) ([]*github.Repository, error) {
	allRepos := make([]*github.Repository, 0)
	//err := c.BindJSON(&request)
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		opt := &github.ListOptions{Page: opts.Page, PerPage: 100}
		listRepos, resp, err := client.Apps.ListRepos(context.Background(), opt)
		if err != nil {
			slog.Error("Failed to list existing repositories",
				"error", err,
			)
			return nil, err
		}
		allRepos = append(allRepos, listRepos.Repositories...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allRepos, nil
}
