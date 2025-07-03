package services

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	utils3 "github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/ee/drift/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	utils2 "github.com/diggerhq/digger/next/utils"
	"log/slog"
	"strconv"
	"strings"
)

func LoadProjectsFromGithubRepo(gh utils2.GithubClientProvider, installationId string, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string) error {
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

	_, token, err := utils.GetGithubService(gh, installationId64, repoFullName, repoOwner, repoName)
	if err != nil {
		slog.Error("getting github service", "error", err)
		return fmt.Errorf("error getting github service")
	}

	err = utils3.CloneGitRepoAndDoAction(cloneUrl, branch, "", *token, "", func(dir string) error {
		config, err := dg_configuration.LoadDiggerConfigYaml(dir, true, nil)
		if err != nil {
			slog.Error("failed to load digger.yml: %v", "error", err)
			return fmt.Errorf("error loading digger.yml %v", err)
		}
		models.DB.RefreshProjectsFromRepo(strconv.Itoa(int(link.OrganisationId)), *config, repoFullName)
		return nil
	})
	if err != nil {
		return fmt.Errorf("error while cloning repo: %v", err)
	}

	return nil
}
