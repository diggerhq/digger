package main

import (
	"log/slog"
	"os"

	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/robfig/cron"

	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/diggerhq/digger/backend/utils"
)

func initLogging() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func main() {
	initLogging()
	slog.Info("Starting Digger tasks scheduler")

	models.ConnectDatabase()
	slog.Info("Database connection established")

	c := cron.New()

	// RunQueues state machine
	c.AddFunc("0 * * * * *", func() {
		slog.Info("Running RunQueues state machine task")

		runQueues, err := models.DB.GetFirstRunQueueForEveryProject()
		if err != nil {
			slog.Error("Error fetching latest queue item runs", "error", err)
			return
		}

		slog.Debug("Processing run queue items", "count", len(runQueues))

		for _, queueItem := range runQueues {
			dr := queueItem.DiggerRun
			repo := dr.Repo

			slog.Debug("Processing run queue item",
				"diggerRunId", dr.ID,
				slog.Group("repository",
					slog.String("fullName", repo.RepoFullName),
					slog.String("owner", repo.RepoOrganisation),
					slog.String("name", repo.RepoName),
				),
				"installationId", dr.GithubInstallationId,
			)

			repoFullName := repo.RepoFullName
			repoOwner := repo.RepoOrganisation
			repoName := repo.RepoName

			service, _, err := utils.GetGithubService(
				&utils.DiggerGithubRealClientProvider{},
				dr.GithubInstallationId,
				repoFullName,
				repoOwner,
				repoName,
			)
			if err != nil {
				slog.Error("Failed to get GitHub service for DiggerRun",
					"diggerRunId", dr.ID,
					"repoFullName", repoFullName,
					"error", err,
				)
				continue
			}

			RunQueuesStateMachine(&queueItem, service, &utils.DiggerGithubRealClientProvider{})
		}

		slog.Info("Completed RunQueues state machine task", "processedItems", len(runQueues))
	})

	// Trigger queued jobs for a batch
	c.AddFunc("30 * * * * *", func() {
		slog.Info("Running trigger queued jobs task")

		jobs, err := models.DB.GetDiggerJobsWithStatus(scheduler.DiggerJobQueuedForRun)
		if err != nil {
			slog.Error("Failed to get queued jobs", "error", err)
			return
		}

		slog.Debug("Processing queued jobs", "count", len(jobs))

		processedCount := 0
		for _, job := range jobs {
			batch := job.Batch

			slog.Debug("Processing queued job",
				"jobId", job.DiggerJobID,
				"batchId", batch.ID,
				slog.Group("repository",
					slog.String("fullName", batch.RepoFullName),
					slog.String("owner", batch.RepoOwner),
					slog.String("name", batch.RepoName),
				),
				"installationId", batch.GithubInstallationId,
			)

			repoFullName := batch.RepoFullName
			repoName := batch.RepoName
			repoOwner := batch.RepoOwner
			githubInstallationId := batch.GithubInstallationId

			service, _, err := utils.GetGithubService(
				&utils.DiggerGithubRealClientProvider{},
				githubInstallationId,
				repoFullName,
				repoOwner,
				repoName,
			)
			if err != nil {
				slog.Error("Failed to get GitHub service for job",
					"jobId", job.DiggerJobID,
					"repoFullName", repoFullName,
					"error", err,
				)
				continue
			}

			ciBackend := ci_backends.GithubActionCi{Client: service.Client}
			err = services.ScheduleJob(
				ciBackend,
				repoFullName,
				repoOwner,
				repoName,
				&batch.ID,
				&job,
				&utils.DiggerGithubRealClientProvider{},
			)

			if err != nil {
				slog.Error("Failed to schedule job",
					"jobId", job.DiggerJobID,
					"batchId", batch.ID,
					"error", err,
				)
			} else {
				processedCount++
				slog.Info("Successfully scheduled job",
					"jobId", job.DiggerJobID,
					"batchId", batch.ID,
				)
			}
		}

		slog.Info("Completed trigger queued jobs task",
			"totalJobs", len(jobs),
			"successfullyProcessed", processedCount,
		)
	})

	// Start the Cron job scheduler
	slog.Info("Starting cron scheduler")
	c.Start()

	slog.Info("Digger tasks scheduler is running")

	// Keep the application running
	// TODO: Add proper channels instead of this approach that consumes CPU cycles
	for {
	}
}
