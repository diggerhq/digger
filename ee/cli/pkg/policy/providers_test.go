package policy

import (
	"testing"
	"time"

	"github.com/diggerhq/digger/libs/policy"
	lib_spec "github.com/diggerhq/digger/libs/spec"
	"github.com/stretchr/testify/assert"
)

func TestGetPolicyProviderPassesSpecGitTimeoutToManagementRepo(t *testing.T) {
	t.Setenv("DIGGER_MANAGEMENT_REPO", "https://github.com/example/policies")
	t.Setenv("GITHUB_TOKEN", "test-token")

	checker, err := AdvancedPolicyProvider{}.GetPolicyProvider(lib_spec.PolicySpec{PolicyType: "http", GitTimeout: 300}, "", "", "", "noop")
	assert.NoError(t, err)
	provider := checker.(policy.DiggerPolicyChecker).PolicyProvider.(DiggerRepoPolicyProvider)
	assert.Equal(t, "https://github.com/example/policies", provider.ManagementRepoUrl)
	assert.Equal(t, 300*time.Second, provider.gitTimeout())

	// a spec without git_timeout (older backend) keeps the 30 second default
	checker, err = AdvancedPolicyProvider{}.GetPolicyProvider(lib_spec.PolicySpec{PolicyType: "http"}, "", "", "", "noop")
	assert.NoError(t, err)
	assert.Equal(t, 30*time.Second, checker.(policy.DiggerPolicyChecker).PolicyProvider.(DiggerRepoPolicyProvider).gitTimeout())
}
