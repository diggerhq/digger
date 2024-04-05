package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/alextanhongpin/go-gin-starter/config"
	"github.com/diggerhq/digger/backend/controllers"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/sessions"
	gormsessions "github.com/gin-contrib/sessions/gorm"
	"github.com/gin-gonic/gin"
)

// based on https://www.digitalocean.com/community/tutorials/using-ldflags-to-set-version-information-for-go-applications
var Version = "dev"

func main() {

	initLogging()
	cfg := config.New()
	cfg.AutomaticEnv()
	web := controllers.WebController{Config: cfg}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:           os.Getenv("SENTRY_DSN"),
		EnableTracing: true,
		// Set TracesSampleRate to 1.0 to capture 100%
		// of transactions for performance monitoring.
		// We recommend adjusting this value in production,
		TracesSampleRate: 0.1,
		Release:          "api@" + Version,
		Debug:            true,
	}); err != nil {
		log.Printf("Sentry initialization failed: %v\n", err)
	}

	//database migrations
	models.ConnectDatabase()

	r := gin.Default()
	// TODO: check "secret"
	store := gormsessions.NewStore(models.DB.GormDB, true, []byte("secret"))

	r.Use(sessions.Sessions("digger-session", store))

	r.Use(sentrygin.New(sentrygin.Options{Repanic: true}))

	r.Static("/static", "./templates/static")

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"build_date":  cfg.GetString("build_date"),
			"deployed_at": cfg.GetString("deployed_at"),
			"version":     Version,
			"commit_sha":  Version,
		})
	})

	r.SetFuncMap(template.FuncMap{
		"formatAsDate": func(msec int64) time.Time {
			return time.UnixMilli(msec)
		},
	})

	r.LoadHTMLGlob("templates/*.tmpl")
	r.GET("/", web.RedirectToLoginOrProjects)

	r.POST("/github-app-webhook", controllers.GithubAppWebHook)
	r.POST("/github-app-webhook/aam", controllers.GithubAppWebHookAfterMerge)

	tenantActionsGroup := r.Group("/tenants")
	tenantActionsGroup.Use(middleware.CORSMiddleware())
	tenantActionsGroup.Any("/associateTenantIdToDiggerOrg", controllers.AssociateTenantIdToDiggerOrg)

	githubGroup := r.Group("/github")
	githubGroup.Use(middleware.GetWebMiddleware())
	githubGroup.GET("/callback", controllers.GithubAppCallbackPage)
	githubGroup.GET("/repos", controllers.GithubReposPage)
	githubGroup.GET("/setup", controllers.GithubAppSetup)
	githubGroup.GET("/exchange-code", controllers.GithubSetupExchangeCode)

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

	checkoutGroup := r.Group("/")
	checkoutGroup.Use(middleware.GetApiMiddleware())
	checkoutGroup.GET("/checkout", web.Checkout)

	authorized := r.Group("/")
	authorized.Use(middleware.GetApiMiddleware(), middleware.AccessLevel(models.AccessPolicyType, models.AdminPolicyType))

	admin := r.Group("/")
	admin.Use(middleware.GetApiMiddleware(), middleware.AccessLevel(models.AdminPolicyType))

	fronteggWebhookProcessor := r.Group("/")
	fronteggWebhookProcessor.Use(middleware.SecretCodeAuth())

	authorized.GET("/repos/:repo/projects/:projectName/access-policy", controllers.FindAccessPolicy)
	authorized.GET("/orgs/:organisation/access-policy", controllers.FindAccessPolicyForOrg)

	authorized.GET("/repos/:repo/projects/:projectName/plan-policy", controllers.FindPlanPolicy)
	authorized.GET("/orgs/:organisation/plan-policy", controllers.FindPlanPolicyForOrg)

	authorized.GET("/repos/:repo/projects/:projectName/drift-policy", controllers.FindDriftPolicy)
	authorized.GET("/orgs/:organisation/drift-policy", controllers.FindDriftPolicyForOrg)

	authorized.GET("/repos/:repo/projects/:projectName/runs", controllers.RunHistoryForProject)
	authorized.POST("/repos/:repo/projects/:projectName/runs", controllers.CreateRunForProject)

	authorized.POST("/repos/:repo/projects/:projectName/jobs/:jobId/set-status", controllers.SetJobStatusForProject)

	authorized.GET("/repos/:repo/projects", controllers.FindProjectsForRepo)
	authorized.POST("/repos/:repo/report-projects", controllers.ReportProjectsForRepo)

	authorized.GET("/orgs/:organisation/projects", controllers.FindProjectsForOrg)

	admin.PUT("/repos/:repo/projects/:projectName/access-policy", controllers.UpsertAccessPolicyForRepoAndProject)
	admin.PUT("/orgs/:organisation/access-policy", controllers.UpsertAccessPolicyForOrg)

	admin.PUT("/repos/:repo/projects/:projectName/plan-policy", controllers.UpsertPlanPolicyForRepoAndProject)
	admin.PUT("/orgs/:organisation/plan-policy", controllers.UpsertPlanPolicyForOrg)

	admin.PUT("/repos/:repo/projects/:projectName/drift-policy", controllers.UpsertDriftPolicyForRepoAndProject)
	admin.PUT("/orgs/:organisation/drift-policy", controllers.UpsertDriftPolicyForOrg)

	admin.POST("/tokens/issue-access-token", controllers.IssueAccessTokenForOrg)

	apiGroup := r.Group("/api")
	apiGroup.Use(middleware.CORSMiddleware())

	projectsApiGroup := apiGroup.Group("/projects")
	projectsApiGroup.Use(middleware.GetWebMiddleware())
	projectsApiGroup.GET("/", controllers.FindProjectsForOrg)
	projectsApiGroup.GET("/:project_id/runs", controllers.RunsForProject)

	runsApiGroup := apiGroup.Group("/runs")
	runsApiGroup.Use(middleware.GetWebMiddleware())
	runsApiGroup.GET("/:run_id", controllers.RunDetails)
	runsApiGroup.POST("/:run_id/approve", controllers.ApproveRun)

	fronteggWebhookProcessor.POST("/create-org-from-frontegg", controllers.CreateFronteggOrgFromWebhook)

	r.Run(fmt.Sprintf(":%d", cfg.GetInt("port")))
}

func initLogging() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Initialized the logger successfully")
}
