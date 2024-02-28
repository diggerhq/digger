package utils

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
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

func CloneGitRepoAndDoAction(repoUrl string, branch string, token string, action action) error {
	dir := createTempDir()
	cloneOptions := git.CloneOptions{
		URL:           repoUrl,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		Depth:         1,
		SingleBranch:  true,
	}

	if token != "" {
		cloneOptions.Auth = &http.BasicAuth{
			Username: "x-access-token", // anything except an empty string
			Password: token,
		}
	}

	_, err := git.PlainClone(dir, false, &cloneOptions)
	if err != nil {
		log.Printf("PlainClone error: %v\n", err)
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
