package tasks

import (
	"fmt"
	utils3 "github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/ee/drift/dbmodels"
	"github.com/diggerhq/digger/ee/drift/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	utils2 "github.com/diggerhq/digger/next/utils"
	"log"
	"strconv"
	"strings"
)

func LoadProjectsFromGithubRepo(gh utils2.GithubClientProvider, installationId string, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string) error {
	link, err := dbmodels.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
		return fmt.Errorf("error getting github app link")
	}

	orgId := link.OrganisationID
	diggerRepoName := strings.ReplaceAll(repoFullName, "/", "-")
	repo, err := dbmodels.DB.GetRepo(orgId, diggerRepoName)
	if err != nil {
		log.Printf("Error getting Repo: %v", err)
		return fmt.Errorf("error getting github app link")
	}
	if repo == nil {
		log.Printf("Repo not found: Org: %v | repo: %v", orgId, diggerRepoName)
		return fmt.Errorf("Repo not found: Org: %v | repo: %v", orgId, diggerRepoName)
	}

	installationId64, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		log.Printf("failed to convert installation id %v to int64", installationId)
		return fmt.Errorf("failed to convert installation id to int64")
	}
	_, token, err := utils.GetGithubService(gh, installationId64, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("Error getting github service: %v", err)
		return fmt.Errorf("error getting github service")
	}

	err = utils3.CloneGitRepoAndDoAction(cloneUrl, branch, "", *token, func(dir string) error {
		config, err := dg_configuration.LoadDiggerConfigYaml(dir, true, nil)
		if err != nil {
			log.Printf("ERROR load digger.yml: %v", err)
			return fmt.Errorf("error loading digger.yml %v", err)
		}
		dbmodels.DB.RefreshProjectsFromRepo(link.OrganisationID, *config, repo)
		return nil
	})
	if err != nil {
		return fmt.Errorf("error while cloning repo: %v", err)
	}

	return nil
}
