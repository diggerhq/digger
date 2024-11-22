package services

import (
	"fmt"
	utils3 "github.com/diggerhq/digger/backend/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/utils"
	"log"
)

func GetWorkflowsForRepoAndBranch(gh utils.GithubClientProvider, repoId int64, branch string, commitHash string) (map[string]dg_configuration.Workflow, error) {
	r := dbmodels.DB.Query.Repo
	repo, err := dbmodels.DB.Query.Repo.Where(r.ID.Eq(repoId)).First()
	if err != nil {
		log.Printf("could not find repo: %v : %v", repoId, err)
		return nil, fmt.Errorf("could not find repo: %v: %v", repoId, err)
	}
	repoOwner := repo.RepoOrganisation
	repoFullName := repo.RepoFullName
	repoName := repo.RepoName
	orgId := repo.OrganizationID

	appInstallation, err := dbmodels.DB.GetGithubAppInstallationByOrgAndRepo(orgId, repoFullName, dbmodels.GithubAppInstallActive)
	if err != nil {
		log.Printf("error retrieving app installation")
		return nil, fmt.Errorf("error retrieving app installation %v", err)
	}
	installationId := appInstallation.GithubInstallationID
	log.Printf("installation id is: %v", installationId)

	cloneUrl := fmt.Sprintf("https://%v/%v", utils.GetGithubHostname(), repo.RepoFullName)

	_, token, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("could not get github service :%v", err)
		return nil, fmt.Errorf("could not get github service :%v", err)
	}

	var config *dg_configuration.DiggerConfig

	err = utils3.CloneGitRepoAndDoAction(cloneUrl, branch, commitHash, *token, func(dir string) error {
		// we create a blank file if it does not exist
		err := dg_configuration.CheckOrCreateDiggerFile(dir)
		if err != nil {
			log.Printf("Error creating blank digger.yml if not exists: %v", err)
			return err
		}
		config, _, _, err = dg_configuration.LoadDiggerConfig(dir, false, nil)
		if err != nil {
			log.Printf("Error loading digger config: %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		log.Printf("could not load digger config :%v", err)
		return nil, fmt.Errorf("could not load digger config :%v", err)
	}

	return config.Workflows, nil
}
