package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	utils2 "github.com/diggerhq/digger/next/utils"
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

type TestSlackNotificationForUrl struct {
	SlackNotificationUrl string `json:"notification_url"`
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

func sectionBlockForProject(project models.Project) (*slack.SectionBlock, error) {
	switch project.DriftStatus {
	case models.DriftStatusNoDrift:
		sectionBlock := slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%v/project/%v|%v>", os.Getenv("DIGGER_APP_URL"), project.ID, project.Name), false, false),
				slack.NewTextBlockObject("mrkdwn", ":large_green_circle: No Drift", false, false),
			},
			nil,
		)
		return sectionBlock, nil
	case models.DriftStatusAcknowledgeDrift:
		sectionBlock := slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%v/project/%v|%v>", os.Getenv("DIGGER_APP_URL"), project.ID, project.Name), false, false),
				slack.NewTextBlockObject("mrkdwn", ":white_circle: Acknowledged Drift", false, false),
			},
			nil,
		)
		return sectionBlock, nil
	case models.DriftStatusNewDrift:
		sectionBlock := slack.NewSectionBlock(
			nil,
			[]*slack.TextBlockObject{
				slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("<%v/project/%v|%v>", os.Getenv("DIGGER_APP_URL"), project.ID, project.Name), false, false),
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

	slackNotificationUrl := org.SlackNotificationUrl

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

	for _, org := range orgs {
		cron := org.DriftCronTab
		matches, err := utils2.MatchesCrontab(cron, time.Now().Add((-7 * time.Minute)))
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
		}
	}

	c.String(200, "success")
}
