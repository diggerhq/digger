package controllers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/google/go-github/v61/github"
)

func getAccountDetails(account *github.User) (string, int64) {
	if account == nil {
		return "", 0
	}
	return account.GetLogin(), int64(account.GetID())
}

// fetchRepoIdentifiers returns repo identifiers and fills missing branch/clone URL by calling GitHub if needed.
func fetchRepoIdentifiers(ctx context.Context, client *github.Client, repo *github.Repository, installationId int64) (repoFullName, owner, name, defaultBranch, cloneURL string, err error) {
	repoFullName = repo.GetFullName()
	if repo.Owner != nil {
		owner = repo.Owner.GetLogin()
	}
	name = repo.GetName()
	defaultBranch = repo.GetDefaultBranch()
	cloneURL = repo.GetCloneURL()

	if repoFullName == "" && owner != "" && name != "" {
		repoFullName = fmt.Sprintf("%s/%s", owner, name)
	}

	if (defaultBranch == "" || cloneURL == "") && owner != "" && name != "" {
		repoDetails, _, fetchErr := client.Repositories.Get(ctx, owner, name)
		if fetchErr != nil {
			slog.Error("Error fetching repo details",
				"installationId", installationId,
				"repoOwner", owner,
				"repoName", name,
				"error", fetchErr)
			return repoFullName, owner, name, defaultBranch, cloneURL, fetchErr
		}
		if defaultBranch == "" {
			defaultBranch = repoDetails.GetDefaultBranch()
		}
		if cloneURL == "" {
			cloneURL = repoDetails.GetCloneURL()
		}
	}

	return repoFullName, owner, name, defaultBranch, cloneURL, nil
}

func upsertRepo(ctx context.Context, ghClient *github.Client, repo *github.Repository, installationId int64, appId int64, accountLogin string, accountId int64) error {
	repoFullName, owner, name, defaultBranch, cloneURL, err := fetchRepoIdentifiers(ctx, ghClient, repo, installationId)
	if err != nil {
		return err
	}
	if repoFullName == "" || owner == "" || name == "" {
		slog.Warn("Skipping repo with missing identifiers",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"owner", owner,
			"name", name,
		)
		return nil
	}

	if _, err := models.DB.GithubRepoAdded(installationId, appId, accountLogin, accountId, repoFullName); err != nil {
		slog.Error("Error recording GitHub repository",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", err)
		return err
	}

	repoUrl := fmt.Sprintf("https://%s/%s", utils.GetGithubHostname(), repoFullName)
	if _, _, err := createOrGetDiggerRepoForGithubRepo(repoFullName, owner, name, repoUrl, installationId, appId, defaultBranch, cloneURL); err != nil {
		slog.Error("Error creating or getting Digger repo",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", err)
		return err
	}

	return nil
}

func removeRepo(ctx context.Context, repo *github.Repository, installationId int64, appId int64, orgId uint) error {
	repoFullName := repo.GetFullName()
	if repoFullName == "" {
		slog.Warn("Skipping repo removal with empty full name", "installationId", installationId)
		return nil
	}

	if _, err := models.DB.GithubRepoRemoved(installationId, appId, repoFullName); err != nil {
		slog.Error("Error marking GitHub repo removed",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", err)
		return err
	}

	if err := models.DB.SoftDeleteRepoAndProjects(orgId, repoFullName); err != nil {
		slog.Error("Error soft deleting repo and projects on remove",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"orgId", orgId,
			"error", err)
		return err
	}

	return nil
}

func handleInstallationDeletedEvent(installation *github.InstallationEvent, appId int64) error {
	installationId := installation.Installation.GetID()

	slog.Info("Handling installation deleted event",
		"installationId", installationId,
		"appId", appId,
	)

	link, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		slog.Error("Error getting installation link", "installationId", installationId, "error", err)
		return err
	}

	if link == nil {
		slog.Error("Installation link not found for deletion", "installationId", installationId)
		return nil
	}

	if _, err = models.DB.MakeGithubAppInstallationLinkInactive(link); err != nil {
		slog.Error("Error making installation link inactive", "installationId", installationId, "error", err)
		return err
	}

	if err := models.DB.GormDB.Model(&models.GithubAppInstallation{}).Where("github_installation_id = ?", installationId).Update("status", models.GithubAppInstallDeleted).Error; err != nil {
		slog.Error("Error marking installations deleted", "installationId", installationId, "error", err)
		return err
	}

	if err := models.DB.SoftDeleteReposAndProjectsByInstallation(link.OrganisationId, installationId); err != nil {
		slog.Error("Error soft deleting repos/projects for installation", "installationId", installationId, "orgId", link.OrganisationId, "error", err)
		return err
	}

	for _, repo := range installation.Repositories {
		if err := removeRepo(context.Background(), repo, installationId, appId, link.OrganisationId); err != nil {
			return err
		}
	}

	slog.Info("Successfully handled installation deleted event", "installationId", installationId)
	return nil
}

func handleInstallationUpsertEvent(ctx context.Context, gh utils.GithubClientProvider, installation *github.InstallationEvent, appId int64) error {
	installationId := installation.Installation.GetID()
	appIdFromPayload := appId
	if installation.Installation.AppID != nil {
		appIdFromPayload = installation.Installation.GetAppID()
	}

	accountLogin, accountId := getAccountDetails(installation.Installation.Account)

	link, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		slog.Error("Error getting installation link", "installationId", installationId, "error", err)
		return err
	}

	if link == nil {
		slog.Error("Installation link not found for upsert", "installationId", installationId)
		return nil
	}

	repoList := installation.Repositories
	if len(repoList) == 0 {
		slog.Warn("No repositories found to sync for installation", "installationId", installationId)
		return nil
	}

	slog.Info("Syncing repositories for installation",
		"installationId", installationId,
		"appId", appIdFromPayload,
		"repoCount", len(repoList),
	)

	if err := models.DB.GormDB.Model(&models.GithubAppInstallation{}).Where("github_installation_id = ?", installationId).Update("status", models.GithubAppInstallDeleted).Error; err != nil {
		slog.Error("Error marking installations deleted prior to resync", "installationId", installationId, "error", err)
		return err
	}

	if err := models.DB.SoftDeleteReposAndProjectsByInstallation(link.OrganisationId, installationId); err != nil {
		slog.Error("Error soft deleting existing repos/projects prior to resync", "installationId", installationId, "orgId", link.OrganisationId, "error", err)
		return err
	}

	ghClient, _, err := gh.Get(appIdFromPayload, installationId)
	if err != nil {
		slog.Error("Error creating GitHub client for repo sync", "installationId", installationId, "error", err)
		return err
	}

	for _, repo := range repoList {
		if err := upsertRepo(ctx, ghClient, repo, installationId, appIdFromPayload, accountLogin, accountId); err != nil {
			return err
		}
	}

	slog.Info("Successfully synced repositories for installation", "installationId", installationId)
	return nil
}

func handleInstallationRepositoriesEvent(ctx context.Context, gh utils.GithubClientProvider, event *github.InstallationRepositoriesEvent, appId int64) error {
	installationId := event.Installation.GetID()
	appIdFromPayload := appId
	if event.Installation.AppID != nil {
		appIdFromPayload = event.Installation.GetAppID()
	}

	accountLogin, accountId := getAccountDetails(event.Installation.Account)

	link, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		slog.Error("Error getting installation link", "installationId", installationId, "error", err)
		return err
	}

	if link == nil {
		slog.Error("Installation link not found for installation_repositories event", "installationId", installationId)
		return nil
	}

	client, _, err := gh.Get(appIdFromPayload, installationId)
	if err != nil {
		slog.Error("Error creating GitHub client for installation_repositories event", "installationId", installationId, "error", err)
		return err
	}

	var errs []error
	for _, repo := range event.RepositoriesAdded {
		if err := upsertRepo(ctx, client, repo, installationId, appIdFromPayload, accountLogin, accountId); err != nil {
			errs = append(errs, err)
		}
	}

	for _, repo := range event.RepositoriesRemoved {
		if err := removeRepo(ctx, repo, installationId, appIdFromPayload, link.OrganisationId); err != nil {
			errs = append(errs, err)
		}
	}

	slog.Info("Handled installation_repositories event",
		"installationId", installationId,
		"addedCount", len(event.RepositoriesAdded),
		"removedCount", len(event.RepositoriesRemoved),
	)
	if len(errs) > 0 {
		return fmt.Errorf("one or more errors during installation_repositories handling: %v", errs)
	}
	return nil
}
