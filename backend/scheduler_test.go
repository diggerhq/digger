package main

import (
	"github.com/diggerhq/digger/backend/models"
	"github.com/google/uuid"
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

func setupSuite(tb testing.TB) (func(tb testing.TB), *models.Database) {
	log.Println("setup suite")

	// database file name
	dbName := "database_test.db"

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
		&models.GithubDiggerJobLink{}, &models.DiggerJob{}, &models.DiggerJobParentLink{})
	if err != nil {
		log.Fatal(err)
	}

	database := &models.Database{GormDB: gdb}

	orgTenantId := "11111111-1111-1111-1111-111111111111"
	externalSource := "test"
	orgName := "testOrg"
	org, err := database.CreateOrganisation(orgName, externalSource, orgTenantId)
	if err != nil {
		log.Fatal(err)
	}

	repoName := "test repo"
	repo, err := database.CreateRepo(repoName, org, "")
	if err != nil {
		log.Fatal(err)
	}

	projectName := "test project"
	_, err = database.CreateProject(projectName, org, repo)
	if err != nil {
		log.Fatal(err)
	}

	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Println("teardown suite")
		e := os.Remove(dbName)
		if e != nil {
			if !strings.Contains(e.Error(), "no such file or directory") {
				log.Fatal(e)
			}
		}
	}, database
}

func TestCreateDiggerJob(t *testing.T) {
	teardownSuite, database := setupSuite(t)
	defer teardownSuite(t)

	batchId, _ := uuid.NewUUID()
	job, err := database.CreateDiggerJob(batchId, []byte{100})

	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.NotZero(t, job.ID)
}

func TestCreateSingleJob(t *testing.T) {
	teardownSuite, database := setupSuite(t)
	defer teardownSuite(t)

	batchId, _ := uuid.NewUUID()
	job, err := database.CreateDiggerJob(batchId, []byte{100})

	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.NotZero(t, job.ID)
}

func TestFindDiggerJobsByParentJobId(t *testing.T) {
	teardownSuite, database := setupSuite(t)
	defer teardownSuite(t)

	batchId, _ := uuid.NewUUID()
	job, err := database.CreateDiggerJob(batchId, []byte{100})
	parentJobId := job.DiggerJobId
	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.NotZero(t, job.ID)

	job, err = database.CreateDiggerJob(batchId, []byte{100})
	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.NotZero(t, job.ID)
	err = database.CreateDiggerJobParentLink(parentJobId, job.DiggerJobId)
	assert.Nil(t, err)

	job, err = database.CreateDiggerJob(batchId, []byte{100})
	assert.NoError(t, err)
	assert.NotNil(t, job)
	err = database.CreateDiggerJobParentLink(parentJobId, job.DiggerJobId)
	assert.Nil(t, err)
	assert.NotZero(t, job.ID)

	jobs, err := database.GetDiggerJobParentLinksByParentId(&parentJobId)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobs))
}
