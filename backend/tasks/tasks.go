package main

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
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
	c.AddFunc("* * * * *", func() {
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

	// Start the Cron job scheduler
	c.Start()

	for {
	}

}
