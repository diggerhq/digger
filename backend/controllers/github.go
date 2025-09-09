package controllers

import (
	"context"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/logging"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v61/github"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
)

type IssueCommentHook func(gh utils.GithubClientProvider, payload *github.IssueCommentEvent, ciBackendProvider ci_backends.CiBackendProvider) error

type DiggerController struct {
	CiBackendProvider                  ci_backends.CiBackendProvider
	GithubClientProvider               utils.GithubClientProvider
	GithubWebhookPostIssueCommentHooks []IssueCommentHook
}

func (d DiggerController) GithubAppWebHook(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	gh := d.GithubClientProvider
	slog.Info("Processing GitHub app webhook")

	appID := c.GetHeader("X-GitHub-Hook-Installation-Target-ID")

	_, _, webhookSecret, _, err := d.GithubClientProvider.FetchCredentials(appID)

	payload, err := github.ValidatePayload(c.Request, []byte(webhookSecret))
	if err != nil {
		slog.Error("Error validating GitHub app webhook's payload", "appID", appID, "error", err)
		c.String(http.StatusBadRequest, "Error validating github app webhook's payload")
		return
	}

	webhookType := github.WebHookType(c.Request)
	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		slog.Error("Failed to parse GitHub event", "webhookType", webhookType, "error", err)
		c.String(http.StatusInternalServerError, "Failed to parse Github Event")
		return
	}

	slog.Info("Received GitHub event",
		"eventType", reflect.TypeOf(event),
		"webhookType", webhookType,
	)

	appId64, err := strconv.ParseInt(appID, 10, 64)
	if err != nil {
		slog.Error("Error converting appId string to int64", "appID", appID, "error", err)
		return
	}

	switch event := event.(type) {
	case *github.InstallationEvent:
		slog.Info("Processing InstallationEvent",
			"action", *event.Action,
			"installationId", *event.Installation.ID,
		)

		if *event.Action == "deleted" {
			err := handleInstallationDeletedEvent(event, appId64)
			if err != nil {
				slog.Error("Failed to handle installation deleted event", "error", err)
				c.String(http.StatusAccepted, "Failed to handle webhook event.")
				return
			}
		}
	case *github.PushEvent:
		slog.Info("Processing PushEvent",
			"repo", *event.Repo.FullName,
		)

		go func(ctx context.Context) {
			defer logging.InheritRequestLogger(ctx)()
			handlePushEvent(ctx, gh, event, appId64)
		}(c.Request.Context())

	case *github.IssueCommentEvent:
		slog.Info("Processing IssueCommentEvent",
			"action", *event.Action,
			"repo", *event.Repo.FullName,
			"issueNumber", *event.Issue.Number,
		)

		if event.Sender.Type != nil && *event.Sender.Type == "Bot" {
			slog.Debug("Ignoring bot comment", "senderType", *event.Sender.Type)
			c.String(http.StatusOK, "OK")
			return
		}
		go func(ctx context.Context) {
			defer logging.InheritRequestLogger(ctx)()
			handleIssueCommentEvent(gh, event, d.CiBackendProvider, appId64, d.GithubWebhookPostIssueCommentHooks)
		}(c.Request.Context())

	case *github.PullRequestEvent:
		slog.Info("Processing PullRequestEvent",
			"action", *event.Action,
			"repo", *event.Repo.FullName,
			"prNumber", *event.PullRequest.Number,
			"prId", *event.PullRequest.ID,
		)

		// run it as a goroutine to avoid timeouts
		go func(ctx context.Context) {
			defer logging.InheritRequestLogger(ctx)()
			handlePullRequestEvent(gh, event, d.CiBackendProvider, appId64)
		}(c.Request.Context())

	default:
		slog.Debug("Unhandled event type", "eventType", reflect.TypeOf(event))
	}

	c.JSON(http.StatusAccepted, "ok")
}

func (d DiggerController) GithubReposPage(c *gin.Context) {
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	if !exists {
		slog.Warn("Organisation ID not found in context")
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	slog.Info("Fetching GitHub repositories for organisation", "orgId", orgId)

	link, err := models.DB.GetGithubInstallationLinkForOrg(orgId)
	if err != nil {
		slog.Error("Failed to get GitHub installation link for organisation",
			"orgId", orgId,
			"error", err,
		)
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	slog.Debug("Found GitHub installation link",
		"orgId", orgId,
		"installationId", link.GithubInstallationId,
	)

	installations, err := models.DB.GetGithubAppInstallations(link.GithubInstallationId)
	if err != nil {
		slog.Error("Failed to get GitHub app installations",
			"installationId", link.GithubInstallationId,
			"error", err,
		)
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	if len(installations) == 0 {
		slog.Warn("No GitHub installations found",
			"installationId", link.GithubInstallationId,
			"orgId", orgId,
		)
		c.String(http.StatusForbidden, "Failed to find any GitHub installations for this org")
		return
	}

	slog.Debug("Found GitHub installations",
		"count", len(installations),
		"appId", installations[0].GithubAppId,
		"installationId", installations[0].GithubInstallationId,
	)

	gh := d.GithubClientProvider
	client, _, err := gh.Get(installations[0].GithubAppId, installations[0].GithubInstallationId)
	if err != nil {
		slog.Error("Failed to create GitHub client",
			"appId", installations[0].GithubAppId,
			"installationId", installations[0].GithubInstallationId,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating GitHub client"})
		return
	}

	slog.Debug("Successfully created GitHub client",
		"appId", installations[0].GithubAppId,
		"installationId", installations[0].GithubInstallationId,
	)

	opts := &github.ListOptions{}
	repos, _, err := client.Apps.ListRepos(context.Background(), opts)
	if err != nil {
		slog.Error("Failed to list GitHub repositories",
			"installationId", installations[0].GithubInstallationId,
			"error", err,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list GitHub repos."})
		return
	}

	slog.Info("Successfully retrieved GitHub repositories",
		"orgId", orgId,
		"repoCount", len(repos.Repositories),
	)

	c.HTML(http.StatusOK, "github_repos.tmpl", gin.H{"Repos": repos.Repositories})
}
