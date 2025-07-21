package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"gorm.io/gorm"
)

func (db *Database) GetDiggerCiJob(diggerJobId string) (*DiggerJob, error) {
	log.Printf("GetDiggerCiJob, diggerJobId: %v", diggerJobId)
	var ciJob DiggerJob

	err := db.GormDB.Preload("Batch").Where("digger_job_id = ?", diggerJobId).First(&ciJob).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("ci job not found")
		}
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, err
	}

	return &ciJob, nil
}

func (db *Database) CreateCiJobFromSpec(spec spec.Spec, runName, projectName, batchId string) (*DiggerJob, error) {
	marshalledJobSpec, err := json.Marshal(spec.Job)
	if err != nil {
		log.Printf("failed to marshal job: %v", err)
		return nil, err
	}

	marshalledReporterSpec, err := json.Marshal(spec.Reporter)
	if err != nil {
		log.Printf("failed to marshal reporter: %v", err)
		return nil, err
	}

	marshalledCommentUpdaterSpec, err := json.Marshal(spec.CommentUpdater)
	if err != nil {
		log.Printf("failed to marshal comment updater: %v", err)
		return nil, err
	}

	marshalledLockSpec, err := json.Marshal(spec.Lock)
	if err != nil {
		log.Printf("failed to marshal lockspec: %v", err)
		return nil, err
	}

	marshalledBackendSpec, err := json.Marshal(spec.Backend)
	if err != nil {
		log.Printf("failed to marshal backend spec: %v", err)
		return nil, err

	}

	marshalledVcsSpec, err := json.Marshal(spec.VCS)
	if err != nil {
		log.Printf("failed to marshal vcs spec: %v", err)
		return nil, err
	}

	marshalledPolicySpec, err := json.Marshal(spec.Policy)
	if err != nil {
		log.Printf("failed to marshal policy spec: %v", err)
		return nil, err
	}

	marshalledVariablesSpec, err := json.Marshal(spec.Variables)
	if err != nil {
		log.Printf("failed to marshal variables spec: %v", err)
		return nil, err
	}

	workflowRunUrl := ""
	summary := DiggerJobSummary{
		ResourcesCreated: 0,
		ResourcesUpdated: 0,
		ResourcesDeleted: 0,
	}
	err = db.GormDB.Create(&summary).Error
	if err != nil {
		log.Printf("failed to create job summary: %v", err)
		return nil, err
	}

	job := &DiggerJob{
		BatchID:                      &batchId,
		DiggerJobID:                  spec.JobId,
		RunName:                      runName,
		SerializedJobSpec:            marshalledJobSpec,
		SerializedReporterSpec:       marshalledReporterSpec,
		SerializedCommentUpdaterSpec: marshalledCommentUpdaterSpec,
		SerializedLockSpec:           marshalledLockSpec,
		SerializedBackendSpec:        marshalledBackendSpec,
		SerializedVcsSpec:            marshalledVcsSpec,
		SerializedPolicySpec:         marshalledPolicySpec,
		SerializedVariablesSpec:      marshalledVariablesSpec,
		WorkflowFile:                 spec.VCS.WorkflowFile,
		WorkflowRunUrl:               &workflowRunUrl,
		Status:                       orchestrator_scheduler.DiggerJobCreated,
		DiggerJobSummary:             summary,
		ProjectName:                  projectName,
	}

	err = db.GormDB.Create(job).Error
	if err != nil {
		log.Printf("failed to create job: %v", err)
		return nil, err
	}

	return job, nil
}
