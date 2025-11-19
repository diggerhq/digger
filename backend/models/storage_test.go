package models

import (
	"os"
	"strings"
	"testing"

	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupSuite(tb testing.TB) (func(tb testing.TB), *Database, *Organisation) {
	// database file name
	dbName := "database_storage_test.db"

	// remove old database
	e := os.Remove(dbName)
	if e != nil {
		if !strings.Contains(e.Error(), "no such file or directory") {
			panic(e)
		}
	}

	// open and create a new database
	gdb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic(err)
	}

	// migrate tables
	err = gdb.AutoMigrate(&Policy{}, &Organisation{}, &Repo{}, &Project{}, &Token{},
		&User{}, &ProjectRun{}, &GithubAppInstallation{}, &VCSConnection{}, &GithubAppInstallationLink{},
		&GithubDiggerJobLink{}, &DiggerJob{}, &DiggerJobParentLink{}, &DiggerLock{})
	if err != nil {
		panic(err)
	}

	database := &Database{GormDB: gdb}
	DB = database

	// create an org
	orgTenantId := "11111111-1111-1111-1111-111111111111"
	externalSource := "test"
	orgName := "testOrg"
	org, err := database.CreateOrganisation(orgName, externalSource, orgTenantId, nil)
	if err != nil {
		panic(err)
	}

	DB = database
	// Return a function to teardown the test
	return func(tb testing.TB) {
		err = os.Remove(dbName)
		if err != nil {
			panic(err)
		}
	}, database, org
}

func TestCreateGithubInstallationLink(t *testing.T) {
	teardownSuite, _, org := setupSuite(t)
	defer teardownSuite(t)

	installationId := int64(1)

	link, err := DB.CreateGithubInstallationLink(org, installationId)
	assert.NoError(t, err)
	assert.NotNil(t, link)

	link2, err := DB.CreateGithubInstallationLink(org, installationId)
	assert.NoError(t, err)
	assert.NotNil(t, link2)
	assert.Equal(t, link.ID, link2.ID)
}

func TestGithubRepoAdded(t *testing.T) {
	teardownSuite, _, _ := setupSuite(t)
	defer teardownSuite(t)

	installationId := int64(1)
	appId := int64(1)
	accountId := int64(1)
	login := "test"
	repoFullName := "test/test"

	i, err := DB.GithubRepoAdded(installationId, appId, login, accountId, repoFullName)
	assert.NoError(t, err)
	assert.NotNil(t, i)

	i2, err := DB.GithubRepoAdded(installationId, appId, login, accountId, repoFullName)
	assert.NoError(t, err)
	assert.NotNil(t, i)
	assert.Equal(t, i.ID, i2.ID)
	assert.Equal(t, GithubAppInstallActive, i.Status)
}

func TestGithubRepoRemoved(t *testing.T) {
	teardownSuite, _, _ := setupSuite(t)
	defer teardownSuite(t)

	installationId := int64(1)
	appId := int64(1)
	accountId := int64(1)
	login := "test"
	repoFullName := "test/test"

	i, err := DB.GithubRepoAdded(installationId, appId, login, accountId, repoFullName)
	assert.NoError(t, err)
	assert.NotNil(t, i)

	i, err = DB.GithubRepoRemoved(installationId, appId, repoFullName)
	assert.NoError(t, err)
	assert.NotNil(t, i)
	assert.Equal(t, GithubAppInstallDeleted, i.Status)

	i2, err := DB.GithubRepoAdded(installationId, appId, login, accountId, repoFullName)
	assert.NoError(t, err)
	assert.NotNil(t, i)
	assert.Equal(t, i.ID, i2.ID)
	assert.Equal(t, GithubAppInstallDeleted, i.Status)
}

func TestGetDiggerJobsForBatchPreloadsSummary(t *testing.T) {
	teardownSuite, _, _ := setupSuite(t)
	defer teardownSuite(t)

	prNumber := 123
	repoName := "test"
	repoOwner := "test"
	repoFullName := "test/test"
	diggerconfig := ""
	branchName := "main"
	batchType := scheduler.DiggerCommandPlan
	commentId := int64(123)
	jobSpec := "abc"

	resourcesCreated := uint(1)
	resourcesUpdated := uint(2)
	resourcesDeleted := uint(3)

	batch, err := DB.CreateDiggerBatch(DiggerVCSGithub, 123, repoOwner, repoName, repoFullName, prNumber, diggerconfig, branchName, batchType, &commentId, 0, "", false, true, nil, "", nil, nil)
	assert.NoError(t, err)

	job, err := DB.CreateDiggerJob(batch.ID, []byte(jobSpec), "workflow_file.yml", nil, nil)
	assert.NoError(t, err)

	job, err = DB.UpdateDiggerJobSummary(job.DiggerJobID, resourcesCreated, resourcesUpdated, resourcesDeleted)
	assert.NoError(t, err)

	jobssss, err := DB.GetDiggerJobsForBatch(batch.ID)
	assert.Equal(t, jobssss[0].DiggerJobSummary.ResourcesCreated, resourcesCreated)
	assert.Equal(t, jobssss[0].DiggerJobSummary.ResourcesUpdated, resourcesUpdated)
	assert.Equal(t, jobssss[0].DiggerJobSummary.ResourcesDeleted, resourcesDeleted)
}

func TestDiggerLockFunctionalities(t *testing.T) {
	teardownSuite, _, _ := setupSuite(t)
	defer teardownSuite(t)

	DB.CreateDiggerLock("org/repo1#dev", 1, 1)
	DB.CreateDiggerLock("org/repo1#staging", 1, 1)
	DB.CreateDiggerLock("org/repo1#prod", 1, 1)

	DB.CreateDiggerLock("org/repo2#dev", 1, 1)
	DB.CreateDiggerLock("org/repo2#prod", 1, 1)

	existingLocks, err := DB.GetLocksForOrg(1)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(existingLocks))

	DB.DeleteAllLocksAcquiredByPR(1, "org/repo1", 1)

	existingLocksAfterDeletion, err := DB.GetLocksForOrg(1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(existingLocksAfterDeletion))
	assert.Equal(t, "org/repo2#dev", existingLocksAfterDeletion[0].Resource)
	assert.Equal(t, "org/repo2#prod", existingLocksAfterDeletion[1].Resource)
}
