package dbmodels

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/diggerhq/digger/ee/drift/model"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log"
	"time"
)

type DiggerJobStatus string

const (
	DiggerJobCreated      DiggerJobStatus = "created"
	DiggerJobTriggered    DiggerJobStatus = "triggered"
	DiggerJobFailed       DiggerJobStatus = "failed"
	DiggerJobStarted      DiggerJobStatus = "started"
	DiggerJobSucceeded    DiggerJobStatus = "succeeded"
	DiggerJobQueuedForRun DiggerJobStatus = "queued"
)

func (db *Database) GetDiggerCiJob(diggerJobId string) (*model.DiggerCiJob, error) {
	log.Printf("GetDiggerCiJob, diggerJobId: %v", diggerJobId)
	var ciJob model.DiggerCiJob

	err := db.GormDB.Where("digger_job_id = ?", diggerJobId).First(&ciJob).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("ci job not found")
		}
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, err
	}

	return &ciJob, nil
}

func (db *Database) UpdateDiggerJob(job *model.DiggerCiJob) error {
	result := db.GormDB.Save(job)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("DiggerJob %v, (id: %v) has been updated successfully\n", job.DiggerJobID, job.ID)
	return nil
}

func (db *Database) CreateCiJobFromSpec(spec spec.Spec, runName string, projectId string) (*model.DiggerCiJob, error) {

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

	job := &model.DiggerCiJob{
		ID:                 uuid.NewString(),
		DiggerJobID:        spec.JobId,
		Spectype:           string(spec.SpecType),
		Commentid:          spec.CommentId,
		Runname:            runName,
		Jobspec:            marshalledJobSpec,
		Reporterspec:       marshalledReporterSpec,
		Commentupdaterspec: marshalledCommentUpdaterSpec,
		Lockspec:           marshalledLockSpec,
		Backendspec:        marshalledBackendSpec,
		Vcsspec:            marshalledVcsSpec,
		Policyspec:         marshalledPolicySpec,
		Variablesspec:      marshalledVariablesSpec,
		CreatedAt:          time.Time{},
		UpdatedAt:          time.Time{},
		DeletedAt:          gorm.DeletedAt{},
		WorkflowFile:       spec.VCS.WorkflowFile,
		WorkflowURL:        "",
		Status:             string(DiggerJobCreated),
		ResourcesCreated:   0,
		ResourcesUpdated:   0,
		ResourcesDeleted:   0,
		ProjectID:          projectId,
	}

	err = db.GormDB.Create(job).Error
	if err != nil {
		log.Printf("failed to create job: %v", err)
		return nil, err
	}

	return job, nil
}
