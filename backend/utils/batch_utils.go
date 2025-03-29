package utils

import (
	"fmt"
	"log/slog"

	"github.com/diggerhq/digger/backend/models"
)

func PostCommentForBatch(batch *models.DiggerBatch, comment string, githubClientProvider GithubClientProvider) error {
	slog.Debug("Posting comment for batch",
		"batchId", batch.ID,
		"vcs", batch.VCS,
		"repo", batch.RepoFullName,
		"prNumber", batch.PrNumber,
		"commentLength", len(comment),
	)

	// todo: perform for rest of vcs as well
	if batch.VCS == models.DiggerVCSGithub {
		ghService, _, err := GetGithubService(githubClientProvider, batch.GithubInstallationId, batch.RepoFullName, batch.RepoOwner, batch.RepoName)
		if err != nil {
			slog.Error("Error getting GitHub service",
				"batchId", batch.ID,
				"installationId", batch.GithubInstallationId,
				"repo", batch.RepoFullName,
				"error", err,
			)
			return fmt.Errorf("error getting ghService: %v", err)
		}

		_, err = ghService.PublishComment(batch.PrNumber, comment)
		if err != nil {
			slog.Error("Error publishing comment",
				"batchId", batch.ID,
				"prNumber", batch.PrNumber,
				"repo", batch.RepoFullName,
				"error", err,
			)
			return fmt.Errorf("error publishing comment (%v): %v", comment, err)
		}

		slog.Info("Successfully posted comment",
			"batchId", batch.ID,
			"prNumber", batch.PrNumber,
			"repo", batch.RepoFullName,
		)
		return nil
	}

	slog.Warn("Unknown VCS type, comment not posted",
		"batchId", batch.ID,
		"vcs", batch.VCS,
		"repo", batch.RepoFullName,
		"prNumber", batch.PrNumber,
	)
	return nil
}
