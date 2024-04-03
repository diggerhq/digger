package main

import (
	"github.com/diggerhq/digger/backend/models"
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
			log.Printf("Error fetching Latest queue runs: %v", err)
			return
		}

		for _, queue := range runQueues {
			log.Printf("%v, %v, %v, %v\n", queue.DiggerRunId, queue.ProjectId, queue.DiggerRun.Status, queue.Project.Name)
		}
	})

	// Start the Cron job scheduler
	c.Start()

	for {
	}

}
