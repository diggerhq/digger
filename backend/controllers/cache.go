package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/digger_config/terragrunt/tac"
	"github.com/diggerhq/digger/libs/git_utils"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/diggerhq/digger/backend/logging"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/gin-gonic/gin"
)

type UpdateCacheRequest struct {
	RepoFullName   string `json:"repo_full_name"`
	Branch         string `json:"branch"`
	OrgId          uint   `json:"org_id"`
	InstallationId int64  `json:"installation_id"`
}

func (d DiggerController) UpdateRepoCache(c *gin.Context) {

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
		c.String(http.StatusInternalServerError, fmt.Sprintf("could not find installation link %v %v", repoFullName, installationId))
		return
	}
	if link == nil {
		slog.Error("GitHub app installation link not found", "repoFullName", repoFullName, "installationId", installationId)
		c.String(http.StatusInternalServerError, fmt.Sprintf("could not find installation link %v %v", repoFullName, installationId))
		return
	}
	orgId := link.OrganisationId

	slog.Info("Processing repo cache update", "orgId", orgId)

	repoOwner, repoName, _ := strings.Cut(repoFullName, "/")
	repoDiggerName := strings.ReplaceAll(repoFullName, "/", "-")

	repo, err := models.DB.GetRepo(orgId, repoDiggerName)
	if err != nil {
		slog.Error("Could not get repo", "error", err, "repoFullName", repoFullName, "orgId", orgId)
		c.String(http.StatusInternalServerError, fmt.Sprintf("could not get repository %v %v", repoFullName, orgId))
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
	var newAtlantisConfig *tac.AtlantisConfig

	// update the cache here, do it async for immediate response
	go func(ctx context.Context) {
		defer logging.InheritRequestLogger(ctx)()
		err = git_utils.CloneGitRepoAndDoAction(cloneUrl, branch, "", *token, "", func(dir string) error {
			diggerYmlBytes, err := os.ReadFile(path.Join(dir, "digger.yml"))
			diggerYmlStr = string(diggerYmlBytes)
			config, _, _, newAtlantisConfig, err = dg_configuration.LoadDiggerConfig(dir, true, nil, nil)
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
		_, err = models.DB.UpsertRepoCache(orgId, repoFullName, diggerYmlStr, *config, newAtlantisConfig)
		if err != nil {
			slog.Error("Could not update repo cache", "error", err)
			return
		}
		slog.Info("Successfully updated repo cache", "repoFullName", repoFullName, "orgId", orgId)
	}(c.Request.Context())

	c.String(http.StatusOK, "successfully submitted cache for processing, check backend logs for progress")
}

func sendProcessCacheRequest(repoFullName string, branch string, installationId int64) error {
	diggerHostname := os.Getenv("HOSTNAME")
	webhookSecret := os.Getenv("DIGGER_INTERNAL_SECRET")

	installationLink, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		slog.Error("Error getting installation link", "installationId", installationId, "error", err)
		return err
	}

	orgId := installationLink.OrganisationId

	payload := UpdateCacheRequest{
		RepoFullName:   repoFullName,
		Branch:         branch,
		InstallationId: installationId,
		OrgId:          orgId,
	}

	cacheRefreshUrl, err := url.JoinPath(diggerHostname, "_internal/update_repo_cache")
	if err != nil {
		slog.Error("Error joining URL paths", "error", err)
		return err
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Process Cache: error marshaling JSON", "error", err)
		return err
	}

	req, err := http.NewRequest("POST", cacheRefreshUrl, bytes.NewBuffer(jsonPayload))
	if err != nil {
		slog.Error("Process Cache: Error creating request", "error", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", webhookSecret))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return err
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	if statusCode != 200 {
		// Read response body to get error details
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			slog.Error("Failed to read error response body", "error", err)
		}

		slog.Error("got unexpected cache status",
			"statusCode", statusCode,
			"repoFullName", repoFullName,
			"orgId", orgId,
			"branch", branch,
			"installationId", installationId,
			"responseBody", string(responseBody))

		return fmt.Errorf("cache update failed with status code %d: %s", statusCode, string(responseBody))
	}
	return nil
}
