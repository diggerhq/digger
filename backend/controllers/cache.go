package controllers

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
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
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}

	repoFullName := request.RepoFullName
	installationId := request.InstallationId
	link, err := models.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		log.Printf("could not installation link: %v", err)
		c.String(500, fmt.Sprintf("coulnt not find installation link %v %v", repoFullName, installationId))
		return

	}
	orgId := link.OrganisationId

	log.Printf("the org id is %v", orgId)

	repoOwner, repoName, _ := strings.Cut(repoFullName, "/")
	repoDiggerName := strings.ReplaceAll(repoFullName, "/", "-")

	repo, err := models.DB.GetRepo(orgId, repoDiggerName)
	if err != nil {
		log.Printf("could not get repo: %v", err)
		c.String(500, fmt.Sprintf("coulnt not get repository %v %v", repoFullName, orgId))
		return
	}

	cloneUrl := fmt.Sprintf("https://%v/%v", utils.GetGithubHostname(), repo.RepoFullName)
	branch := request.Branch

	_, token, err := utils.GetGithubService(d.GithubClientProvider, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("could not get github service :%v", err)
		c.String(500, fmt.Sprintf("could not get github service %v %v", repoFullName, orgId))
		return
	}

	var diggerYmlStr string
	var config *dg_configuration.DiggerConfig

	// update the cache here, do it async for immediate response
	go func() {
		err = utils.CloneGitRepoAndDoAction(cloneUrl, branch, "", *token, func(dir string) error {
			diggerYmlBytes, err := os.ReadFile(path.Join(dir, "digger.yml"))
			diggerYmlStr = string(diggerYmlBytes)
			config, _, _, err = dg_configuration.LoadDiggerConfig(dir, true, nil)
			if err != nil {
				log.Printf("Error loading digger config: %v", err)
				return err
			}
			return nil
		})

		if err != nil {
			log.Printf("could not load digger config :%v", err)
			return
		}
		_, err = models.DB.UpsertRepoCache(orgId, repoFullName, diggerYmlStr, *config)
		if err != nil {
			log.Printf("could upadate repo cache :%v", err)
			return

		}
	}()

	c.String(200, "successfully submitted cache for processing, check backend logs for progress")
}
