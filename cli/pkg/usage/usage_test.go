package usage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// util function for testing of send usage record
func TestSendingUsageRecord(t *testing.T) {
	if os.Getenv("MANUAL_TEST") == "" {
		t.Skip("Skipping manual test")
	}
	err := SendUsageRecord("repoOwner", "testEvent", "testing")
	assert.Nil(t, err)
}
