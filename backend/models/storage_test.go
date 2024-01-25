package models

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"strings"
	"testing"
)

func setupSuite(tb testing.TB) (func(tb testing.TB), *Database, *Organisation) {
	log.Println("setup suite")

	// database file name
	dbName := "database_storage_test.db"

	// remove old database
	e := os.Remove(dbName)
	if e != nil {
		if !strings.Contains(e.Error(), "no such file or directory") {
			log.Fatal(e)
		}
	}

	// open and create a new database
	gdb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal(err)
	}

	// migrate tables
	err = gdb.AutoMigrate(&Policy{}, &Organisation{}, &Repo{}, &Project{}, &Token{},
		&User{}, &ProjectRun{}, &GithubAppInstallation{}, &GithubApp{}, &GithubAppInstallationLink{},
		&GithubDiggerJobLink{}, &DiggerJob{}, &DiggerJobParentLink{})
	if err != nil {
		log.Fatal(err)
	}

	database := &Database{GormDB: gdb}
	DB = database

	// create an org
	orgTenantId := "11111111-1111-1111-1111-111111111111"
	externalSource := "test"
	orgName := "testOrg"
	org, err := database.CreateOrganisation(orgName, externalSource, orgTenantId)
	if err != nil {
		log.Fatal(err)
	}

	DB = database
	// Return a function to teardown the test
	return func(tb testing.TB) {
		log.Println("teardown suite")
		err = os.Remove(dbName)
		if err != nil {
			log.Fatal(err)
		}
	}, database, org
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
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
	batchType := scheduler.BatchTypePlan
	commentId := int64(123)
	jobSpec := "\x7b2270726f6a6563744e616d65223a22646576222c2270726f6a656374446972223a22\n646576222c2270726f6a656374576f726b7370616365223a2264656661756c74222c2274657272616772756e74223a66616c73652c22636f6d6d616e6473223a5b2264696767657220706c616e225d2c226170706c795374616765223a7b227374657073223a5b7b22616374696f6e\n223a22696e6974222c22657874726141726773223a5b5d7d2c7b22616374696f6e223a226170706c79222c22657874726141726773223a5b5d7d5d7d2c22706c616e5374616765223a7b227374657073223a5b7b22616374696f6e223a22696e6974222c2265787472614172677322\n3a5b5d7d2c7b22616374696f6e223a22706c616e222c22657874726141726773223a5b5d7d5d7d2c2270756c6c526571756573744e756d626572223a332c226576656e744e616d65223a2270756c6c5f72657175657374222c227265717565737465644279223a225a494a222c226e\n616d657370616365223a2264696767657268712f746573742d73656c66686f7374222c227374617465456e7656617273223a7b7d2c22636f6d6d616e64456e7656617273223a7b7d7d   "

	batch, err := DB.CreateDiggerBatch(123, repoOwner, repoName, repoFullName, prNumber, diggerconfig, branchName, batchType, &commentId)
	assert.NoError(t, err)

	job, err := DB.CreateDiggerJob(batch.ID, []byte(jobSpec))
	assert.NoError(t, err)

	job, err = DB.UpdateDiggerJobSummary(job.DiggerJobID, 1, 2, 3)
	assert.NoError(t, err)

	jobssss, err := DB.GetDiggerJobsForBatch(batch.ID)
	println(job.DiggerJobSummary.ResourcesCreated, job.DiggerJobSummary.ResourcesUpdated, job.DiggerJobSummary.ResourcesDeleted)
	println(jobssss[0].DiggerJobSummary.ResourcesCreated, jobssss[0].DiggerJobSummary.ResourcesUpdated, jobssss[0].DiggerJobSummary.ResourcesDeleted)

	fetchedBatch, err := DB.GetDiggerBatch(&batch.ID)
	assert.NoError(t, err)
	jsons, err := fetchedBatch.MapToJsonStruct()
	assert.NoError(t, err)
	spew.Dump(jsons)
}
