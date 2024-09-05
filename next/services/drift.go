package services

import (
	"encoding/json"
	"fmt"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
	"log"
	"strings"
)

func SaveUpdatedDriftStatus(batch model.DiggerBatch, job model.DiggerJob, terraformOutput string) error {
	orgId := batch.OrganizationID
	var jobSpec orchestrator_scheduler.Job
	err := json.Unmarshal(job.JobSpec, &jobSpec)
	if err != nil {
		log.Printf("error whilst unmarshalling jobspec ")
		return fmt.Errorf("error whilst unmarshalling jobs")
	}

	diggerRepoName := strings.ReplaceAll(batch.RepoFullName, "/", "-")
	repo, err := dbmodels.DB.GetRepo(orgId, diggerRepoName)
	if err != nil {
		log.Printf("error whilst retrieving job repo %v", err)
		return fmt.Errorf("error whilst retrieving job repo %v", err)

	}

	project, err := dbmodels.DB.GetProjectByName(orgId, repo, jobSpec.ProjectName)
	if err != nil {
		log.Printf("error whilst retrieving project %v", err)
		return fmt.Errorf("error whilst retrieving job project %v", err)
	}

	// empty plan output
	if strings.Contains(terraformOutput, "No changes. Your infrastructure matches the configuration.") {
		terraformOutput = ""
	}

	project.DriftLatestTerraformOutput = terraformOutput
	err = dbmodels.DB.GormDB.Save(project).Error
	if err != nil {
		log.Printf("failed to save project: error %v", err)
		return fmt.Errorf("failed to save project: error %v", err)
	}
	return nil
}
