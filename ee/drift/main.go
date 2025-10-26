package main

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	utils2 "github.com/diggerhq/digger/backend/utils"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/ee/drift/controllers"
	"github.com/diggerhq/digger/ee/drift/middleware"
	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

func init() {
	logLevel := os.Getenv("DIGGER_LOG_LEVEL")
	var level slog.Leveler

	if logLevel == "DEBUG" {
		level = slog.LevelDebug
	} else if logLevel == "WARN" {
		level = slog.LevelWarn
	} else if logLevel == "ERROR" {
		level = slog.LevelError
	} else {
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
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
	models.ConnectDatabase()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(sloggin.New(logger))
	if sentryDsn != "" {
		r.Use(sentrygin.New(sentrygin.Options{}))
	}

	controller := controllers.MainController{
		GithubClientProvider: utils2.DiggerGithubRealClientProvider{},
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

	r.POST("/repos/:repo/projects/:projectName/jobs/:jobId/set-status", middleware.JobTokenAuth(), controller.SetJobStatusForProject)

	r.POST("/_internal/process_notifications", middleware.WebhookAuth(), controller.ProcessAllNotifications)
	r.POST("/_internal/send_slack_notification_for_org", middleware.WebhookAuth(), controller.SendRealSlackNotificationForOrg)
	r.POST("/_internal/send_test_slack_notification_for_url", middleware.WebhookAuth(), controller.SendTestSlackNotificationForUrl)
	r.POST("/_internal/send_teams_notification_for_org", middleware.WebhookAuth(), controller.SendRealTeamsNotificationForOrg)
	r.POST("/_internal/send_test_teams_notification_for_url", middleware.WebhookAuth(), controller.SendTestTeamsNotificationForUrl)

	r.POST("/_internal/process_drift", middleware.WebhookAuth(), controller.ProcessAllDrift)
	r.POST("/_internal/process_drift_for_org", middleware.WebhookAuth(), controller.ProcessDriftForOrg)
	r.POST("/_internal/trigger_drift_for_project", middleware.WebhookAuth(), controller.TriggerDriftRunForProject)

	port := os.Getenv("DIGGER_PORT")
	if port == "" {
		port = "3000"
	}
	r.Run(fmt.Sprintf(":%v", port))

}
