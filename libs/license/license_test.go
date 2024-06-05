package license

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLicenseKeyGeneration(t *testing.T) {
	err := LicenseKeyChecker{}.Check()
	assert.NoError(t, err)
}
