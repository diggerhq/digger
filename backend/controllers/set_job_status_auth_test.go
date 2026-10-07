package controllers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func serializedJobSpecWithToken(t *testing.T, token string) []byte {
	t.Helper()
	spec, err := json.Marshal(orchestrator_scheduler.JobJson{BackendJobToken: token})
	require.NoError(t, err)
	return spec
}

func TestJobTokenMatchesJob(t *testing.T) {
	job := &models.DiggerJob{SerializedJobSpec: serializedJobSpecWithToken(t, "cli:job-a")}

	assert.True(t, jobTokenMatchesJob("cli:job-a", job))
	assert.False(t, jobTokenMatchesJob("cli:job-b", job), "token issued for another job")
	assert.False(t, jobTokenMatchesJob("", job), "no job token on the request")
	assert.False(t, jobTokenMatchesJob("cli:job-a", &models.DiggerJob{}), "job without a spec")
	assert.False(t, jobTokenMatchesJob("cli:job-a", &models.DiggerJob{SerializedJobSpec: serializedJobSpecWithToken(t, "")}), "spec without a token")
	assert.False(t, jobTokenMatchesJob("cli:job-a", &models.DiggerJob{SerializedJobSpec: []byte("not json")}), "unreadable spec")
}

// setJobStatusTestJob stores a batch and one job whose spec carries jobToken,
// in a fresh SQLite database.
func setJobStatusTestJob(t *testing.T, jobToken string) *models.DiggerJob {
	t.Helper()

	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.DiggerBatch{}, &models.DiggerJob{}, &models.DiggerJobSummary{}))

	previousDB := models.DB
	models.DB = &models.Database{GormDB: gdb}
	t.Cleanup(func() { models.DB = previousDB })

	batch, err := models.DB.CreateDiggerBatch(models.DiggerVCSGithub, 1, "owner", "repo", "owner/repo", 1, "", "main",
		orchestrator_scheduler.DiggerCommandPlan, nil, 0, "", false, true, nil, "sha", nil, nil)
	require.NoError(t, err)
	job, err := models.DB.CreateDiggerJob(batch.ID, serializedJobSpecWithToken(t, jobToken), "digger_workflow.yml", nil, nil, "lazy", "dev")
	require.NoError(t, err)
	return job
}

func setJobStatusWithJobToken(t *testing.T, jobId string, token string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/repos/owner-repo/projects/dev/jobs/"+jobId+"/set-status",
		strings.NewReader(`{"status":"succeeded"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "jobId", Value: jobId}}
	c.Set(middleware.ORGANISATION_ID_KEY, uint(1))
	c.Set(middleware.ACCESS_LEVEL_KEY, models.CliJobAccessType)
	c.Set(middleware.JOB_TOKEN_KEY, token)

	DiggerController{}.SetJobStatusForProject(c)
	return w
}

func TestSetJobStatusRejectsJobTokenForAnotherJob(t *testing.T) {
	job := setJobStatusTestJob(t, "cli:job-a")

	w := setJobStatusWithJobToken(t, job.DiggerJobID, "cli:job-b")
	assert.Equal(t, http.StatusForbidden, w.Code)

	stored, err := models.DB.GetDiggerJob(job.DiggerJobID)
	require.NoError(t, err)
	assert.Equal(t, job.Status, stored.Status, "job status must not change")
}

func TestSetJobStatusReturnsNotFoundForUnknownJob(t *testing.T) {
	setJobStatusTestJob(t, "cli:job-a")

	w := setJobStatusWithJobToken(t, "does-not-exist", "cli:job-a")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSetJobStatusAcceptsJobTokenForItsOwnJob(t *testing.T) {
	job := setJobStatusTestJob(t, "cli:job-a")

	// The test database has no organisations table, so the handler fails
	// with 500 when it looks one up, after the token check let the request
	// through.
	w := setJobStatusWithJobToken(t, job.DiggerJobID, "cli:job-a")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
