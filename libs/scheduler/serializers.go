package scheduler

import (
	"encoding/json"
	"log/slog"
)

func GetJobSpecs(jobs []SerializedJob) ([]JobJson, error) {
	jobSpecs := make([]JobJson, 0)
	for _, job := range jobs {
		var jobSpec JobJson
		err := json.Unmarshal(job.JobString, &jobSpec)
		if err != nil {
			slog.Error("Failed to unmarshal serialized job",
				"projectName", job.ProjectName,
				"jobId", job.DiggerJobId,
				"error", err)
			return nil, err
		}
		jobSpecs = append(jobSpecs, jobSpec)
	}
	slog.Debug("Successfully unmarshaled job specs", "count", len(jobSpecs))
	return jobSpecs, nil
}

func JobsToProjectMap(jobs []SerializedJob) (map[string]SerializedJob, error) {
	res := make(map[string]SerializedJob)
	for _, job := range jobs {
		res[job.ProjectName] = job
		slog.Debug("Added job to project map",
			"projectName", job.ProjectName,
			"jobId", job.DiggerJobId)
	}
	slog.Debug("Created project map from jobs", "projectCount", len(res))
	return res, nil
}
