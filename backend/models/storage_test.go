package models

import (
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
	gdb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
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
