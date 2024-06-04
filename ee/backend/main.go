package main

import (
	"embed"
	"fmt"
	"github.com/diggerhq/digger/backend/bootstrap"
	"github.com/diggerhq/digger/backend/config"
	ce_controllers "github.com/diggerhq/digger/backend/controllers"
	"github.com/diggerhq/digger/backend/middleware"
	ci_backends2 "github.com/diggerhq/digger/ee/backend/ci_backends"
	"github.com/diggerhq/digger/ee/backend/controllers"
	"github.com/gin-gonic/gin"
	"net/http"
)

// based on https://www.digitalocean.com/community/tutorials/using-ldflags-to-set-version-information-for-go-applications
var Version = "dev"

//go:embed templates
var templates embed.FS

func main() {
	ghController := ce_controllers.GithubController{
		CiBackendProvider: ci_backends2.EEBackendProvider{},
	}

	r := bootstrap.Bootstrap(templates, ghController)
	cfg := config.DiggerConfig

	// redirect to projects by default
	r.GET("/", func(context *gin.Context) {
		context.Redirect(http.StatusFound, "/projects")
	})

	web := controllers.WebController{Config: cfg}
	projectsGroup := r.Group("/projects")
	projectsGroup.Use(middleware.GetWebMiddleware())
	projectsGroup.GET("/", web.ProjectsPage)
	projectsGroup.GET("/:projectid/details", web.ProjectDetailsPage)
	projectsGroup.POST("/:projectid/details", web.ProjectDetailsUpdatePage)

	runsGroup := r.Group("/runs")
	runsGroup.Use(middleware.GetWebMiddleware())
	runsGroup.GET("/", web.RunsPage)
	runsGroup.GET("/:runid/details", web.RunDetailsPage)

	reposGroup := r.Group("/repos")
	reposGroup.Use(middleware.GetWebMiddleware())
	reposGroup.GET("/", web.ReposPage)

	repoGroup := r.Group("/repo")
	repoGroup.Use(middleware.GetWebMiddleware())
	repoGroup.GET("/", web.ReposPage)
	repoGroup.GET("/:repoid/", web.UpdateRepoPage)
	repoGroup.POST("/:repoid/", web.UpdateRepoPage)

	policiesGroup := r.Group("/policies")
	policiesGroup.Use(middleware.GetWebMiddleware())
	policiesGroup.GET("/", web.PoliciesPage)
	policiesGroup.GET("/add", web.AddPolicyPage)
	policiesGroup.POST("/add", web.AddPolicyPage)
	policiesGroup.GET("/:policyid/details", web.PolicyDetailsPage)
	policiesGroup.POST("/:policyid/details", web.PolicyDetailsUpdatePage)

	port := config.GetPort()
	r.Run(fmt.Sprintf(":%d", port))
}
