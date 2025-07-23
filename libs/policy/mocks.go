package policy

import "github.com/diggerhq/digger/libs/ci"

type MockPolicyChecker struct{}

func (t MockPolicyChecker) CheckAccessPolicy(ciService ci.OrgService, prService *ci.PullRequestService, scmOrganisation, scmrepository, projectName, projectDir, command string, prNumber *int, requestedBy string, planPolicyViolations []string) (bool, error) {
	return false, nil
}

func (t MockPolicyChecker) CheckPlanPolicy(scmRepository, scmOrganisation, projectname, projectDir, planOutput string) (bool, []string, error) {
	return false, nil, nil
}

func (t MockPolicyChecker) CheckDriftPolicy(scmOrganisation, scmRepository, projectname string) (bool, error) {
	return true, nil
}
