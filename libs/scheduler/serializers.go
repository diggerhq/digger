package scheduler

import (
	"encoding/json"
	"log"
)

func GetJobSpecs(jobs []SerializedJob) ([]JobJson, error) {
	jobSpecs := make([]JobJson, 0)
	for _, job := range jobs {
		var jobSpec JobJson
		err := json.Unmarshal(job.JobString, &jobSpec)
		if err != nil {
			log.Printf("Failed to convert unmarshall Serialized job")
			return nil, err
		}
		jobSpecs = append(jobSpecs, jobSpec)
	}
	return jobSpecs, nil
}

func JobsToProjectMap(jobs []SerializedJob) (map[string]SerializedJob, error) {
	res := make(map[string]SerializedJob)
	for _, job := range jobs {
		res[job.ProjectName] = job
	}
	return res, nil
}
