package controllers

import (
	"log/slog"

	"github.com/diggerhq/digger/backend/models"
	"github.com/google/go-github/v61/github"
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

func handleInstallationRepositoriesEvent(event *github.InstallationRepositoriesEvent, appId int64) error {
	installationId := *event.Installation.ID
	action := *event.Action

	slog.Info("Handling installation repositories event",
		"installationId", installationId,
		"appId", appId,
		"action", action,
		"repositoriesAdded", len(event.RepositoriesAdded),
		"repositoriesRemoved", len(event.RepositoriesRemoved),
	)

	// Handle removed repositories
	for _, repo := range event.RepositoriesRemoved {
		repoFullName := *repo.FullName
		slog.Info("Removing repository from installation",
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

	// Handle added repositories
	for _, repo := range event.RepositoriesAdded {
		repoFullName := *repo.FullName
		slog.Info("Adding repository to installation",
			"installationId", installationId,
			"repoFullName", repoFullName,
		)

		_, err := models.DB.GithubRepoAdded(
			installationId,
			appId,
			*event.Installation.Account.Login,
			*event.Installation.Account.ID,
			repoFullName,
		)
		if err != nil {
			slog.Error("Error adding GitHub repo",
				"installationId", installationId,
				"repoFullName", repoFullName,
				"error", err,
			)
			return err
		}
	}

	slog.Info("Successfully handled installation repositories event",
		"installationId", installationId,
		"repositoriesAdded", len(event.RepositoriesAdded),
		"repositoriesRemoved", len(event.RepositoriesRemoved),
	)

	return nil
}
