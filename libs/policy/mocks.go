package policy

import "github.com/diggerhq/digger/libs/ci"

type MockPolicyChecker struct {
}

func (t MockPolicyChecker) CheckAccessPolicy(ciService ci.OrgService, prService *ci.PullRequestService, SCMOrganisation string, SCMrepository string, projectName string, projectDir string, command string, prNumber *int, requestedBy string, planPolicyViolations []string) (bool, error) {
	return false, nil
}

func (t MockPolicyChecker) CheckPlanPolicy(SCMrepository string, SCMOrganisation string, projectname string, projectDir string, planOutput string) (bool, []string, error) {
	return false, nil, nil
}

func (t MockPolicyChecker) CheckDriftPolicy(SCMOrganisation string, SCMrepository string, projectname string) (bool, error) {
	return true, nil
}
