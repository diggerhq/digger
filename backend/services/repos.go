package services

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/service_clients"
	utils3 "github.com/diggerhq/digger/backend/utils"
	"log/slog"
	"strconv"
	"strings"
)

func LoadProjectsFromGithubRepo(gh utils3.GithubClientProvider, installationId string, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string) error {
	installationId64, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		slog.Error("failed to convert installation id %v to int64", "insallationId", installationId)
		return fmt.Errorf("failed to convert installation id to int64")
	}
	link, err := models.DB.GetGithubAppInstallationLink(installationId64)
	if err != nil {
		slog.Error("getting GetGithubAppInstallationLink: %v", "installationId", installationId, "error", err)
		return fmt.Errorf("error getting github app link")
	}

	orgId := link.OrganisationId
	diggerRepoName := strings.ReplaceAll(repoFullName, "/", "-")
	repo, err := models.DB.GetRepo(orgId, diggerRepoName)
	if err != nil {
		slog.Error("getting Repo", "repoName", diggerRepoName, "error", err)
		return fmt.Errorf("error getting github app link")
	}
	if repo == nil {
		slog.Error("Repo not found", "orgId", orgId, "repoName", diggerRepoName, "error", err)
		return fmt.Errorf("repo not found: Org: %v | repo: %v", orgId, diggerRepoName)
	}

	slog.Debug("getting github service", "installationId", installationId, "repoFullName", repoFullName, "repoOwner", repoOwner, "repoName", repoName)
	_, token, err := utils3.GetGithubService(gh, installationId64, repoFullName, repoOwner, repoName)
	if err != nil {
		slog.Error("getting github service", "error", err)
		return fmt.Errorf("error getting github service")
	}

	resp, err := service_clients.TriggerProjectsRefreshService(cloneUrl, branch, *token, repoFullName, strconv.Itoa(int(link.OrganisationId)))
	if err != nil {
		return fmt.Errorf("error triggering projects refresh service: %v", err)
	}
	if resp == nil {
		return fmt.Errorf("error triggering projects refresh service: response nil")
	}

	return nil
}
