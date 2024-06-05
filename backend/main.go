package main

import (
	"embed"
	"fmt"
	"github.com/diggerhq/digger/backend/bootstrap"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/controllers"
)

//go:embed templates
var templates embed.FS

func main() {
	ghController := controllers.GithubController{
		CiBackendProvider: ci_backends.DefaultBackendProvider{},
	}
	r := bootstrap.Bootstrap(templates, ghController)
	r.GET("/", controllers.Home)
	port := config.GetPort()
	r.Run(fmt.Sprintf(":%d", port))
}
