package comment_updater

import (
	"fmt"
	"log/slog"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type CommentUpdater interface {
	UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error
}

// getStatusEmoji returns a custom emoji for the status text in the Status column
func getStatusEmoji(statusText string) string {
	switch statusText {
	case "succeeded":
		return "🎉" // Success celebration
	case "running":
		return "🚀" // Rocket - Work in progress
	case "failed":
		return "🚫" // Cross - failure
	case "created":
		return "➕" // Plus - newly created
	case "unknown status":
		return "❓" // Question - unknown
	default:
		return "❓" // Default for any unexpected status
	}
}

// formatResourceCount formats resource counts with emojis and bold text for better visibility
func formatResourceCount(count uint, emoji string) string {
	if count > 0 {
		return fmt.Sprintf("%v **%d**", emoji, count)
	}
	return "0"
}

type BasicCommentUpdater struct {
}

func (b BasicCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error {
	jobSpecs, err := scheduler.GetJobSpecs(jobs)
	if err != nil {
		slog.Error("could not get jobspecs", "error", err, "jobCount", len(jobs))
		return err
	}

	if len(jobSpecs) == 0 {
		slog.Warn("no job specs found, cannot update comment", "jobCount", len(jobs))
		return nil
	}

	firstJobSpec := jobSpecs[0]
	jobType := firstJobSpec.JobType
	jobTypeTitle := cases.Title(language.AmericanEnglish).String(string(jobType))

	slog.Info("updating comment with job results",
		"prNumber", prNumber,
		"commentId", prCommentId,
		"jobCount", len(jobs),
		"jobType", jobType)

	message := ""
	message = message + fmt.Sprintf("| Project | Status | %v | ➕ Add | 🔄 Modify | ➖ Delete |\n", jobTypeTitle)
	message = message + fmt.Sprintf("|---------|--------|------|---------|-----------|----------|\n")

	for _, job := range jobs {
		prCommentUrl := job.PRCommentUrl

		// Safe handling of WorkflowRunUrl pointer
		workflowUrl := "#"
		if job.WorkflowRunUrl != nil {
			workflowUrl = *job.WorkflowRunUrl
		}

		message = message + fmt.Sprintf("|%v **%v** |%v <a href='%v'>%v</a> | <a href='%v'>%v</a> | %v | %v | %v|\n",
			job.Status.ToEmoji(),
			scheduler.GetProjectAlias(job),
			getStatusEmoji(job.Status.ToString()),
			workflowUrl,
			cases.Title(language.AmericanEnglish).String(job.Status.ToString()),
			prCommentUrl,
			jobTypeTitle,
			formatResourceCount(job.ResourcesCreated, "➕"),
			formatResourceCount(job.ResourcesUpdated, "🔄"),
			formatResourceCount(job.ResourcesDeleted, "➖"))
	}

	message = message + "\n" + formatExampleCommands()

	const GithubCommentMaxLength = 65536
	if len(message) > GithubCommentMaxLength {
		// TODO: Handle the case where message is too long by trimming
		slog.Warn("message is too long, trimming",
			"originalLength", len(message),
			"maxLength", GithubCommentMaxLength)

		const footer = "[trimmed]"
		trimLength := len(message) - GithubCommentMaxLength + len(footer)
		message = message[:len(message)-trimLength] + footer

		slog.Debug("trimmed message", "newLength", len(message))
	}

	err = prService.EditComment(prNumber, prCommentId, message)
	if err != nil {
		slog.Warn("failed to update summary comment",
			"error", err,
			"prNumber", prNumber,
			"commentId", prCommentId)
	} else {
		slog.Info("successfully updated summary comment",
			"prNumber", prNumber,
			"commentId", prCommentId)
	}

	return nil
}

// formatExampleCommands creates a collapsible markdown section with example commands
func formatExampleCommands() string {
	return `
<details>
  <summary><b>📖 How to use these results</b></summary>

### ⏩ Apply Changes

To apply the planned changes, comment on this PR:

` + "```" + `bash
digger apply
` + "```" + `

### 🔓 Unlock Projects

If you need to unlock the projects in this PR (e.g., after a failed run), use:

` + "```" + `bash
digger unlock
` + "```" + `

### 💡 Tips

- Click on the **Status** links to view detailed workflow runs
- Click on the **Plan/Apply** links to see individual project comments
- The table shows: **+** (resources to add), **~** (resources to modify), **-** (resources to delete)

</details>
`
}

type NoopCommentUpdater struct {
}

func (b NoopCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService ci.PullRequestService, prCommentId string) error {
	slog.Debug("noop comment updater called, no action taken",
		"prNumber", prNumber,
		"commentId", prCommentId,
		"jobCount", len(jobs))
	return nil
}
