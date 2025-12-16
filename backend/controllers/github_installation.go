package controllers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/google/go-github/v61/github"
	"github.com/sethvargo/go-retry"
)

// getAccountDetails safely extracts login and account ID from a GitHub user.
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

	// If owner is missing but we have fullName, parse it from there
	// This handles webhook payloads that don't include the full Owner object
	if owner == "" && repoFullName != "" {
		parts := strings.Split(repoFullName, "/")
		if len(parts) == 2 {
			owner = parts[0]
			if name == "" {
				name = parts[1]
			}
		}
	}

	if repoFullName == "" && owner != "" && name != "" {
		repoFullName = fmt.Sprintf("%s/%s", owner, name)
	}

	// If we don't have full repo details (defaultBranch or cloneURL), fetch them from GitHub
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

// upsertRepo creates or restores a repo and its GithubAppInstallation record.
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

// removeRepo marks a repo as removed and soft-deletes it along with its projects.
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

	// Mark all GithubAppInstallation records as deleted
	if err := models.DB.GormDB.Model(&models.GithubAppInstallation{}).Where("github_installation_id = ?", installationId).Update("status", models.GithubAppInstallDeleted).Error; err != nil {
		slog.Error("Error marking installations deleted", "installationId", installationId, "error", err)
		return err
	}

	// Soft-delete all repos and projects for this installation
	if err := models.DB.SoftDeleteReposAndProjectsByInstallation(link.OrganisationId, installationId); err != nil {
		slog.Error("Error soft deleting repos/projects for installation", "installationId", installationId, "orgId", link.OrganisationId, "error", err)
		return err
	}

	// Also process individual repos from the payload (for consistency)
	for _, repo := range installation.Repositories {
		if err := removeRepo(context.Background(), repo, installationId, appId, link.OrganisationId); err != nil {
			return err
		}
	}

	slog.Info("Successfully handled installation deleted event", "installationId", installationId)
	return nil
}

// handleInstallationUpsertEvent handles installation created/unsuspended/new_permissions_accepted events.
func handleInstallationUpsertEvent(ctx context.Context, gh utils.GithubClientProvider, installation *github.InstallationEvent, appId int64) error {
	installationId := installation.Installation.GetID()
	appIdFromPayload := appId
	if installation.Installation.AppID != nil {
		appIdFromPayload = installation.Installation.GetAppID()
	}

	accountLogin, accountId := getAccountDetails(installation.Installation.Account)

	// Retry fetching the link since webhook may arrive before OAuth callback creates it
	var link *models.GithubAppInstallationLink
	backoff := retry.WithMaxRetries(5, retry.NewConstant(2*time.Second))
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		var dbErr error
		link, dbErr = models.DB.GetGithubInstallationLinkForInstallationId(installationId)
		if dbErr != nil {
			return dbErr // permanent error, stop retrying
		}
		if link == nil {
			return retry.RetryableError(errors.New("installation link not found"))
		}
		return nil
	})
	if err != nil {
		slog.Error("Installation link not found after retries", "installationId", installationId, "error", err)
		return fmt.Errorf("installation link not found for installation %d after retries: %w", installationId, err)
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

	// Mark existing installations as deleted before resync
	if err := models.DB.GormDB.Model(&models.GithubAppInstallation{}).Where("github_installation_id = ?", installationId).Update("status", models.GithubAppInstallDeleted).Error; err != nil {
		slog.Error("Error marking installations deleted prior to resync", "installationId", installationId, "error", err)
		return err
	}

	// Soft-delete existing repos and projects
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

// handleInstallationRepositoriesEvent handles incremental repo changes (added/removed from installation scope).
func handleInstallationRepositoriesEvent(ctx context.Context, gh utils.GithubClientProvider, event *github.InstallationRepositoriesEvent, appId int64) error {
	installationId := event.Installation.GetID()
	appIdFromPayload := appId
	if event.Installation.AppID != nil {
		appIdFromPayload = event.Installation.GetAppID()
	}

	accountLogin, accountId := getAccountDetails(event.Installation.Account)

	// Retry fetching the link since webhook may arrive before OAuth callback creates it
	var link *models.GithubAppInstallationLink
	backoff := retry.WithMaxRetries(5, retry.NewConstant(2*time.Second))
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		var dbErr error
		link, dbErr = models.DB.GetGithubInstallationLinkForInstallationId(installationId)
		if dbErr != nil {
			return dbErr // permanent error, stop retrying
		}
		if link == nil {
			return retry.RetryableError(errors.New("installation link not found"))
		}
		return nil
	})
	if err != nil {
		slog.Error("Installation link not found after retries", "installationId", installationId, "error", err)
		return fmt.Errorf("installation link not found for installation %d after retries: %w", installationId, err)
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
