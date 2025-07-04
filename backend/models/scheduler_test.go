package models

import (
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSuiteScheduler(tb testing.TB) (func(tb testing.TB), *Database) {
	// database file name
	dbName := "database_test.db"

	// remove old database
	e := os.Remove(dbName)
	if e != nil {
		if !strings.Contains(e.Error(), "no such file or directory") {
			panic(e)
		}
	}

	// open and create a new database
	gdb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	// migrate tables
	err = gdb.AutoMigrate(&Policy{}, &Organisation{}, &Repo{}, &Project{}, &Token{},
		&User{}, &ProjectRun{}, &GithubAppInstallation{}, &VCSConnection{}, &GithubAppInstallationLink{},
		&GithubDiggerJobLink{}, &DiggerJob{}, &DiggerJobParentLink{})
	if err != nil {
		panic(err)
	}

	database := &Database{GormDB: gdb}

	orgTenantId := "11111111-1111-1111-1111-111111111111"
	externalSource := "test"
	orgName := "testOrg"
	org, err := database.CreateOrganisation(orgName, externalSource, orgTenantId)
	if err != nil {
		panic(err)
	}

	repoName := "test repo"
	repo, err := database.CreateRepo(repoName, "", "", "", "", org, "")
	if err != nil {
		panic(err)
	}

	projectName := "test project"
	_, err = database.CreateProject(projectName, "", org, repo.RepoFullName, false, false)
	if err != nil {
		panic(err)
	}

	// Return a function to teardown the test
	return func(tb testing.TB) {
		e := os.Remove(dbName)
		if e != nil {
			if !strings.Contains(e.Error(), "no such file or directory") {
				panic(e)
			}
		}
	}, database
}

func TestCreateDiggerJob(t *testing.T) {
	teardownSuite, database := setupSuiteScheduler(t)
	defer teardownSuite(t)

	batchId, _ := uuid.NewUUID()
	job, err := database.CreateDiggerJob(batchId, []byte{100}, "digger_workflow.yml")

	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.NotZero(t, job.ID)
}

func TestCreateSingleJob(t *testing.T) {
	teardownSuite, database := setupSuiteScheduler(t)
	defer teardownSuite(t)

	batchId, _ := uuid.NewUUID()
	job, err := database.CreateDiggerJob(batchId, []byte{100}, "digger_workflow.yml")

	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.NotZero(t, job.ID)
}

func TestFindDiggerJobsByParentJobId(t *testing.T) {
	teardownSuite, database := setupSuiteScheduler(t)
	defer teardownSuite(t)

	batchId, _ := uuid.NewUUID()
	job, err := database.CreateDiggerJob(batchId, []byte{100}, "digger_workflow.yml")
	parentJobId := job.DiggerJobID
	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.NotZero(t, job.ID)

	job, err = database.CreateDiggerJob(batchId, []byte{100}, "digger_workflow.yml")
	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.NotZero(t, job.ID)
	err = database.CreateDiggerJobParentLink(parentJobId, job.DiggerJobID)
	assert.Nil(t, err)

	job, err = database.CreateDiggerJob(batchId, []byte{100}, "digger_workflow.yml")
	assert.NoError(t, err)
	assert.NotNil(t, job)
	err = database.CreateDiggerJobParentLink(parentJobId, job.DiggerJobID)
	assert.Nil(t, err)
	assert.NotZero(t, job.ID)

	jobs, err := database.GetDiggerJobParentLinksByParentId(&parentJobId)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(jobs))
}
