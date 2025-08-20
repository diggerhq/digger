package github

import (
	"context"
	"log"
	"log/slog"

	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/libs/ci"
	next_utils "github.com/diggerhq/digger/next/utils"
	"github.com/google/go-github/v61/github"
)

type DiggerController struct {
	CiBackendProvider    ci_backends.CiBackendProvider
	GithubClientProvider next_utils.GithubClientProvider
}

func (d DiggerController) ListRepos() ([]*ci.Repo, error) {
	allRepos := make([]*ci.Repo, 0)
	//err := c.BindJSON(&request)
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		opt := &github.ListOptions{Page: opts.Page, PerPage: 100}
		client, _, err := d.GithubClientProvider.Get(10, 10)
		if err != nil {
			log.Printf("Error retrieving github client: %v", err)
			//c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching organisation"})
		}
		listRepos, resp, err := client.Apps.ListRepos(context.Background(), opt)
		if err != nil {
			slog.Error("Failed to list existing repositories",
				"installationId",
				"error", err,
			)
			//c.JSON(http.StatusInternalServerError, "Failed to list existing repos: %v", err)
		}
		repos := listRepos.Repositories
		for _, repo := range repos {
			//if repo != nil {
			//	// this is an pull request, skip
			//	continue
			//}
			allRepos = append(allRepos, &ci.Repo{ID: int64(*repo.ID), Title: *repo.FullName})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allRepos, nil
}
