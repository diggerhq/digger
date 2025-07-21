package bootstrap

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	pprof_gin "github.com/gin-contrib/pprof"
	"github.com/gin-contrib/sessions"
	gormsessions "github.com/gin-contrib/sessions/gorm"
	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"

	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/controllers"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/utils"
)

// based on https://www.digitalocean.com/community/tutorials/using-ldflags-to-set-version-information-for-go-applications
var Version = "dev"

func setupProfiler(r *gin.Engine) {
	// Enable pprof endpoints
	pprof_gin.Register(r)

	// Create profiles directory if it doesn't exist
	if err := os.MkdirAll("/tmp/profiles", 0o755); err != nil {
		slog.Error("Failed to create profiles directory", "error", err)
		panic(err)
	}

	// Start periodic profiling goroutine
	go periodicProfiling()
}

func periodicProfiling() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Trigger GC before taking memory profile
			runtime.GC()

			// Create memory profile
			timestamp := time.Now().Format("2006-01-02-15-04-05")
			memProfilePath := filepath.Join("/tmp/profiles", fmt.Sprintf("memory-%s.pprof", timestamp))
			f, err := os.Create(memProfilePath)
			if err != nil {
				slog.Error("Failed to create memory profile", "error", err)
				continue
			}

			if err := pprof.WriteHeapProfile(f); err != nil {
				slog.Error("Failed to write memory profile", "error", err)
			}
			f.Close()

			// Cleanup old profiles (keep last 24)
			cleanupOldProfiles("/tmp/profiles", 168)
		}
	}
}

func cleanupOldProfiles(dir string, keep int) {
	files, err := filepath.Glob(filepath.Join(dir, "memory-*.pprof"))
	if err != nil {
		slog.Error("Failed to list profile files", "error", err)
		return
	}

	if len(files) <= keep {
		return
	}

	// Sort files by name (which includes timestamp)
	for i := 0; i < len(files)-keep; i++ {
		if err := os.Remove(files[i]); err != nil {
			slog.Error("Failed to remove old profile", "file", files[i], "error", err)
		}
	}
}

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
		DebugWriter:      utils.NewSentrySlogWriter(slog.Default().WithGroup("sentry")),
	}); err != nil {
		slog.Error("Sentry initialization failed", "error", err)
	}

	// database migrations
	models.ConnectDatabase()

	r := gin.Default()

	r.Use(sloggin.New(slog.Default().WithGroup("http")))

	if _, exists := os.LookupEnv("DIGGER_PPROF_DEBUG_ENABLED"); exists {
		setupProfiler(r)
	}

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

	tenantActionsGroup := r.Group("/api/tenants")
	tenantActionsGroup.Use(middleware.CORSMiddleware())
	tenantActionsGroup.Any("/associateTenantIdToDiggerOrg", controllers.AssociateTenantIdToDiggerOrg)

	githubGroup := r.Group("/github")
	githubGroup.Use(middleware.GetWebMiddleware())
	// authless endpoint because we no longer rely on orgId
	r.GET("/github/callback", diggerController.GithubAppCallbackPage)
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

	// internal endpoints not meant to be exposed to public and protected behind webhook secret
	if enableInternal := os.Getenv("DIGGER_ENABLE_INTERNAL_ENDPOINTS"); enableInternal == "true" {
		r.POST("_internal/update_repo_cache", middleware.InternalApiAuth(), diggerController.UpdateRepoCache)
		r.POST("_internal/api/create_user", middleware.InternalApiAuth(), diggerController.CreateUserInternal)
		r.POST("_internal/api/upsert_org", middleware.InternalApiAuth(), diggerController.UpsertOrgInternal)
	}

	if enableApi := os.Getenv("DIGGER_ENABLE_API_ENDPOINTS"); enableApi == "true" {
		apiGroup := r.Group("/api")
		apiGroup.Use(middleware.InternalApiAuth(), middleware.HeadersApiAuth())

		orgsApiGroup := apiGroup.Group("/orgs")
		orgsApiGroup.GET("/settings/", controllers.GetOrgSettingsApi)
		orgsApiGroup.PUT("/settings/", controllers.UpdateOrgSettingsApi)

		billingApiGroup := apiGroup.Group("/billing")
		billingApiGroup.GET("/", controllers.BillingStatusApi)

		reposApiGroup := apiGroup.Group("/repos")
		reposApiGroup.GET("/", controllers.ListReposApi)
		reposApiGroup.GET("/:repo_id/jobs", controllers.GetJobsForRepoApi)

		projectsApiGroup := apiGroup.Group("/projects")
		projectsApiGroup.GET("/", controllers.ListProjectsApi)
		projectsApiGroup.GET("/:project_id/", controllers.ProjectsDetailsApi)
		projectsApiGroup.PUT("/:project_id/", controllers.UpdateProjectApi)

		githubApiGroup := apiGroup.Group("/github")
		githubApiGroup.POST("/link", controllers.LinkGithubInstallationToOrgApi)

		vcsApiGroup := apiGroup.Group("/connections")
		vcsApiGroup.GET("/:id", controllers.GetVCSConnection)
		vcsApiGroup.GET("/", controllers.ListVCSConnectionsApi)
		vcsApiGroup.POST("/", controllers.CreateVCSConnectionApi)
		vcsApiGroup.DELETE("/:id", controllers.DeleteVCSConnection)

		policyApiGroup := apiGroup.Group("/policies")
		policyApiGroup.GET("/:policy_type", controllers.PolicyOrgGetApi)
		policyApiGroup.PUT("/", controllers.PolicyOrgUpsertApi)
	}

	return r
}

func initLogging() {
	logLevel := os.Getenv("DIGGER_LOG_LEVEL")
	var level slog.Leveler

	switch logLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}
