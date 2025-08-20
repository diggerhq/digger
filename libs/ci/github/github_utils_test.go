package github

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	net "net/http"
	"os"
	"strconv"
	"testing"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v61/github"
	"github.com/stretchr/testify/assert"
)

func initLogger() {
	logLevel := os.Getenv("DIGGER_LOG_LEVEL")
	var level slog.Leveler
	if logLevel == "DEBUG" {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	slog.SetDefault(logger)
}

func TestListRepositoriesReturnsAllReposities(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("skipping in CI")
	}
	githubAppPrivateKeyB64 := os.Getenv("GITHUB_APP_PRIVATE_KEY_BASE64")
	decodedBytes, err := base64.StdEncoding.DecodeString(githubAppPrivateKeyB64)
	if err != nil {
		slog.Info("Failed to decode GITHUB_APP_PRIVATE_KEY_BASE64", "error", err)
		t.Error(err)
	}
	githubAppPrivateKey := string(decodedBytes)
	tr := net.DefaultTransport
	var githubAppId = os.Getenv("GITHUB_APP_ID")
	var installationId = os.Getenv("INSTALLATION_ID")

	githubAppIdintValue, err := strconv.ParseInt(githubAppId, 10, 64)
	if err != nil {
		fmt.Printf("Error converting environment variable to int64: %v\n", err)
		t.Fatalf("Failed to parse GITHUB_APP_ID: %v", err)
	}
	installationIdintValue, err := strconv.ParseInt(installationId, 10, 64)
	if err != nil {
		fmt.Printf("Error converting environment variable to int64: %v\n", err)
		t.Fatalf("Failed to parse INSTALLATION_ID: %v", err)
	}

	itr, err := ghinstallation.New(tr, githubAppIdintValue, installationIdintValue, []byte(githubAppPrivateKey))
	if err != nil {
		slog.Info("Failed to initialize GitHub app installation",
			"githubAppId", githubAppId,
			"installationId", installationId,
			"error", err,
		)
		t.Error(err)
	}

	client := github.NewClient(&net.Client{Transport: itr})

	allRepos, err := ListGithubRepos(client)
	fmt.Println("err is", err)
	assert.Nil(t, err)
	// Currently Digger has 388 repositories, Update the hardcoded value to the expected number of repositories if the number changes in the future.
	assert.Equal(t, 388, len(allRepos))
}
