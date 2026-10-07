package controllers

import (
	"testing"
	"time"

	"github.com/diggerhq/digger/backend/models"
	"github.com/stretchr/testify/assert"
)

func TestProjectDriftCheckFailedRecordsFailureAndRecovers(t *testing.T) {
	teardownSuite, _ := setupSuite(t)
	defer teardownSuite(t)

	org := models.Organisation{Name: "test-org", ExternalId: "test-org-external-id"}
	models.DB.GormDB.Create(&org)
	project := models.Project{
		Name:           "test-drift-project",
		OrganisationID: org.ID,
		RepoFullName:   "org/repo",
		DriftEnabled:   true,
		DriftStatus:    models.DriftStatusNewDrift,
		DriftToCreate:  2,
		DriftToUpdate:  1,
		DriftToDelete:  0,
	}
	models.DB.GormDB.Create(&project)

	err := ProjectDriftCheckFailed(project, "Error: plan failed: something broke")
	assert.NoError(t, err)

	var failed models.Project
	models.DB.GormDB.First(&failed, project.ID)
	assert.Equal(t, models.DriftStatusCheckFailed, failed.DriftStatus)
	assert.Equal(t, "Error: plan failed: something broke", failed.DriftTerraformPlan)
	assert.Equal(t, uint(0), failed.DriftToCreate)
	assert.Equal(t, uint(0), failed.DriftToUpdate)
	assert.Equal(t, uint(0), failed.DriftToDelete)
	assert.WithinDuration(t, time.Now(), failed.LatestDriftCheck, time.Minute)

	// a later successful empty plan recovers to "no drift"
	err = ProjectDriftStateMachineApply(failed, "", 0, 0, 0)
	assert.NoError(t, err)
	var recovered models.Project
	models.DB.GormDB.First(&recovered, project.ID)
	assert.Equal(t, models.DriftStatusNoDrift, recovered.DriftStatus)

	// and from a fresh failure, a successful plan WITH changes surfaces "new drift"
	// (this is why the failure clears the old counts: wasEmptyPlan must be true)
	err = ProjectDriftCheckFailed(recovered, "Error: transient provider error")
	assert.NoError(t, err)
	var failedAgain models.Project
	models.DB.GormDB.First(&failedAgain, project.ID)
	err = ProjectDriftStateMachineApply(failedAgain, "plan with changes", 2, 1, 0)
	assert.NoError(t, err)
	var drifted models.Project
	models.DB.GormDB.First(&drifted, project.ID)
	assert.Equal(t, models.DriftStatusNewDrift, drifted.DriftStatus)
}
