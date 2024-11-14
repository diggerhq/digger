package controllers

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/dominikbraun/graph"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

func (d DiggerController) UpdateRepoCache(c *gin.Context) {
	type UpdateCacheRequest struct {
		RepoFullName string `json:"repo_full_name"`
		Branch       string `json:"branch"`
		orgId        uint   `json:"org_id"`
	}

	var request UpdateCacheRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}

	orgId := request.orgId
	repoFullName := request.RepoFullName

	repoOwner, repoName, _ := strings.Cut(repoFullName, "/")

	repo, err := models.DB.GetRepo(orgId, repoName)
	if err != nil {
		log.Printf("could not get repo: %v", err)
		c.String(500, fmt.Sprintf("coulnt not get repository %v %v", repoFullName, orgId))
		return
	}

	cloneUrl := repo.RepoUrl
	branch := request.Branch

	ghInstallation, err := models.DB.GetInstallationForRepo(repoFullName)
	if err != nil {
		log.Printf("could not get repo: %v", err)
		c.String(500, fmt.Sprintf("coulnt not get repository %v %v", repoFullName, orgId))
		return
	}

	_, token, err := utils.GetGithubService(d.GithubClientProvider, ghInstallation.GithubInstallationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("could not get github service :%v", err)
		c.String(500, fmt.Sprintf("could not get github service %v %v", repoFullName, orgId))
		return
	}

	var diggerYmlStr string
	var config *dg_configuration.DiggerConfig
	var dependencyGraph graph.Graph[string, dg_configuration.Project]
	err = utils.CloneGitRepoAndDoAction(cloneUrl, branch, *token, func(dir string) error {
		diggerYmlBytes, err := os.ReadFile(path.Join(dir, "digger.yml"))
		diggerYmlStr = string(diggerYmlBytes)
		config, _, dependencyGraph, err = dg_configuration.LoadDiggerConfig(dir, true, nil)
		if err != nil {
			log.Printf("Error loading digger config: %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		log.Printf("could not load digger config :%v", err)
		c.String(500, fmt.Sprintf("could load digger config %v %v", repoFullName, orgId))
		return
	}
	// update the cache here
	_, err = models.DB.UpsertRepoCache(orgId, repoFullName, diggerYmlStr, *config, dependencyGraph)
	if err != nil {
		log.Printf("could upadate repo cache :%v", err)
		c.String(500, fmt.Sprintf("could not udpate repo cache %v %v", repoFullName, orgId))
		return

	}

	c.String(200, "ok")
}
