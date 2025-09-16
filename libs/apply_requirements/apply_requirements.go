package apply_requirements

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/digger_config"
	"log/slog"
)

func CheckApplyRequirements(ghService ci.PullRequestService, impactedProjects []digger_config.Project, prNumber int, sourceBranch string, targetBranch string) error {
	isMergeable, err := ghService.IsMergeable(prNumber)
	if err != nil {
		slog.Error("Error checking if PR is mergeable", "issueNumber", prNumber)
		return fmt.Errorf("error checking if PR is mergeable")
	}
	approvals, err := ghService.GetApprovals(prNumber)
	if err != nil {
		slog.Error("Error getting approvals", "issueNumber", prNumber)
		return fmt.Errorf("error getting approvals")
	}
	isApproved := len(approvals) > 0
	isDiverged, err := ghService.IsDivergedFromBranch(sourceBranch, targetBranch)
	if err != nil {
		slog.Error("Error checking if PR is diverged", "issueNumber", prNumber)
		return fmt.Errorf("error checking if PR is diverged")
	}
	for _, proj := range impactedProjects {
		for _, req := range proj.ApplyRequirements {
			if req == digger_config.ApplyRequirementsApproved && isApproved == false {
				return fmt.Errorf("PR fails apply requirements for project %v, Expected PR to be approved, a minimum of one approval is required before proceeding", proj.Name)
			} else if req == digger_config.ApplyRequirementsUndiverged && isDiverged == true {
				return fmt.Errorf("PR fails apply requirements for project %v, Expected PR to be undiverged from target branch. Merge main into the PR branch or rebase the PR branch on top of main", proj.Name)
			} else if req == digger_config.ApplyRequirementsMergeable && isMergeable == false {
				return fmt.Errorf("PR fails apply requirements for project %v, Expected PR to be mergable. Ensure all status checks are successful in order to proceed", proj.Name)
			} else {
				slog.Warn("unknown apply requirements found", "project", proj.Name, "requirement", req)
			}
		}
	}
	return nil
}
