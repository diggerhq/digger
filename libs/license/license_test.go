// use this to ignore tests from external contributions
//go:build !external

package license

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestLicenseKeyChecker(t *testing.T) {
	if os.Getenv("IS_EXTERNAL_PR") == "true" {
		t.Skip("External PRs are not allowed to run license tests")
	}
	err := LicenseKeyChecker{}.Check()
	assert.NoError(t, err)
}
