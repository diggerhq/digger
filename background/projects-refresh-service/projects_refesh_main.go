package main

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	utils3 "github.com/diggerhq/digger/libs/git_utils"
	"log/slog"
	"os"
)

func init() {
	logLevel := os.Getenv("DIGGER_LOG_LEVEL")
	var level slog.Leveler

	if logLevel == "DEBUG" {
		level = slog.LevelDebug
	} else if logLevel == "WARN" {
		level = slog.LevelWarn
	} else if logLevel == "ERROR" {
		level = slog.LevelError
	} else {
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func main() {
	cloneUrl := os.Getenv("DIGGER_GITHUB_REPO_CLONE_URL")
	branch := os.Getenv("DIGGER_GITHUB_REPO_CLONE_BRANCH")
	token := os.Getenv("DIGGER_GITHUB_TOKEN")
	repoFullName := os.Getenv("DIGGER_REPO_FULL_NAME")
	orgId := os.Getenv("DIGGER_ORG_ID")

	if cloneUrl == "" && branch == "" && token == "" && orgId == "" && repoFullName == "" {
		slog.Info("smoketests mode, skipping this run")
		os.Exit(0)
	}

	models.ConnectDatabase()

	slog.Info("refreshing projects from repo", "repoFullName", repoFullName)
	err := utils3.CloneGitRepoAndDoAction(cloneUrl, branch, "", token, "", func(dir string) error {
		config, _, err := dg_configuration.LoadDiggerConfigYaml(dir, true, nil, nil)
		if err != nil {
			slog.Error("failed to load digger.yml: %v", "error", err)
			return fmt.Errorf("error loading digger.yml %v", err)
		}
		slog.Debug("RefreshProjectsFromRepo", "repoFullName", repoFullName)
		err = models.DB.RefreshProjectsFromRepo(orgId, *config, repoFullName)
		slog.Debug("finished RefreshProjectsFromRepo", "repoFullName", repoFullName, "error", err)
		if err != nil {
			slog.Error("failed to refresh projects from repo: %v", "error", err)
			return fmt.Errorf("error refreshing projects from repo %v", err)
		}
		return nil
	})
	if err != nil {
		slog.Error("error while cloning repo: %v", err)
		os.Exit(1)
	}

}


