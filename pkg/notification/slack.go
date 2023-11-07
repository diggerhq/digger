package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Notification interface {
	Send(message string) error
}

type SlackNotification struct {
	Url string
}

func (slack SlackNotification) Send(message string) error {
	httpClient := &http.Client{}
	type SlackMessage struct {
		Text string `json:"text"`
	}
	slackMessage := SlackMessage{
		Text: message,
	}

	jsonData, err := json.Marshal(slackMessage)
	if err != nil {
		msg := fmt.Sprintf("failed to marshal slack message. %v", err)
		log.Printf(msg)
		return fmt.Errorf(msg)
	}

	request, err := http.NewRequest("POST", slack.url, bytes.NewBuffer(jsonData))
	if err != nil {
		msg := fmt.Sprintf("failed to create slack notification request. %v", err)
		log.Printf(msg)
		return fmt.Errorf(msg)
	}

	request.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(request)
	if err != nil {
		msg := fmt.Sprintf("failed to send slack notification request. %v", err)
		log.Printf(msg)
	}
	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			msg := fmt.Sprintf("failed to read response body. %v", err)
			log.Printf(msg)
			return fmt.Errorf(msg)
		}
		msg := fmt.Sprintf("failed to send slack notification request. %v. Message: %v", resp.Status, body)
		log.Printf(msg)
		return fmt.Errorf(msg)
	}
	defer resp.Body.Close()
	return nil
}
