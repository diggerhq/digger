package main

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	utils3 "github.com/diggerhq/digger/libs/git_utils"
	"log/slog"
	"os"
)

func main() {
	cloneUrl := os.Getenv("DIGGER_GITHUB_REPO_CLONE_URL")
	branch := os.Getenv("DIGGER_GITHUB_REPO_CLONE_BRANCH")
	token := os.Getenv("DIGGER_GITHUB_TOKEN")
	orgId := os.Getenv("DIGGER_ORG_ID")
	repoFullName := os.Getenv("DIGGER_REPO_FULL_NAME")

	if cloneUrl == "" && branch == "" && token == "" && orgId == "" && repoFullName == "" {
		slog.Info("smoketests mode, skipping this run")
		os.Exit(0)
	}

	models.ConnectDatabase()

	err := utils3.CloneGitRepoAndDoAction(cloneUrl, branch, "", token, "", func(dir string) error {
		config, err := dg_configuration.LoadDiggerConfigYaml(dir, true, nil)
		if err != nil {
			slog.Error("failed to load digger.yml: %v", "error", err)
			return fmt.Errorf("error loading digger.yml %v", err)
		}
		models.DB.RefreshProjectsFromRepo(orgId, *config, repoFullName)
		return nil
	})
	if err != nil {
		slog.Error("error while cloning repo: %v", err)
		os.Exit(1)
	}

}
