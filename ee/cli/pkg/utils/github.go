package utils

import (
	"github.com/diggerhq/digger/backend/utils"
	"log"
	"os"
)

func createTempDir() string {
	tempDir, err := os.MkdirTemp("", "repo")
	if err != nil {
		log.Fatal(err)
	}
	return tempDir
}

type action func(string) error

func CloneGitRepoAndDoAction(repoUrl string, branch string, token string, tokenUsername string, action action) error {
	dir := createTempDir()
	git := utils.NewGitShellWithTokenAuth(dir, token, tokenUsername)
	err := git.Clone(repoUrl, branch)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	err = action(dir)
	if err != nil {
		log.Printf("error performing action: %v", err)
		return err
	}

	return nil

}
