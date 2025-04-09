package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	pprof_gin "github.com/gin-contrib/pprof"
	"github.com/gin-contrib/sessions"
	gormsessions "github.com/gin-contrib/sessions/gorm"
	"github.com/gin-gonic/gin"
	"github.com/go-substrate/strate/backend/ci_backends"
	"github.com/go-substrate/strate/backend/config"
	"github.com/go-substrate/strate/backend/controllers"
	"github.com/go-substrate/strate/backend/middleware"
	"github.com/go-substrate/strate/backend/models"
	"github.com/go-substrate/strate/backend/utils"
	"github.com/go-substrate/strate/backend/version"
	"golang.org/x/sync/errgroup"
)

// StrateApp represents the core application structure for the Strate backend
type StrateApp struct {
	cfg    *config.Config
	router *gin.Engine

	diggerController controllers.DiggerController

	// Profiling related
	profilingTicker *time.Ticker

	// Server resources
	httpServer *http.Server
}

// NewApp creates a new instance of the Strate backend application
func NewApp(cfg *config.Config) (*StrateApp, error) {
	app := &StrateApp{
		cfg: cfg,
		diggerController: controllers.DiggerController{
			CiBackendProvider:                  ci_backends.DefaultBackendProvider{},
			GithubClientProvider:               utils.DiggerGithubRealClientProvider{},
			GithubWebhookPostIssueCommentHooks: make([]controllers.IssueCommentHook, 0),
		},
	}

	return app, nil
}

// setup initializes the application components
func (app *StrateApp) setup() error {
	// Initialize Sentry if enabled
	if app.cfg.Analytics.Sentry.Enabled {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              app.cfg.Analytics.Sentry.DSN,
			EnableTracing:    app.cfg.Analytics.Sentry.EnableTracing,
			TracesSampleRate: app.cfg.Analytics.Sentry.TracesSampleRate,
			Release:          app.cfg.Analytics.Sentry.Release,
			Debug:            app.cfg.Analytics.Sentry.Debug,
			Environment:      app.cfg.Analytics.Sentry.Environment,
		}); err != nil {
			slog.Warn("Sentry initialization failed", "error", err)
		}
	}

	// Initialize database
	models.ConnectDatabase()

	// Set up the Gin router
	app.router = app.createRouter()

	return nil
}

// createRouter sets up all routes and middleware for the application
func (app *StrateApp) createRouter() *gin.Engine {
	// Set Gin mode based on environment
	if app.cfg.Log.Level == slog.LevelDebug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// Apply CORS middleware to all routes
	r.Use(middleware.CORSMiddleware())

	// Enable profiling if configured
	if app.cfg.Server.Pprof.Enabled {
		pprof_gin.Register(r)
	}

	// Set up session store
	// TODO: Remove harcoded session and use config instead
	store := gormsessions.NewStore(models.DB.GormDB, true, []byte(app.cfg.Auth.JWTSecret))
	r.Use(sessions.Sessions("digger-session", store))

	// Configure Sentry middleware if enabled
	if app.cfg.Analytics.Sentry.Enabled {
		r.Use(sentrygin.New(sentrygin.Options{
			Repanic: true,
		}))
	}

	// Set up routes
	app.setupRoutes(r)

	return r
}

// setupRoutes configures all application routes
func (app *StrateApp) setupRoutes(r *gin.Engine) {
	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version":    version.Version,
			"commit_sha": version.Meta,
		})
	})

	// GitHub app webhook routes
	r.POST("/github-app-webhook", app.diggerController.GithubAppWebHook)

	// Tenant actions routes
	tenantActionsGroup := r.Group("/api/tenants")
	tenantActionsGroup.Any("/associateTenantIdToDiggerOrg", controllers.AssociateTenantIdToDiggerOrg)

	// GitHub OAuth and setup routes
	githubGroup := r.Group("/github")
	githubGroup.Use(middleware.GetWebMiddleware())
	// Authless endpoint because we no longer rely on orgId
	r.GET("/github/callback", app.diggerController.GithubAppCallbackPage)
	githubGroup.GET("/repos", app.diggerController.GithubReposPage)
	githubGroup.GET("/setup", controllers.GithubAppSetup)
	githubGroup.GET("/exchange-code", app.diggerController.GithubSetupExchangeCode)

	// Authorized API routes
	authorized := r.Group("/")
	authorized.Use(middleware.GetApiMiddleware(), middleware.AccessLevel(models.CliJobAccessType, models.AccessPolicyType, models.AdminPolicyType))

	// Admin API routes
	admin := r.Group("/")
	admin.Use(middleware.GetApiMiddleware(), middleware.AccessLevel(models.AdminPolicyType))

	// FrontEgg webhook routes
	fronteggWebhookProcessor := r.Group("/")
	fronteggWebhookProcessor.Use(middleware.SecretCodeAuth())

	// Set up all the routes from the original code
	app.setupAuthorizedRoutes(authorized)
	app.setupAdminRoutes(admin)

	// Set up internal routes if enabled
	if app.cfg.Server.EnableInternalEndpoints {
		app.setupInternalRoutes(r)
	}

	// Set up API endpoints if enabled
	if app.cfg.Server.EnableApiEndpoints {
		app.setupAPIRoutes(r)
	}
}

// setupAuthorizedRoutes configures routes requiring basic authorization
func (app *StrateApp) setupAuthorizedRoutes(r *gin.RouterGroup) {
	r.GET("/repos/:repo/projects/:projectName/access-policy", controllers.FindAccessPolicy)
	r.GET("/orgs/:organisation/access-policy", controllers.FindAccessPolicyForOrg)
	r.GET("/repos/:repo/projects/:projectName/plan-policy", controllers.FindPlanPolicy)
	r.GET("/orgs/:organisation/plan-policy", controllers.FindPlanPolicyForOrg)
	r.GET("/repos/:repo/projects/:projectName/drift-policy", controllers.FindDriftPolicy)
	r.GET("/orgs/:organisation/drift-policy", controllers.FindDriftPolicyForOrg)
	r.GET("/repos/:repo/projects/:projectName/runs", controllers.RunHistoryForProject)
	r.POST("/repos/:repo/projects/:projectName/runs", controllers.CreateRunForProject)
	r.POST("/repos/:repo/projects/:projectName/jobs/:jobId/set-status", app.diggerController.SetJobStatusForProject)
	r.GET("/repos/:repo/projects", controllers.FindProjectsForRepo)
	r.POST("/repos/:repo/report-projects", controllers.ReportProjectsForRepo)
	r.GET("/orgs/:organisation/projects", controllers.FindProjectsForOrg)
}

// setupAdminRoutes configures routes requiring admin privileges
func (app *StrateApp) setupAdminRoutes(r *gin.RouterGroup) {
	r.PUT("/repos/:repo/projects/:projectName/access-policy", controllers.UpsertAccessPolicyForRepoAndProject)
	r.PUT("/orgs/:organisation/access-policy", controllers.UpsertAccessPolicyForOrg)
	r.PUT("/repos/:repo/projects/:projectName/plan-policy", controllers.UpsertPlanPolicyForRepoAndProject)
	r.PUT("/orgs/:organisation/plan-policy", controllers.UpsertPlanPolicyForOrg)
	r.PUT("/repos/:repo/projects/:projectName/drift-policy", controllers.UpsertDriftPolicyForRepoAndProject)
	r.PUT("/orgs/:organisation/drift-policy", controllers.UpsertDriftPolicyForOrg)
	r.POST("/tokens/issue-access-token", controllers.IssueAccessTokenForOrg)
}

// setupInternalRoutes configures internal routes
func (app *StrateApp) setupInternalRoutes(r *gin.Engine) {
	r.POST("_internal/update_repo_cache", middleware.InternalApiAuth(), app.diggerController.UpdateRepoCache)
	r.POST("_internal/api/create_user", middleware.InternalApiAuth(), app.diggerController.CreateUserInternal)
	r.POST("_internal/api/upsert_org", middleware.InternalApiAuth(), app.diggerController.UpsertOrgInternal)
}

// setupAPIRoutes configures API routes
func (app *StrateApp) setupAPIRoutes(r *gin.Engine) {
	apiGroup := r.Group("/api")
	apiGroup.Use(middleware.InternalApiAuth(), middleware.HeadersApiAuth())

	reposApiGroup := apiGroup.Group("/repos")
	reposApiGroup.GET("/", controllers.ListReposApi)
	reposApiGroup.GET("/:repo_id/jobs", controllers.GetJobsForRepoApi)

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

// setupProfiler initializes the profiler if enabled
func (app *StrateApp) setupProfiler() error {
	if !app.cfg.Server.Pprof.PeriodicEnabled {
		return nil
	}

	// Create profiles directory if it doesn't exist
	if err := os.MkdirAll(app.cfg.Server.Pprof.Dir, 0o755); err != nil {
		return fmt.Errorf("failed to create profiles directory: %v", err)
	}

	// Start periodic profiling goroutine
	intervalMinutes := app.cfg.Server.Pprof.IntervalMinutes
	app.profilingTicker = time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	return nil
}

// runProfiler handles periodic profiling
func (app *StrateApp) runProfiler(ctx context.Context) {
	if !app.cfg.Server.Pprof.PeriodicEnabled {
		return
	}

	for {
		select {
		case <-ctx.Done():
			app.profilingTicker.Stop()
			return
		case <-app.profilingTicker.C:
			// Trigger GC before taking memory profile
			runtime.GC()

			// Create memory profile
			timestamp := time.Now().Format("2006-01-02-15-04-05")
			memProfilePath := filepath.Join(app.cfg.Server.Pprof.Dir, fmt.Sprintf("memory-%s.pprof", timestamp))
			f, err := os.Create(memProfilePath)
			if err != nil {
				slog.Error("Failed to create memory profile", "error", err)
				continue
			}

			if err := pprof.WriteHeapProfile(f); err != nil {
				slog.Error("Failed to write memory profile", "error", err)
			}
			f.Close()

			// Cleanup old profiles
			app.cleanupOldProfiles(app.cfg.Server.Pprof.Dir, app.cfg.Server.Pprof.KeepProfiles)
		}
	}
}

// cleanupOldProfiles removes old profile files
func (app *StrateApp) cleanupOldProfiles(dir string, keep int) {
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

// Serve starts the application server
func (app *StrateApp) Serve() error {
	// Set up the application
	if err := app.setup(); err != nil {
		return fmt.Errorf("failed to set up application: %w", err)
	}

	// Set up profiling if enabled
	if err := app.setupProfiler(); err != nil {
		return fmt.Errorf("failed to set up profiler: %w", err)
	}

	slog.Info("Starting Strate Backend API",
		"version", version.Version,
		"commit", version.Meta,
		"port", app.cfg.Server.Port)

	// Create an error group to manage all goroutines
	g := new(errgroup.Group)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run profiler if enabled
	if app.cfg.Server.Pprof.PeriodicEnabled {
		g.Go(func() error {
			app.runProfiler(ctx)
			return nil
		})
	}

	// Set up and start HTTP server
	listenAddr := fmt.Sprintf(":%d", app.cfg.Server.Port)
	app.httpServer = &http.Server{
		Addr:         listenAddr,
		Handler:      app.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	g.Go(func() error {
		slog.Info("Server starting", "address", listenAddr)
		if err := app.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server error: %w", err)
		}
		return nil
	})

	// Handle shutdown signals
	g.Go(func() error {
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

		select {
		case sig := <-signalCh:
			slog.Info("Received signal", "signal", sig)
			cancel()

			// Shut down the HTTP server
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()

			slog.Info("Shutting down HTTP server...")
			if err := app.httpServer.Shutdown(shutdownCtx); err != nil {
				slog.Error("HTTP server shutdown error", "error", err)
			}

			return nil
		case <-ctx.Done():
			return nil
		}
	})

	// Wait for all goroutines to complete
	return g.Wait()
}
