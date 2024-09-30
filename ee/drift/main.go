package main

import (
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/ee/drift/controllers"
	"github.com/diggerhq/digger/ee/drift/dbmodels"
	"github.com/diggerhq/digger/ee/drift/middleware"
	next_utils "github.com/diggerhq/digger/next/utils"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
	"log"
	"log/slog"
	"net/http"
	"os"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Initialized the logger successfully")
}

var Version = "dev"

func main() {

	sentryDsn := os.Getenv("SENTRY_DSN")
	if sentryDsn != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              sentryDsn,
			EnableTracing:    true,
			AttachStacktrace: true,
			// Set TracesSampleRate to 1.0 to capture 100%
			// of transactions for performance monitoring.
			// We recommend adjusting this value in production,
			TracesSampleRate: 0.1,
			Release:          "api@" + Version,
			Debug:            true,
		}); err != nil {
			log.Printf("Sentry initialization failed: %v\n", err)
		}
	}

	// initialize the database
	dbmodels.ConnectDatabase()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(sloggin.New(logger))
	if sentryDsn != "" {
		r.Use(sentrygin.New(sentrygin.Options{}))
	}

	controller := controllers.MainController{
		GithubClientProvider: next_utils.DiggerGithubRealClientProvider{},
		CiBackendProvider:    ci_backends.DefaultBackendProvider{},
	}

	r.GET("/ping", controller.Ping)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version":    Version,
			"commit_sha": Version,
		})
	})
	//authorized := r.Group("/")
	//authorized.Use(middleware.GetApiMiddleware(), middleware.AccessLevel(dbmodels.CliJobAccessType, dbmodels.AccessPolicyType, models.AdminPolicyType))

	r.POST("github-app-webhook", controller.GithubAppWebHook)
	r.GET("/github/callback_fe", middleware.WebhookAuth(), controller.GithubAppCallbackPage)

	r.POST("/_internal/trigger_drift_for_project", middleware.WebhookAuth(), controller.TriggerDriftRunForProject)

	port := os.Getenv("DIGGER_PORT")
	if port == "" {
		port = "3000"
	}
	r.Run(fmt.Sprintf(":%v", port))

}
