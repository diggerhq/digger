package github

func ListRepos() ([]*ci.Repo, error) {
	allRepos := make([]*ci.Repo, 0)
	opts := &github.RepoListByRepoOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		opt := &github.ListOptions{Page: 1, PerPage: 100}
		listRepos, _, err := client.Apps.ListRepos(context.Background(), opt)
		if err != nil {
			slog.Error("Failed to list existing repositories",
				"installationId", installationId64,
				"error", err,
			)
			c.String(http.StatusInternalServerError, "Failed to list existing repos: %v", err)
			return
		}
		repos := listRepos.Repositories
		for _, repo := range repos {
			if repo.PullRequestLinks != nil {
				// this is an pull request, skip
				continue
			}

			allRepos = append(allRepos, &ci.Repos{ID: int64(*repo.Number), Title: *repo.Title, Body: *repo.Body})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allRepos, nil
}