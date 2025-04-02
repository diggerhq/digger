package main

import (
	"embed"
	"fmt"

	"github.com/go-substrate/strate/backend/bootstrap"
	"github.com/go-substrate/strate/backend/ci_backends"
	"github.com/go-substrate/strate/backend/config"
	"github.com/go-substrate/strate/backend/controllers"
	"github.com/go-substrate/strate/backend/utils"
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
