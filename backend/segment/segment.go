package segment

import (
	"log/slog"
	"os"

	"github.com/diggerhq/digger/backend/models"
	"github.com/segmentio/analytics-go/v3"
)

var client analytics.Client = nil

func getClient() analytics.Client {
	segmentApiKey := os.Getenv("SEGMENT_API_KEY")
	if segmentApiKey == "" {
		slog.Debug("Not initializing segment because SEGMENT_API_KEY is missing")
		return nil
	}
	if client == nil {
		client = analytics.New(segmentApiKey)
	}
	return client
}

func CloseClient() {
	if client == nil {
		return
	}
	client.Close()
}

func IdentifyClient(userId string, userFullName string, username string, email string, organisationName string, organisationId string, userPlan string) {
	getClient()
	if client == nil {
		return
	}
	slog.Debug("Identifying client", "userId", userId)
	client.Enqueue(analytics.Identify{
		UserId: userId,
		Traits: analytics.NewTraits().
			SetName(userFullName).
			SetUsername(username).
			SetEmail(email).
			Set("organisationName", organisationName).
			Set("organisationId", organisationId).
			Set("plan", userPlan),
	})
}

func Track(orgnaisation models.Organisation, vcsOwner string, vcsUser string, vcsType string, action string, extraProps map[string]string) {
	getClient()
	if client == nil {
		return
	}
	externalOrgId := orgnaisation.ExternalId
	var adminEmail string
	if orgnaisation.AdminEmail != nil {
		adminEmail = *orgnaisation.AdminEmail
	} else {
		adminEmail = "UNKNOWN"
	}

	props := analytics.NewProperties().
		Set("org_id", externalOrgId).
		Set("vcs_user", vcsUser).
		Set("vcs_owner", vcsOwner).
		Set("vcs_type", vcsType)

	if extraProps != nil {
		for k, v := range extraProps {
			props.Set(k, v)
		}
	}
	slog.Debug("Tracking client action", "userId", adminEmail, "action", action)
	client.Enqueue(analytics.Track{
		Event:      action,
		UserId:     adminEmail,
		Properties: props,
	})
}
