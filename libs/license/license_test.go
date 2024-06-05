package license

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLicenseKeyChecker(t *testing.T) {
	err := LicenseKeyChecker{}.Check()
	assert.NoError(t, err)
}
