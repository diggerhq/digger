package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/ee/drift/utils"
	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
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
					{"type": "mrkdwn", "text": fmt.Sprintf("<%v|Dev environment>", os.Getenv("DIGGER_APP_URL"))},
					{"type": "mrkdwn", "text": ":large_yellow_circle: Drift detected"},
				},
			},
			{"type": "divider"},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": fmt.Sprintf("<%v|Staging environment>", os.Getenv("DIGGER_APP_URL"))},
					{"type": "mrkdwn", "text": ":white_circle: Acknowledged drift"},
				},
			},
			{"type": "divider"},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": fmt.Sprintf("<%v|Prod environment>", os.Getenv("DIGGER_APP_URL"))},
					{"type": "mrkdwn", "text": ":large_green_circle: No drift"},
				},
			},
			{"type": "divider"},
			{
				"type": "section",
				"fields": []map[string]string{
					{"type": "mrkdwn", "text": ":arrow_right: *Note: This is a test notification*pwd"},
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

func sendTestTeamsWebhook(webhookURL string) error {
	messageCard := map[string]interface{}{
		"@type":    "MessageCard",
		"@context": "http://schema.org/extensions",
		"themeColor": "0076D7",
		"summary":  "Digger Drift Detection Test",
		"sections": []map[string]interface{}{
			{
				"activityTitle":    "Digger Drift Detection",
				"activitySubtitle": "Test Notification",
				"activityText":     "This is a test notification to verify your MS Teams integration is working correctly.",
				"facts": []map[string]string{
					{"name": "Project", "value": "Dev environment"},
					{"name": "Status", "value": "🟡 Drift detected"},
				},
			},
			{
				"activityTitle":    "Environment Status",
				"activitySubtitle": "Current Status Overview",
				"facts": []map[string]string{
					{"name": "Dev environment", "value": "🟡 Drift detected"},
					{"name": "Staging environment", "value": "⚪ Acknowledged drift"},
					{"name": "Prod environment", "value": "🟢 No drift"},
				},
			},
			{
				"activityTitle": "Note",
				"activityText":  "✅ This is a test notification",
			},
		},
		"potentialAction": []map[string]interface{}{
			{
				"@type": "OpenUri",
				"name":  "View Dashboard",
				"targets": []map[string]interface{}{
					{"os": "default", "uri": os.Getenv("DIGGER_APP_URL")},
				},
			},
		},
	}

	jsonPayload, err := json.Marshal(messageCard)
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

type TestSlackNotificationForUrl struct {
	SlackNotificationUrl string `json:"notification_url"`
}

type TestTeamsNotificationForUrl struct {
	TeamsNotificationUrl string `json:"notification_url"`
}

func (mc MainController) SendTestSlackNotificationForUrl(c *gin.Context) {
	var request TestSlackNotificationForUrl
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}
	slackNotificationUrl := request.SlackNotificationUrl

	err = sendTestSlackWebhook(slackNotificationUrl)
	if err != nil {
		log.Printf("Error sending slack notification: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error sending slack notification"})
		return
	}

	c.String(200, "ok")
}

func (mc MainController) SendTestTeamsNotificationForUrl(c *gin.Context) {
	var request TestTeamsNotificationForUrl
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}
	teamsNotificationUrl := request.TeamsNotificationUrl

	err = sendTestTeamsWebhook(teamsNotificationUrl)
	if err != nil {
		log.Printf("Error sending teams notification: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error sending teams notification"})
		return
	}

	c.String(200, "ok")
}

func sectionBlockForProject(project models.Project) (*slack.SectionBlock, error) {
	switch project.DriftStatus {
	case models.DriftStatusNoDrift:
		sectionBlock := slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%v/dashboard/projects/%v|%v>", os.Getenv("DIGGER_APP_URL"), project.ID, project.Name), false, false),
				slack.NewTextBlockObject("mrkdwn", ":large_green_circle: No Drift", false, false),
			},
			nil,
		)
		return sectionBlock, nil
	case models.DriftStatusAcknowledgeDrift:
		sectionBlock := slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%v/dashboard/projects/%v|%v>", os.Getenv("DIGGER_APP_URL"), project.ID, project.Name), false, false),
				slack.NewTextBlockObject("mrkdwn", ":white_circle: Acknowledged Drift", false, false),
			},
			nil,
		)
		return sectionBlock, nil
	case models.DriftStatusNewDrift:
		sectionBlock := slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%v/dashboard/projects/%v|%v>", os.Getenv("DIGGER_APP_URL"), project.ID, project.Name), false, false),
				slack.NewTextBlockObject("mrkdwn", ":large_yellow_circle: Drift detected", false, false),
			},
			nil,
		)
		return sectionBlock, nil
	default:
		return nil, fmt.Errorf("Could not")
	}
}

type RealSlackNotificationForOrgRequest struct {
	OrgId uint `json:"org_id"`
}

func (mc MainController) SendRealSlackNotificationForOrg(c *gin.Context) {
	var request RealSlackNotificationForOrgRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}
	orgId := request.OrgId

	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("could not get org %v err: %v", orgId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Could not get org %v", orgId)})
		return
	}

	slackNotificationUrl := org.DriftWebhookUrl

	projects, err := models.DB.LoadProjectsForOrg(orgId)
	if err != nil {
		log.Printf("could not load projects for org %v err: %v", orgId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Could not load projects for org %v", orgId)})
		return
	}

	numOfProjectsWithDriftEnabled := 0
	var messageBlocks []slack.Block
	fieldsBlock := slack.NewSectionBlock(
		nil,
		[]*slack.TextBlockObject{
			slack.NewTextBlockObject("mrkdwn", "*Project*", false, false),
			slack.NewTextBlockObject("mrkdwn", "*Status*", false, false),
		},
		nil,
	)
	messageBlocks = append(messageBlocks, fieldsBlock)
	messageBlocks = append(messageBlocks, slack.NewDividerBlock())
	for _, project := range projects {
		if project.DriftEnabled {
			numOfProjectsWithDriftEnabled++
			sectionBlockForProject, err := sectionBlockForProject(*project)
			if err != nil {
				log.Printf("could not get block for project: %v err: %v", project.ID, err)
				c.JSON(500, gin.H{"error": fmt.Sprintf("could not get notification block for project %v", project.ID)})
				return
			}
			messageBlocks = append(messageBlocks, sectionBlockForProject)
			messageBlocks = append(messageBlocks, slack.NewDividerBlock())
		}
	}

	if numOfProjectsWithDriftEnabled == 0 {
		log.Printf("no projects with drift enabled for org: %v, succeeding", orgId)
		c.String(200, "ok")
		return
	}

	msg := &slack.WebhookMessage{
		Blocks: &slack.Blocks{
			BlockSet: messageBlocks,
		},
	}

	err = slack.PostWebhook(slackNotificationUrl, msg)
	if err != nil {
		log.Printf("error sending slack webhook: %v", err)
		c.JSON(500, gin.H{"error": "error sending slack webhook"})
		return
	}

	c.String(200, "ok")
}

func createTeamsMessageCardForProjects(projects []*models.Project) map[string]interface{} {
	facts := []map[string]string{
		{"name": "Project", "value": "Status"},
	}

	var sections []map[string]interface{}

	for _, project := range projects {
		if project.DriftEnabled {
			var statusValue string

			switch project.DriftStatus {
			case models.DriftStatusNoDrift:
				statusValue = "🟢 No Drift"
			case models.DriftStatusAcknowledgeDrift:
				statusValue = "⚪ Acknowledged Drift"
			case models.DriftStatusNewDrift:
				statusValue = "🟡 Drift detected"
			default:
				statusValue = "❓ Unknown"
			}

			facts = append(facts, map[string]string{
				"name":  project.Name,
				"value": statusValue,
			})
		}
	}

	section := map[string]interface{}{
		"activityTitle":    "Digger Drift Detection Report",
		"activitySubtitle": fmt.Sprintf("Found %d projects with drift enabled", len(facts)-1),
		"facts":           facts,
	}

	sections = append(sections, section)

	messageCard := map[string]interface{}{
		"@type":       "MessageCard",
		"@context":    "http://schema.org/extensions",
		"themeColor":  "0076D7",
		"summary":     "Digger Drift Detection Report",
		"sections":    sections,
		"potentialAction": []map[string]interface{}{
			{
				"@type": "OpenUri",
				"name":  "View Dashboard",
				"targets": []map[string]interface{}{
					{"os": "default", "uri": os.Getenv("DIGGER_APP_URL")},
				},
			},
		},
	}

	return messageCard
}

func (mc MainController) SendRealTeamsNotificationForOrg(c *gin.Context) {
	var request RealSlackNotificationForOrgRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}
	orgId := request.OrgId

	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil {
		log.Printf("could not get org %v err: %v", orgId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Could not get org %v", orgId)})
		return
	}

	teamsNotificationUrl := org.DriftTeamsWebhookUrl

	projects, err := models.DB.LoadProjectsForOrg(orgId)
	if err != nil {
		log.Printf("could not load projects for org %v err: %v", orgId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Could not load projects for org %v", orgId)})
		return
	}

	numOfProjectsWithDriftEnabled := 0
	for _, project := range projects {
		if project.DriftEnabled {
			numOfProjectsWithDriftEnabled++
		}
	}

	if numOfProjectsWithDriftEnabled == 0 {
		log.Printf("no projects with drift enabled for org: %v, succeeding", orgId)
		c.String(200, "ok")
		return
	}

	messageCard := createTeamsMessageCardForProjects(projects)

	jsonPayload, err := json.Marshal(messageCard)
	if err != nil {
		log.Printf("error marshalling teams message card: %v", err)
		c.JSON(500, gin.H{"error": "error marshalling teams message card"})
		return
	}

	resp, err := http.Post(teamsNotificationUrl, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Printf("error sending teams webhook: %v", err)
		c.JSON(500, gin.H{"error": "error sending teams webhook"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("teams webhook got unexpected status for org: %v - status: %v", org.ID, resp.StatusCode)
		c.JSON(500, gin.H{"error": "teams webhook got unexpected status"})
		return
	}

	c.String(200, "ok")
}

func (mc MainController) ProcessAllNotifications(c *gin.Context) {
	diggerHostname := os.Getenv("DIGGER_HOSTNAME")
	webhookSecret := os.Getenv("DIGGER_WEBHOOK_SECRET")
	var orgs []*models.Organisation
	err := models.DB.GormDB.Find(&orgs).Error
	if err != nil {
		log.Printf("could not select all orgs: %v", err)
	}

	sendSlackNotificationUrl, err := url.JoinPath(diggerHostname, "_internal/send_slack_notification_for_org")
	if err != nil {
		log.Printf("could not form drift url: %v", err)
		c.JSON(500, gin.H{"error": "could not form drift url"})
		return
	}

	sendTeamsNotificationUrl, err := url.JoinPath(diggerHostname, "_internal/send_teams_notification_for_org")
	if err != nil {
		log.Printf("could not form teams drift url: %v", err)
		c.JSON(500, gin.H{"error": "could not form teams drift url"})
		return
	}

	for _, org := range orgs {
		if org.DriftEnabled == false {
			continue
		}
		cron := org.DriftCronTab
		matches, err := utils.MatchesCrontab(cron, time.Now().Add((-7 * time.Minute)))
		if err != nil {
			log.Printf("could not check matching crontab for org: %v %v", org.ID, err)
			continue
		}

		if matches {
			fmt.Println("Matching org ID: ", org.ID)
			payload := RealSlackNotificationForOrgRequest{OrgId: org.ID}

			// Convert payload to JSON
			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				fmt.Println("Process Drift: error marshaling JSON:", err)
				continue
			}

			// Create a new request
			req, err := http.NewRequest("POST", sendSlackNotificationUrl, bytes.NewBuffer(jsonPayload))
			if err != nil {
				fmt.Println("Process slack notification: Error creating request:", err)
				continue
			}

			// Set headers
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", webhookSecret))

			// Send the request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("Error sending request:", err)
				continue
			}
			defer resp.Body.Close()

			// Get the status code
			statusCode := resp.StatusCode
			if statusCode != 200 {
				log.Printf("send slack notification got unexpected status for org: %v - status: %v", org.ID, statusCode)
			}

			// Send MS Teams notification if webhook URL is configured
			if org.DriftTeamsWebhookUrl != "" {
				fmt.Println("Sending teams notification for org ID: ", org.ID)
				teamsPayload := RealSlackNotificationForOrgRequest{OrgId: org.ID}

				// Convert payload to JSON
				teamsJsonPayload, err := json.Marshal(teamsPayload)
				if err != nil {
					fmt.Println("Process Teams notification: error marshaling JSON:", err)
					continue
				}

				// Create a new request for MS Teams
				teamsReq, err := http.NewRequest("POST", sendTeamsNotificationUrl, bytes.NewBuffer(teamsJsonPayload))
				if err != nil {
					fmt.Println("Process teams notification: Error creating request:", err)
					continue
				}

				// Set headers
				teamsReq.Header.Set("Content-Type", "application/json")
				teamsReq.Header.Set("Authorization", fmt.Sprintf("Bearer %v", webhookSecret))

				// Send the request
				teamsClient := &http.Client{}
				teamsResp, err := teamsClient.Do(teamsReq)
				if err != nil {
					fmt.Println("Error sending teams request:", err)
					continue
				}
				teamsResp.Body.Close()

				// Get the status code
				teamsStatusCode := teamsResp.StatusCode
				if teamsStatusCode != 200 {
					log.Printf("send teams notification got unexpected status for org: %v - status: %v", org.ID, teamsStatusCode)
				}
			}
		}
	}

	c.String(200, "success")
}
