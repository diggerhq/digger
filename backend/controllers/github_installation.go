package controllers

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/google/go-github/v61/github"
	"log/slog"
)

func handleInstallationDeletedEvent(installation *github.InstallationEvent, appId int64) error {
	installationId := *installation.Installation.ID

	slog.Info("Handling installation deleted event",
		"installationId", installationId,
		"appId", appId,
	)

	link, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		slog.Error("Error getting installation link", "installationId", installationId, "error", err)
		return err
	}

	_, err = models.DB.MakeGithubAppInstallationLinkInactive(link)
	if err != nil {
		slog.Error("Error making installation link inactive", "installationId", installationId, "error", err)
		return err
	}

	for _, repo := range installation.Repositories {
		repoFullName := *repo.FullName
		slog.Info("Removing installation for repo",
			"installationId", installationId,
			"repoFullName", repoFullName,
		)

		_, err := models.DB.GithubRepoRemoved(installationId, appId, repoFullName)
		if err != nil {
			slog.Error("Error removing GitHub repo",
				"installationId", installationId,
				"repoFullName", repoFullName,
				"error", err,
			)
			return err
		}
	}

	slog.Info("Successfully handled installation deleted event", "installationId", installationId)
	return nil
}
