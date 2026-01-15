package policy

type MockPolicyChecker struct {
}

func (t MockPolicyChecker) CheckAccessPolicy(SCMOrganisation string, SCMrepository string, projectName string, projectDir string, command string, prNumber *int, requestedBy string, teams []string, approvals []string, approvalTeams []string, planPolicyViolations []string) (bool, error) {
	return false, nil
}

func (t MockPolicyChecker) CheckPlanPolicy(SCMrepository string, SCMOrganisation string, projectname string, projectDir string, requestedBy string, teams []string, approvals []string, approvalTeams []string, planOutput string) (bool, []string, error) {
	return false, nil, nil
}

func (t MockPolicyChecker) CheckDriftPolicy(SCMOrganisation string, SCMrepository string, projectname string) (bool, error) {
	return true, nil
}

func (t MockPolicyChecker) CheckApplyPolicy(SCMOrganisation string, SCMrepository string, projectName string, projectDir string, command string, prNumber *int, requestedBy string, teams []string, approvals []string, approvalTeams []string, planPolicyViolations []string, planOutput string) (bool, []string, error) {
	return true, nil, nil
}
