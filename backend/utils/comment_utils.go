package utils

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/ci"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"encoding/json"
)

// UpdatePRCommentRealtime updates the GitHub PR comment with current job statuses
func UpdatePRCommentRealtime(gh GithubClientProvider, batch *models.DiggerBatch) error {
	slog.Debug("Updating PR comment with real-time job statuses", "batchId", batch.ID, "prNumber", batch.PrNumber)

	// Get PR service for this batch
	prService, err := GetPrServiceFromBatch(batch, gh)
	if err != nil {
		slog.Error("Error getting PR service for real-time comment update", "batchId", batch.ID, "error", err)
		return fmt.Errorf("error getting PR service: %v", err)
	}

	// Get all jobs for this batch (initial check)
	jobs, err := models.DB.GetDiggerJobsForBatch(batch.ID)
	if err != nil {
		slog.Error("Error getting jobs for batch", "batchId", batch.ID, "error", err)
		return fmt.Errorf("error getting jobs for batch: %v", err)
	}

	if len(jobs) == 0 {
		slog.Debug("No jobs found for batch", "batchId", batch.ID)
		return nil
	}

	// Requery database immediately before generating comment to get latest job statuses and batch data
	// This minimizes race conditions where job statuses or batch data might change between queries
	slog.Debug("Requerying jobs and batch for latest status before comment generation", "batchId", batch.ID)

	// Get fresh batch data
	freshBatch, err := models.DB.GetDiggerBatch(&batch.ID)
	if err != nil {
		slog.Error("Error requerying batch", "batchId", batch.ID, "error", err)
		return fmt.Errorf("error requerying batch: %v", err)
	}

	// Get fresh job data
	freshJobs, err := models.DB.GetDiggerJobsForBatch(batch.ID)
	if err != nil {
		slog.Error("Error requerying jobs for batch", "batchId", freshBatch.ID, "error", err)
		return fmt.Errorf("error requerying jobs for batch: %v", err)
	}

	if len(freshJobs) == 0 {
		slog.Debug("No jobs found after requery", "batchId", freshBatch.ID)
		return nil
	}

	// Generate comment message with fresh job data
	message, err := GenerateRealtimeCommentMessage(freshJobs, freshBatch.BatchType)
	if err != nil {
		slog.Error("Error generating real-time comment message", "batchId", freshBatch.ID, "error", err)
		return fmt.Errorf("error generating comment message: %v", err)
	}

	// Update or create the summary comment using fresh batch data
	commentId, err := UpdateOrCreateSummaryComment(prService, freshBatch, message)
	if err != nil {
		slog.Error("Error updating real-time summary comment", "batchId", freshBatch.ID, "error", err)
		return fmt.Errorf("error updating summary comment: %v", err)
	}

	// Update batch with comment ID if it was newly created (using fresh batch data)
	if freshBatch.CommentId == nil && commentId != nil {
		freshBatch.CommentId = commentId
		err = models.DB.GormDB.Save(&freshBatch).Error
		if err != nil {
			slog.Error("Error saving comment ID to batch", "batchId", freshBatch.ID, "commentId", commentId, "error", err)
			return fmt.Errorf("error saving comment ID to batch: %v", err)
		}
	}

	slog.Debug("Successfully updated real-time PR comment", "batchId", freshBatch.ID, "prNumber", freshBatch.PrNumber, "commentId", commentId)
	return nil
}

// UpdatePRComment updates the PR comment for a job status change
func UpdatePRComment(gh GithubClientProvider, jobId string, job *models.DiggerJob, status string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Recovered from panic in UpdatePRComment goroutine",
				"jobId", jobId,
				"status", status,
				"error", r,
				"stack", string(debug.Stack()),
			)
		}
	}()

	err := UpdatePRCommentRealtime(gh, job.Batch)
	if err != nil {
		slog.Warn("Failed to update PR comment for job",
			"jobId", jobId,
			"batchId", job.Batch.ID,
			"status", status,
			"error", err,
		)
	}
}

// GenerateRealtimeCommentMessage creates the markdown table for real-time PR comments
// This matches the exact format used by the CLI's BasicCommentUpdater
func GenerateRealtimeCommentMessage(jobs []models.DiggerJob, batchType orchestrator_scheduler.DiggerCommand) (string, error) {
	if len(jobs) == 0 {
		return "", fmt.Errorf("no jobs provided")
	}

	jobTypeTitle := cases.Title(language.AmericanEnglish).String(string(batchType))

	// Match exact CLI format - no header, just the table
	message := ""
	message += fmt.Sprintf("| Project | Status | %s | + | ~ | - |\n", jobTypeTitle)
	message += fmt.Sprintf("|---------|--------|------|---|---|---|\n")

	for _, job := range jobs {
		prCommentUrl := job.PRCommentUrl
		if prCommentUrl == "" {
			prCommentUrl = "#"
		}

		// Safe handling of WorkflowRunUrl pointer
		workflowUrl := "#"
		if job.WorkflowRunUrl != nil {
			workflowUrl = *job.WorkflowRunUrl
		}

		// Try to get project alias with proper error handling and fallbacks
		projectAlias := "Unknown" // Final fallback

		//  Try using MapToJsonStruct for proper alias handling
		serializedJob, err := job.MapToJsonStruct()
		if err == nil {
			// Success - use the scheduler function for alias
			projectAlias = orchestrator_scheduler.GetProjectAlias(serializedJob)
			slog.Debug("Successfully got project alias from serialized job",
				"jobId", job.DiggerJobID,
				"projectName", serializedJob.ProjectName,
				"projectAlias", projectAlias)
		} else {
			//Fallback - construct minimal SerializedJob manually
			slog.Warn("Failed to convert job to serialized struct, falling back to manual construction",
				"jobId", job.DiggerJobID,
				"error", err)

			if job.SerializedJobSpec != nil {
				var jobSpec orchestrator_scheduler.JobJson
				unmarshalErr := json.Unmarshal(job.SerializedJobSpec, &jobSpec)
				if unmarshalErr == nil {
					// Use the same logic as orchestrator_scheduler.GetProjectAlias
					projectAlias = orchestrator_scheduler.GetProjectAlias(orchestrator_scheduler.SerializedJob{
						ProjectAlias: jobSpec.ProjectAlias,
						ProjectName:  jobSpec.ProjectName,
					})

					// Add your empty string protection
					if projectAlias == "" {
						projectAlias = "Unknown"
					}

					slog.Debug("Got project alias using GetProjectAlias",
						"jobId", job.DiggerJobID,
						"projectName", jobSpec.ProjectName,
						"projectAlias", projectAlias)
				} else {
					if job.ProjectName != "" {
						projectAlias = job.ProjectName
						slog.Debug("Using job.ProjectName as final fallback",
							"jobId", job.DiggerJobID,
							"projectName", job.ProjectName)
					} else {
						slog.Warn("Failed to get any project information, using 'Unknown'",
							"jobId", job.DiggerJobID,
							"marshalError", err,
							"unmarshalError", unmarshalErr)
					}
				}
			} else {
				slog.Warn("SerializedJobSpec is nil, cannot determine project name",
					"jobId", job.DiggerJobID)
			}
		}

		// Default resource counts to 0 if DiggerJobSummary is nil
		resourcesCreated := uint(0)
		resourcesUpdated := uint(0)
		resourcesDeleted := uint(0)

		// Only access DiggerJobSummary fields if it's not nil
		if job.DiggerJobSummary.ID != 0 {
			resourcesCreated = job.DiggerJobSummary.ResourcesCreated
			resourcesUpdated = job.DiggerJobSummary.ResourcesUpdated
			resourcesDeleted = job.DiggerJobSummary.ResourcesDeleted
		}

		// Match exact CLI format: |emoji **project** |<a href='workflow'>status</a> | <a href='comment'>jobType</a> | + | ~ | - |
		message += fmt.Sprintf("|%s **%s** |<a href='%s'>%s</a> | <a href='%s'>%s</a> | %d | %d | %d|\n",
			job.Status.ToEmoji(),
			projectAlias,
			workflowUrl,
			job.Status.ToString(),
			prCommentUrl,
			jobTypeTitle,
			resourcesCreated,
			resourcesUpdated,
			resourcesDeleted)
	}

	// Add instruction helpers (same as CLI)
	message += "\n" + formatExampleCommands()

	// Handle comment length limits
	const GithubCommentMaxLength = 65536
	if len(message) > GithubCommentMaxLength {
		slog.Warn("Comment message too long, trimming", "originalLength", len(message), "maxLength", GithubCommentMaxLength)
		const footer = "\n\n[Message truncated due to length limits]"
		trimLength := len(message) - GithubCommentMaxLength + len(footer)
		message = message[:len(message)-trimLength] + footer
		slog.Debug("Trimmed comment message", "newLength", len(message))
	}

	return message, nil
}

// formatExampleCommands creates a collapsible markdown section with example commands
// This matches the exact format used by the CLI's BasicCommentUpdater
func formatExampleCommands() string {
	return `
<details>
  <summary>Instructions</summary>

⏩ To apply these changes, run the following command:

` + "```" + `bash
digger apply
` + "```" + `

🚮 To unlock the projects in this PR run the following command:
` + "```" + `bash
digger unlock
` + "```" + `
</details>
`
}

// UpdateOrCreateSummaryComment updates or creates the summary comment for the batch
func UpdateOrCreateSummaryComment(prService ci.PullRequestService, batch *models.DiggerBatch, message string) (*int64, error) {
	if batch.CommentId != nil {
		// Update existing comment
		commentIdStr := strconv.FormatInt(*batch.CommentId, 10)
		err := prService.EditComment(batch.PrNumber, commentIdStr, message)
		if err != nil {
			slog.Warn("Failed to update existing comment, will create new one", "commentId", *batch.CommentId, "prNumber", batch.PrNumber, "error", err)
			// Fall through to create new comment
		} else {
			slog.Debug("Successfully updated existing comment", "commentId", *batch.CommentId, "prNumber", batch.PrNumber)
			return batch.CommentId, nil
		}
	}

	// Create new comment
	comment, err := prService.PublishComment(batch.PrNumber, message)
	if err != nil {
		slog.Error("Failed to create new comment", "prNumber", batch.PrNumber, "error", err)
		return nil, fmt.Errorf("failed to create comment: %v", err)
	}

	commentId, err := strconv.ParseInt(comment.Id, 10, 64)
	if err != nil {
		slog.Error("Failed to parse comment ID", "commentIdStr", comment.Id, "error", err)
		return nil, fmt.Errorf("failed to parse comment ID: %v", err)
	}

	slog.Debug("Successfully created new comment", "commentId", commentId, "prNumber", batch.PrNumber)
	return &commentId, nil
}

// GetPrServiceFromBatch gets the appropriate PR service for a batch
func GetPrServiceFromBatch(batch *models.DiggerBatch, gh GithubClientProvider) (ci.PullRequestService, error) {
	slog.Debug("Getting PR service for batch",
		"batchId", batch.ID,
		"vcs", batch.VCS,
		"prNumber", batch.PrNumber,
	)

	switch batch.VCS {
	case "github":
		slog.Debug("Using GitHub service for batch",
			"batchId", batch.ID,
			"installationId", batch.GithubInstallationId,
			"repoFullName", batch.RepoFullName,
		)

		service, _, err := GetGithubService(
			gh,
			batch.GithubInstallationId,
			batch.RepoFullName,
			batch.RepoOwner,
			batch.RepoName,
		)

		if err != nil {
			slog.Error("Error getting GitHub service",
				"batchId", batch.ID,
				"repoFullName", batch.RepoFullName,
				"error", err,
			)
		} else {
			slog.Debug("Successfully got GitHub service",
				"batchId", batch.ID,
				"repoFullName", batch.RepoFullName,
			)
		}

		return service, err

	case "gitlab":
		slog.Debug("Using GitLab service for batch",
			"batchId", batch.ID,
			"projectId", batch.GitlabProjectId,
			"repoFullName", batch.RepoFullName,
		)

		service, err := GetGitlabService(
			GitlabClientProvider{},
			batch.GitlabProjectId,
			batch.RepoName,
			batch.RepoFullName,
			batch.PrNumber,
			"",
		)

		if err != nil {
			slog.Error("Error getting GitLab service",
				"batchId", batch.ID,
				"projectId", batch.GitlabProjectId,
				"error", err,
			)
		} else {
			slog.Debug("Successfully got GitLab service",
				"batchId", batch.ID,
				"projectId", batch.GitlabProjectId,
			)
		}

		return service, err
	}

	slog.Error("Unsupported VCS type", "vcs", batch.VCS, "batchId", batch.ID)
	return nil, fmt.Errorf("could not retrieve a service for %v", batch.VCS)
}
