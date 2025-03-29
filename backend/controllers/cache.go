package controllers

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/gin-gonic/gin"
)

func (d DiggerController) UpdateRepoCache(c *gin.Context) {
	type UpdateCacheRequest struct {
		RepoFullName   string `json:"repo_full_name"`
		Branch         string `json:"branch"`
		OrgId          uint   `json:"org_id"`
		InstallationId int64  `json:"installation_id"`
	}

	var request UpdateCacheRequest
	err := c.BindJSON(&request)
	if err != nil {
		slog.Error("Error binding JSON", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}

	repoFullName := request.RepoFullName
	installationId := request.InstallationId
	link, err := models.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		slog.Error("Could not get installation link", "error", err, "repoFullName", repoFullName, "installationId", installationId)
		c.String(http.StatusInternalServerError, fmt.Sprintf("coulnt not find installation link %v %v", repoFullName, installationId))
		return
	}
	orgId := link.OrganisationId

	slog.Info("Processing repo cache update", "orgId", orgId)

	repoOwner, repoName, _ := strings.Cut(repoFullName, "/")
	repoDiggerName := strings.ReplaceAll(repoFullName, "/", "-")

	repo, err := models.DB.GetRepo(orgId, repoDiggerName)
	if err != nil {
		slog.Error("Could not get repo", "error", err, "repoFullName", repoFullName, "orgId", orgId)
		c.String(http.StatusInternalServerError, fmt.Sprintf("coulnt not get repository %v %v", repoFullName, orgId))
		return
	}

	cloneUrl := fmt.Sprintf("https://%v/%v", utils.GetGithubHostname(), repo.RepoFullName)
	branch := request.Branch

	_, token, err := utils.GetGithubService(d.GithubClientProvider, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		slog.Error("Could not get GitHub service", "error", err, "repoFullName", repoFullName, "orgId", orgId)
		c.String(http.StatusInternalServerError, fmt.Sprintf("could not get github service %v %v", repoFullName, orgId))
		return
	}

	var diggerYmlStr string
	var config *dg_configuration.DiggerConfig

	// update the cache here, do it async for immediate response
	go func() {
		err = utils.CloneGitRepoAndDoAction(cloneUrl, branch, "", *token, "", func(dir string) error {
			diggerYmlBytes, err := os.ReadFile(path.Join(dir, "digger.yml"))
			diggerYmlStr = string(diggerYmlBytes)
			config, _, _, err = dg_configuration.LoadDiggerConfig(dir, true, nil)
			if err != nil {
				slog.Error("Error loading digger config", "error", err)
				return err
			}
			return nil
		})

		if err != nil {
			slog.Error("Could not load digger config", "error", err)
			return
		}
		_, err = models.DB.UpsertRepoCache(orgId, repoFullName, diggerYmlStr, *config)
		if err != nil {
			slog.Error("Could not update repo cache", "error", err)
			return
		}
		slog.Info("Successfully updated repo cache", "repoFullName", repoFullName, "orgId", orgId)
	}()

	c.String(http.StatusOK, "successfully submitted cache for processing, check backend logs for progress")
}
