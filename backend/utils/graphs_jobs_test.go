package utils

import (
	"os"
	"testing"

	"github.com/diggerhq/digger/backend/models"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Regression test: a child with multiple predecessors must result in exactly
// one DiggerJob row linked to all parent jobs, so that triggering waits for
// every predecessor. Previously one child job row was created per parent edge.
func TestConvertJobsToDiggerJobsMultiParentChildSingleJob(t *testing.T) {
	dbName := "database_convert_jobs_test.db"
	if _, err := os.Stat(dbName); err == nil {
		err := os.Remove(dbName)
		assert.NoError(t, err)
	}

	gdb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	assert.NoError(t, err)

	err = gdb.AutoMigrate(&models.Policy{}, &models.Organisation{}, &models.Repo{}, &models.Project{}, &models.Token{},
		&models.User{}, &models.ProjectRun{}, &models.GithubAppInstallation{}, &models.VCSConnection{}, &models.GithubAppInstallationLink{},
		&models.GithubDiggerJobLink{}, &models.DiggerBatch{}, &models.DiggerJob{}, &models.DiggerJobParentLink{}, &models.JobToken{}, &models.DiggerLock{})
	assert.NoError(t, err)

	previousDB := models.DB
	models.DB = &models.Database{GormDB: gdb}
	defer func() {
		models.DB = previousDB
		err := os.Remove(dbName)
		assert.NoError(t, err)
	}()

	org := models.Organisation{Name: "test-org", ExternalSource: "github", ExternalId: "test-org"}
	assert.NoError(t, gdb.Create(&org).Error)

	p1 := configuration.Project{Name: "p1"}
	p2 := configuration.Project{Name: "p2"}
	p3 := configuration.Project{Name: "p3"}
	child := configuration.Project{Name: "child", DependencyProjects: []string{"p1", "p2", "p3"}}
	projects := []configuration.Project{p1, p2, p3, child}

	projectsGraph, err := configuration.CreateProjectDependencyGraph(projects)
	assert.NoError(t, err)

	projectMap := map[string]configuration.Project{"p1": p1, "p2": p2, "p3": p3, "child": child}
	jobsMap := map[string]scheduler.Job{}
	for _, project := range projects {
		jobsMap[project.Name] = scheduler.Job{ProjectName: project.Name}
	}

	batchId, convertedJobs, err := ConvertJobsToDiggerJobs(scheduler.DiggerCommandPlan, "noop", models.DiggerVCSGithub, org.ID, jobsMap, projectMap, projectsGraph, 12345, "branch", 1, "owner", "repo", "owner/repo", "commit-sha", nil, "digger-config", 0, "", false, true, nil, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, batchId)
	assert.Len(t, convertedJobs, 4)

	jobsForBatch, err := models.DB.GetDiggerJobsForBatch(*batchId)
	assert.NoError(t, err)
	assert.Len(t, jobsForBatch, 4, "expected one job row per project")

	childJobs := 0
	for _, job := range jobsForBatch {
		if job.ProjectName == "child" {
			childJobs++
		}
	}
	assert.Equal(t, 1, childJobs, "expected exactly one job row for the multi-parent child")

	childJob, ok := convertedJobs["child"]
	assert.True(t, ok, "expected child job in converted jobs map")

	parentLinks, err := models.DB.GetDiggerJobParentLinksChildId(&childJob.DiggerJobID)
	assert.NoError(t, err)
	assert.Len(t, parentLinks, 3, "expected the single child job to be linked to all three parents")
}
