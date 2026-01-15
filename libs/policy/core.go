package policy

type Provider interface {
	GetAccessPolicy(organisation string, repository string, projectname string, projectDir string) (string, error)
	GetPlanPolicy(organisation string, repository string, projectname string, projectDir string) (string, error)
	GetDriftPolicy() (string, error)
	GetApplyPolicy(organisation string, repository string, projectname string, projectDir string) (string, error)
	GetOrganisation() string // TODO: remove this method from here since out of place
}

type Checker interface {
	// TODO refactor arguments - use AccessPolicyContext
	CheckAccessPolicy(SCMOrganisation string, SCMrepository string, projectName string, projectDir string, command string, prNumber *int, requestedBy string, teams []string, approvals []string, approvalTeams []string, planPolicyViolations []string) (bool, error)
	CheckPlanPolicy(SCMrepository string, SCMOrganisation string, projectname string, projectDir string, requestedBy string, teams []string, approvals []string, approvalTeams []string, planOutput string) (bool, []string, error)
	CheckDriftPolicy(SCMOrganisation string, SCMrepository string, projectname string) (bool, error)
	CheckApplyPolicy(SCMOrganisation string, SCMrepository string, projectName string, projectDir string, command string, prNumber *int, requestedBy string, teams []string, approvals []string, approvalTeams []string, planPolicyViolations []string, planOutput string) (bool, []string, error)
}

type PolicyCheckerProvider interface {
	Get(hostname string, organisationName string, authToken string) (Checker, error)
}

type AccessPolicyContext struct {
	SCMOrganisation  string
	SCMrepository    string
	projectName      string
	command          string
	prNumber         *int
	requestedBy      string
	policyViolations []string
}
