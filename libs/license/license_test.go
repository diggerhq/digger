// use this to ignore tests from external contributions
//go:build !external

package license

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLicenseKeyChecker(t *testing.T) {
	err := LicenseKeyChecker{}.Check()
	assert.NoError(t, err)
}
