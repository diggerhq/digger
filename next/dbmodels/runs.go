package dbmodels

type DiggerRunStatus string

const (
	RunQueued          DiggerRunStatus = "Queued"
	RunPendingPlan     DiggerRunStatus = "Pending Plan"
	RunPlanning        DiggerRunStatus = "Running Plan"
	RunPendingApproval DiggerRunStatus = "Pending Approval"
	RunApproved        DiggerRunStatus = "Approved"
	RunPendingApply    DiggerRunStatus = "Pending Apply"
	RunApplying        DiggerRunStatus = "Running Apply"
	RunSucceeded       DiggerRunStatus = "Succeeded"
	RunFailed          DiggerRunStatus = "Failed"
)

type RunType string

const (
	PlanAndApply RunType = "Plan and Apply"
	PlanOnly     RunType = "Plan Only"
)
