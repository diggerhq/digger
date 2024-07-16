package main

import (
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/robfig/cron"
	"log"
	"os"
)

func initLogging() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Initialized the logger successfully")
}

func main() {
	initLogging()
	models.ConnectDatabase()

	c := cron.New()

	// RunQueues state machine
	c.AddFunc("0 * * * * *", func() {
		runQueues, err := models.DB.GetFirstRunQueueForEveryProject()
		if err != nil {
			log.Printf("Error fetching Latest queueItem runs: %v", err)
			return
		}

		for _, queueItem := range runQueues {
			dr := queueItem.DiggerRun
			repo := dr.Repo
			repoFullName := repo.RepoFullName
			repoOwner := repo.RepoOrganisation
			repoName := repo.RepoName
			service, _, err := utils.GetGithubService(&utils.DiggerGithubRealClientProvider{}, dr.GithubInstallationId, repoFullName, repoOwner, repoName)
			if err != nil {
				log.Printf("failed to get github service for DiggerRun ID: %v: %v", dr.ID, err)
				continue
			}
			RunQueuesStateMachine(&queueItem, service)
		}
	})

	// Triggered queued jobs for a batch
	c.AddFunc("30 * * * * *", func() {
		jobs, err := models.DB.GetDiggerJobsWithStatus(scheduler.DiggerJobQueuedForRun)
		if err != nil {
			log.Printf("Failed to get Jobs %v", err)
		}
		for _, job := range jobs {
			batch := job.Batch
			repoFullName := batch.RepoFullName
			repoName := batch.RepoName
			repoOwner := batch.RepoOwner
			githubInstallationid := batch.GithubInstallationId
			service, _, err := utils.GetGithubService(&utils.DiggerGithubRealClientProvider{}, githubInstallationid, repoFullName, repoOwner, repoName)
			if err != nil {
				log.Printf("Failed to get github service: %v", err)
			}

			ciBackend := ci_backends.GithubActionCi{Client: service.Client}
			services.ScheduleJob(ciBackend, repoFullName, repoOwner, repoName, &batch.ID, &job, &utils.DiggerGithubRealClientProvider{})
		}
	})

	// Start the Cron job scheduler
	c.Start()

	for {
	}

}
