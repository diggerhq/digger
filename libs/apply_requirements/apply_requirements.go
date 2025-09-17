package apply_requirements

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"log/slog"
)

// IgnoreMergeabilityForProject will strip out the 'mergeability' requirement if
// the project's workflow has specified skip_merge_check: true
func IgnoreMergeabilityForProject(project digger_config.Project, jobs []scheduler.Job) bool {
	job, err := scheduler.JobForProjectName(jobs, project.Name)
	if err != nil {
		slog.Warn("could not find job for mergeability ignore check, skipping this check and returning false")
		return false
	}
	return job.SkipMergeCheck
}
func CheckApplyRequirements(ghService ci.PullRequestService, impactedProjects []digger_config.Project, jobs []scheduler.Job, prNumber int, sourceBranch string, targetBranch string) error {
	isMergeable, err := ghService.IsMergeable(prNumber)
	if err != nil {
		slog.Error("Error checking if PR is mergeable", "prNumber", prNumber)
		return fmt.Errorf("error checking if PR is mergeable")
	}
	approvals, err := ghService.GetApprovals(prNumber)
	if err != nil {
		slog.Error("Error getting approvals", "prNumber", prNumber)
		return fmt.Errorf("error getting approvals")
	}
	isApproved := len(approvals) > 0
	isDiverged, err := ghService.IsDivergedFromBranch(sourceBranch, targetBranch)
	if err != nil {
		slog.Error("Error checking if PR is diverged", "prNumber", prNumber)
		return fmt.Errorf("error checking if PR is diverged")
	}
	for _, proj := range impactedProjects {
		for _, req := range proj.ApplyRequirements {
			ignoreMergeability := IgnoreMergeabilityForProject(proj, jobs)
			if req == digger_config.ApplyRequirementsApproved && isApproved == false {
				return fmt.Errorf("PR fails apply requirements for project %v, Expected PR to be approved, a minimum of one approval is required before proceeding", proj.Name)
			} else if req == digger_config.ApplyRequirementsUndiverged && isDiverged == true {
				return fmt.Errorf("PR fails apply requirements for project %v, Expected PR to be undiverged from target branch. Merge main into the PR branch or rebase the PR branch on top of main", proj.Name)
			} else if req == digger_config.ApplyRequirementsMergeable && isMergeable == false && !ignoreMergeability {
				return fmt.Errorf("PR fails apply requirements for project %v, Expected PR to be mergable. Ensure all status checks are successful in order to proceed", proj.Name)
			} else {
				slog.Warn("unknown apply requirements found", "project", proj.Name, "requirement", req)
			}
		}
	}
	return nil
}
