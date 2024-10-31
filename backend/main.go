package main

import (
	"embed"
	"fmt"
	"github.com/diggerhq/digger/backend/bootstrap"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/controllers"
	"github.com/diggerhq/digger/backend/utils"
	"os"
	"path/filepath"
)

//go:embed templates
var templates embed.FS

func main() {
	ghController := controllers.DiggerController{
		CiBackendProvider:                  ci_backends.DefaultBackendProvider{},
		GithubClientProvider:               utils.DiggerGithubRealClientProvider{},
		GithubWebhookPostIssueCommentHooks: make([]controllers.IssueCommentHook, 0),
	}
	r := bootstrap.Bootstrap(templates, ghController)
	r.GET("/", controllers.Home)
	port := config.GetPort()
	r.Run(fmt.Sprintf(":%d", port))
}
