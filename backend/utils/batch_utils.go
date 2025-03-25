package utils

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"log"
)

func PostCommentForBatch(batch *models.DiggerBatch, comment string, githubClientProvider GithubClientProvider) error {
	// todo: perform for rest of vcs as well
	if batch.VCS == models.DiggerVCSGithub {
		ghService, _, err := GetGithubService(githubClientProvider, batch.GithubInstallationId, batch.RepoFullName, batch.RepoOwner, batch.RepoName)
		if err != nil {
			log.Printf("error getting ghService: %v", err)
			return fmt.Errorf("error getting ghService: %v", err)
		}
		_, err = ghService.PublishComment(batch.PrNumber, comment)
		if err != nil {
			log.Printf("error publishing comment (%v): %v", comment, err)
			return fmt.Errorf("error publishing comment (%v): %v", comment, err)
		}
		return nil
	}
	log.Printf("Warning: Unknown vcs type: %v", batch.VCS)
	return nil
}
