package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/ee/drift/dbmodels"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func sendTestSlackWebhook(webhookURL string) error {
	payload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": "*Project*"},
					{"type": "mrkdwn", "text": "*Status*"},
				},
			},
			{"type": "divider"},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": "<https://driftapp.digger.dev|Dev environment>"},
					{"type": "mrkdwn", "text": ":large_yellow_circle: Drift detected"},
				},
			},
			{"type": "divider"},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": "<https://driftapp.digger.dev|Staging environment>"},
					{"type": "mrkdwn", "text": ":white_circle: Acknowledged drift"},
				},
			},
			{"type": "divider"},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": "<https://driftapp.digger.dev|Prod environment>"},
					{"type": "mrkdwn", "text": ":large_green_circle: No drift"},
				},
			},
			{"type": "divider"},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("error sending POST request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non-OK HTTP status: %s", resp.Status)
	}

	return nil
}

type TestSlackNotificationForOrgRequest struct {
	OrgId string `json:"org_id"`
}

func (mc MainController) SendTestSlackNotificationForOrg(c *gin.Context) {
	var request TestSlackNotificationForOrgRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}
	orgId := request.OrgId

	os := dbmodels.DB.Query.OrgSetting
	orgSettings, err := dbmodels.DB.Query.OrgSetting.Where(os.OrgID.Eq(orgId)).First()

	slackNotificationUrl := orgSettings.SlackNotificationURL

	err = sendTestSlackWebhook(slackNotificationUrl)
	if err != nil {
		log.Printf("Error sending slack notification: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error sending slack notification"})
		return
	}

	c.String(200, "ok")
}
