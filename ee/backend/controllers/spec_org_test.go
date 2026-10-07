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
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGetSpecIssuesJobTokenForCallersOrganisation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	gdb, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.Organisation{}, &models.JobToken{}))
	previousDB := models.DB
	models.DB = &models.Database{GormDB: gdb}
	t.Cleanup(func() { models.DB = previousDB })

	_, err = models.DB.CreateOrganisation("digger", "", models.DEFAULT_ORG_NAME, nil)
	require.NoError(t, err)
	tenant, err := models.DB.CreateOrganisation("tenant", "", "tenant", nil)
	require.NoError(t, err)

	config, err := json.Marshal(digger_config.DiggerConfig{
		Workflows: map[string]digger_config.Workflow{"default": {
			Plan:          &digger_config.Stage{},
			Apply:         &digger_config.Stage{},
			Configuration: &digger_config.WorkflowConfiguration{},
		}},
	})
	require.NoError(t, err)
	project, err := json.Marshal(digger_config.Project{Name: "dev", Dir: ".", Workflow: "default", WorkflowFile: "digger_workflow.yml"})
	require.NoError(t, err)
	payload, err := json.Marshal(spec.GetSpecPayload{
		Command:      "digger plan",
		RepoFullName: "owner/repo",
		Actor:        "someone",
		DiggerConfig: string(config),
		Project:      string(project),
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/get-spec", strings.NewReader(string(payload)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(middleware.ORGANISATION_ID_KEY, tenant.ID)

	DiggerEEController{}.GetSpec(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var issued spec.Spec
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &issued))
	jobToken, err := models.DB.GetJobToken(issued.Job.BackendJobToken)
	require.NoError(t, err)
	require.NotNil(t, jobToken)
	assert.Equal(t, tenant.ID, jobToken.OrganisationID)
}
