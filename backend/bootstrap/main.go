package bootstrap

import (
	"embed"
	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/segment"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

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

func Bootstrap(templates embed.FS, diggerController controllers.DiggerController) *gin.Engine {
	defer segment.CloseClient()
	initLogging()
	cfg := config.DiggerConfig

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

	if _, err := os.Stat("templates"); err != nil {
		matches, _ := fs.Glob(templates, "templates/*.tmpl")
		for _, match := range matches {
			r.LoadHTMLFiles(match)
		}
		r.StaticFS("/static", http.FS(templates))
	} else {
		r.Static("/static", "./templates/static")
		r.LoadHTMLGlob("templates/*.tmpl")
	}

	r.POST("/github-app-webhook", diggerController.GithubAppWebHook)
	r.POST("/github-app-webhook/aam", diggerController.GithubAppWebHookAfterMerge)

	tenantActionsGroup := r.Group("/api/tenants")
	tenantActionsGroup.Use(middleware.CORSMiddleware())
	tenantActionsGroup.Any("/associateTenantIdToDiggerOrg", controllers.AssociateTenantIdToDiggerOrg)

	githubGroup := r.Group("/github")
	githubGroup.Use(middleware.GetWebMiddleware())
	githubGroup.GET("/callback", diggerController.GithubAppCallbackPage)
	githubGroup.GET("/repos", diggerController.GithubReposPage)
	githubGroup.GET("/setup", controllers.GithubAppSetup)
	githubGroup.GET("/exchange-code", diggerController.GithubSetupExchangeCode)

	authorized := r.Group("/")
	authorized.Use(middleware.GetApiMiddleware(), middleware.AccessLevel(models.CliJobAccessType, models.AccessPolicyType, models.AdminPolicyType))

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

	authorized.POST("/repos/:repo/projects/:projectName/jobs/:jobId/set-status", diggerController.SetJobStatusForProject)

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

	r.Use(middleware.CORSMiddleware())
	projectsApiGroup := r.Group("/api/projects")
	projectsApiGroup.Use(middleware.GetApiMiddleware())
	projectsApiGroup.GET("/", controllers.FindProjectsForOrg)
	projectsApiGroup.GET("/:project_id", controllers.ProjectDetails)
	projectsApiGroup.GET("/:project_id/runs", controllers.RunsForProject)

	runsApiGroup := r.Group("/api/runs")
	runsApiGroup.Use(middleware.CORSMiddleware(), middleware.GetApiMiddleware())
	runsApiGroup.GET("/:run_id", controllers.RunDetails)
	runsApiGroup.POST("/:run_id/approve", controllers.ApproveRun)

	fronteggWebhookProcessor.POST("/create-org-from-frontegg", controllers.CreateFronteggOrgFromWebhook)

	return r
}

func initLogging() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Initialized the logger successfully")
}
