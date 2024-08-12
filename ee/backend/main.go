package main

import (
	"embed"
	"fmt"
	"github.com/diggerhq/digger/backend/bootstrap"
	"github.com/diggerhq/digger/backend/config"
	ce_controllers "github.com/diggerhq/digger/backend/controllers"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/utils"
	ci_backends2 "github.com/diggerhq/digger/ee/backend/ci_backends"
	"github.com/diggerhq/digger/ee/backend/controllers"
	"github.com/diggerhq/digger/ee/backend/providers/github"
	"github.com/diggerhq/digger/libs/license"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
)

// based on https://www.digitalocean.com/community/tutorials/using-ldflags-to-set-version-information-for-go-applications
var Version = "dev"

//go:embed templates
var templates embed.FS

func main() {
	err := license.LicenseKeyChecker{}.Check()
	if err != nil {
		log.Printf("error checking license %v", err)
		os.Exit(1)
	}
	diggerController := ce_controllers.DiggerController{
		CiBackendProvider:    ci_backends2.EEBackendProvider{},
		GithubClientProvider: github.DiggerGithubEEClientProvider{},
	}

	r := bootstrap.Bootstrap(templates, diggerController)
	cfg := config.DiggerConfig

	eeController := controllers.DiggerEEController{
		GitlabProvider:    utils.GitlabClientProvider{},
		CiBackendProvider: ci_backends2.EEBackendProvider{},
	}

	r.POST("/get-spec", eeController.GetSpec)
	r.POST("/gitlab-webhook", eeController.GitlabWebHookHandler)

	legacyUiShown := os.Getenv("DIGGER_LEGACY_UI")
	if legacyUiShown != "" {
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

		policiesGroup := r.Group("/policies")
		policiesGroup.Use(middleware.GetWebMiddleware())
		policiesGroup.GET("/", web.PoliciesPage)
		policiesGroup.GET("/add", web.AddPolicyPage)
		policiesGroup.POST("/add", web.AddPolicyPage)
		policiesGroup.GET("/:policyid/details", web.PolicyDetailsPage)
		policiesGroup.POST("/:policyid/details", web.PolicyDetailsUpdatePage)
	} else {
		r.GET("/", func(c *gin.Context) {
			c.HTML(http.StatusOK, "healthy.tmpl", gin.H{})
			return
		})

	}

	jobArtefactsGroup := r.Group("/job_artefacts")
	jobArtefactsGroup.Use(middleware.GetApiMiddleware())
	jobArtefactsGroup.PUT("/", controllers.SetJobArtefact)
	jobArtefactsGroup.GET("/", controllers.DownloadJobArtefact)
	
	port := config.GetPort()
	r.Run(fmt.Sprintf(":%d", port))
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}
