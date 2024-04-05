package main

import (
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"os"
	"strings"
	"testing"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

type MockCiBackend struct {
}

func (m MockCiBackend) TriggerWorkflow(repoOwner string, repoName string, job models.DiggerJob, jobString string, commentId int64) error {
	return nil
}

func setupSuite(tb testing.TB) (func(tb testing.TB), *models.Database) {
	log.Println("setup suite")

	// database file name
	dbName := "database_tasks_test.db"

	// remove old database
	e := os.Remove(dbName)
	if e != nil {
		if !strings.Contains(e.Error(), "no such file or directory") {
			log.Fatal(e)
		}
	}

	// open and create a new database
	gdb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// migrate tables
	err = gdb.AutoMigrate(&models.Policy{}, &models.Organisation{}, &models.Repo{}, &models.Project{}, &models.Token{},
		&models.User{}, &models.ProjectRun{}, &models.GithubAppInstallation{}, &models.GithubApp{}, &models.GithubAppInstallationLink{},
		&models.GithubDiggerJobLink{}, &models.DiggerJob{}, &models.DiggerJobParentLink{}, &models.DiggerRun{}, &models.DiggerRunQueueItem{})
	if err != nil {
		log.Fatal(err)
	}

	database := &models.Database{GormDB: gdb}
	models.DB = database

	// create an org
	orgTenantId := "11111111-1111-1111-1111-111111111111"
	externalSource := "test"
	orgName := "testOrg"
	org, err := database.CreateOrganisation(orgName, externalSource, orgTenantId)
	if err != nil {
		log.Fatal(err)
	}

	// create digger repo
	repoName := "test repo"
	repo, err := database.CreateRepo(repoName, "", "", "", "", org, "")
	if err != nil {
		log.Fatal(err)
	}

	// create test project
	projectName := "test project"
	_, err = database.CreateProject(projectName, org, repo)
	if err != nil {
		log.Fatal(err)
	}

	models.DB = database
	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Println("teardown suite")
		err = os.Remove(dbName)
		if err != nil {
			log.Fatal(err)
		}
	}, database
}

func TestThatRunQueueItemMovesFromQueuedToPlanningAfterPickup(t *testing.T) {
	teardownSuite, _ := setupSuite(t)
	defer teardownSuite(t)

	type params struct {
		BatchStatus        orchestrator_scheduler.DiggerBatchStatus
		InitialStatus      models.DiggerRunStatus
		NextExpectedStatus models.DiggerRunStatus
	}

	testParameters := []params{
		{
			BatchStatus:        orchestrator_scheduler.BatchJobSucceeded,
			InitialStatus:      models.RunQueued,
			NextExpectedStatus: models.RunPlanning,
		},
		// TODO: Uncomment when functionality working
		//{
		//	BatchStatus:        orchestrator_scheduler.BatchJobFailed,
		//	InitialStatus:      models.RunPlanning,
		//	NextExpectedStatus: models.RunFailed,
		//},
		{
			BatchStatus:        orchestrator_scheduler.BatchJobSucceeded,
			InitialStatus:      models.RunPlanning,
			NextExpectedStatus: models.RunPendingApproval,
		},
		{
			BatchStatus:        orchestrator_scheduler.BatchJobSucceeded,
			InitialStatus:      models.RunApproved,
			NextExpectedStatus: models.RunApplying,
		},
		// TODO: uncomment when functional
		//{
		//	BatchStatus:        orchestrator_scheduler.BatchJobFailed,
		//	InitialStatus:      models.RunApplying,
		//	NextExpectedStatus: models.RunFailed,
		//},
		{
			BatchStatus:        orchestrator_scheduler.BatchJobSucceeded,
			InitialStatus:      models.RunApplying,
			NextExpectedStatus: models.RunSucceeded,
		},
	}

	for i, testParam := range testParameters {
		ciBackend := MockCiBackend{}
		batch, _ := models.DB.CreateDiggerBatch(123, "", "", "", 22, "", "", "", nil)
		project, _ := models.DB.CreateProject(fmt.Sprintf("test%v", i), nil, nil)
		diggerRun, _ := models.DB.CreateDiggerRun("", 1, testParam.InitialStatus, "sha", "", 123, 1, project.Name, "apply", nil, nil)
		queueItem, _ := models.DB.CreateDiggerRunQueueItem(diggerRun.ID)
		batch.Status = testParam.BatchStatus
		models.DB.UpdateDiggerBatch(batch)
		queueItem, _ = models.DB.GetDiggerRunQueueItem(queueItem.ID)

		RunQueuesStateMachine(queueItem, ciBackend)
		diggerRunRefreshed, _ := models.DB.GetDiggerRun(diggerRun.ID)
		assert.Equal(t, testParam.NextExpectedStatus, diggerRunRefreshed.Status)
	}

}
