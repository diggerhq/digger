package drift

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestTeamsNotificationProviderInstantiation(t *testing.T) {
	os.Setenv("INPUT_DRIFT_DETECTION_TEAMS_NOTIFICATION_URL", "https://outlook.office.com/webhook/test")
	defer os.Unsetenv("INPUT_DRIFT_DETECTION_TEAMS_NOTIFICATION_URL")

	provider := DriftNotificationProviderBasic{}
	notification, err := provider.Get(nil)
	assert.NoError(t, err)
	_, ok := notification.(*TeamsNotification)
	assert.True(t, ok)
}
