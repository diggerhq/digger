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

func TestSoftDeleteRepoAndProjects(t *testing.T) {
	teardownSuite, db, org := setupSuite(t)
	defer teardownSuite(t)

	installationId := int64(1)
	appId := int64(1)
	repoFullName := "test/test"

	repo, err := db.CreateRepo("test-test", repoFullName, "test", "test", "", org, "", installationId, appId, "main", "")
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	project := Project{
		Name:           "proj",
		OrganisationID: org.ID,
		Organisation:   org,
		RepoFullName:   repoFullName,
		Status:         ProjectActive,
	}
	err = db.GormDB.Create(&project).Error
	assert.NoError(t, err)

	err = db.SoftDeleteRepoAndProjects(org.ID, repoFullName)
	assert.NoError(t, err)

	// Verify repo is soft-deleted
	var repoRecord Repo
	err = db.GormDB.Unscoped().Where("id = ?", repo.ID).First(&repoRecord).Error
	assert.NoError(t, err)
	assert.True(t, repoRecord.DeletedAt.Valid)

	// Verify project is soft-deleted
	var projectRecord Project
	err = db.GormDB.Unscoped().Where("id = ?", project.ID).First(&projectRecord).Error
	assert.NoError(t, err)
	assert.True(t, projectRecord.DeletedAt.Valid)
}

func TestSoftDeleteReposAndProjectsByInstallation(t *testing.T) {
	teardownSuite, db, org := setupSuite(t)
	defer teardownSuite(t)

	appId := int64(1)
	installA := int64(1)
	installB := int64(2)

	repoA, err := db.CreateRepo("org-repo-a", "org/repo-a", "org", "repo-a", "", org, "", installA, appId, "main", "")
	assert.NoError(t, err)
	repoB, err := db.CreateRepo("org-repo-b", "org/repo-b", "org", "repo-b", "", org, "", installB, appId, "main", "")
	assert.NoError(t, err)

	projectA := Project{
		Name:           "proj-a",
		OrganisationID: org.ID,
		Organisation:   org,
		RepoFullName:   repoA.RepoFullName,
		Status:         ProjectActive,
	}
	projectB := Project{
		Name:           "proj-b",
		OrganisationID: org.ID,
		Organisation:   org,
		RepoFullName:   repoB.RepoFullName,
		Status:         ProjectActive,
	}
	assert.NoError(t, db.GormDB.Create(&projectA).Error)
	assert.NoError(t, db.GormDB.Create(&projectB).Error)

	// Soft-delete only repos for installA
	err = db.SoftDeleteReposAndProjectsByInstallation(org.ID, installA)
	assert.NoError(t, err)

	// Verify repoA is soft-deleted, repoB is not
	var repoARecord, repoBRecord Repo
	assert.NoError(t, db.GormDB.Unscoped().Where("id = ?", repoA.ID).First(&repoARecord).Error)
	assert.NoError(t, db.GormDB.Unscoped().Where("id = ?", repoB.ID).First(&repoBRecord).Error)
	assert.True(t, repoARecord.DeletedAt.Valid)
	assert.False(t, repoBRecord.DeletedAt.Valid)

	// Verify projectA is soft-deleted, projectB is not
	var projectARecord, projectBRecord Project
	assert.NoError(t, db.GormDB.Unscoped().Where("id = ?", projectA.ID).First(&projectARecord).Error)
	assert.NoError(t, db.GormDB.Unscoped().Where("id = ?", projectB.ID).First(&projectBRecord).Error)
	assert.True(t, projectARecord.DeletedAt.Valid)
	assert.False(t, projectBRecord.DeletedAt.Valid)
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

	job, err := DB.CreateDiggerJob(batch.ID, []byte(jobSpec), "workflow_file.yml", nil, nil, "lazy", "")
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

// setupImpactedSuite provisions an isolated DB migrating only ImpactedProject.
// It deliberately avoids the shared setupSuite because the Project and
// ImpactedProject models both declare a gorm index named "idx_org_repo", which
// collides under SQLite (global index namespace) during a combined AutoMigrate.
// Production schema is managed by Atlas migrations, not AutoMigrate.
func setupImpactedSuite(tb testing.TB) (func(tb testing.TB), *Database) {
	dbName := "database_impacted_test.db"
	if e := os.Remove(dbName); e != nil && !strings.Contains(e.Error(), "no such file or directory") {
		panic(e)
	}

	gdb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic(err)
	}
	if err = gdb.AutoMigrate(&ImpactedProject{}); err != nil {
		panic(err)
	}
	database := &Database{GormDB: gdb}
	DB = database
	return func(tb testing.TB) {
		if e := os.Remove(dbName); e != nil {
			panic(e)
		}
	}, database
}

func TestGetImpactedProjectSingleReturnsRequestedProject(t *testing.T) {
	teardownSuite, db := setupImpactedSuite(t)
	defer teardownSuite(t)

	repo := "acme/infra"
	sha := "abc123"
	projects := []string{"projectA", "projectB", "projectC"}

	for _, name := range projects {
		_, err := db.CreateImpactedProject(repo, sha, name, nil, nil)
		assert.NoError(t, err)
	}

	for _, name := range projects {
		ip, err := db.GetImpactedProjectSingle(repo, sha, name)
		assert.NoError(t, err)
		assert.NotNil(t, ip)
		assert.Equal(t, name, ip.ProjectName)
	}
}

func TestGetImpactedProjectSingleNotFound(t *testing.T) {
	teardownSuite, db := setupImpactedSuite(t)
	defer teardownSuite(t)

	ip, err := db.GetImpactedProjectSingle("acme/infra", "abc123", "missing")
	assert.NoError(t, err)
	assert.Nil(t, ip)
}

// Regression test for multi-project auto_merge: each project's apply must flip its
// own Applied flag so AllImpactedProjectApplied can reach true for N>1 projects.
func TestAllImpactedProjectAppliedMultiProject(t *testing.T) {
	teardownSuite, db := setupImpactedSuite(t)
	defer teardownSuite(t)

	repo := "acme/infra"
	sha := "abc123"
	projects := []string{"projectA", "projectB", "projectC"}

	for _, name := range projects {
		_, err := db.CreateImpactedProject(repo, sha, name, nil, nil)
		assert.NoError(t, err)
	}

	allApplied, _, err := db.AllImpactedProjectApplied(repo, sha)
	assert.NoError(t, err)
	assert.False(t, allApplied)

	for _, name := range projects {
		ip, err := db.GetImpactedProjectSingle(repo, sha, name)
		assert.NoError(t, err)
		ip.Applied = true
		assert.NoError(t, db.GormDB.Save(ip).Error)
	}

	allApplied, applied, err := db.AllImpactedProjectApplied(repo, sha)
	assert.NoError(t, err)
	assert.Len(t, applied, len(projects))
	assert.True(t, allApplied)
}
