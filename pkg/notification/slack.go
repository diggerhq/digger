package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
)

type Notification interface {
	Send(message string) error
}

type SlackNotification struct {
	Url string
}

func SplitCodeBlocks(message string) []string {
	var res []string
	regex := regexp.MustCompile(`\\n`)
	split := regex.Split(message, -1)
	part := ""
	for _, line := range split {
		if len(part+line) > 4000 {
			res = append(res, part+"\n"+line+"\n```")
			part = "```\n" + line
		} else {
			part = part + "\n" + line
		}
	}
	if len(part) > 0 {
		res = append(res, part)
	}
	return res
}

func (slack SlackNotification) Send(message string) error {
	httpClient := &http.Client{}
	type SlackMessage struct {
		Text string `json:"text"`
	}
	parts := SplitCodeBlocks(message)
	for _, part := range parts {
		slackMessage := SlackMessage{
			Text: part,
		}

		jsonData, err := json.Marshal(slackMessage)
		if err != nil {
			msg := fmt.Sprintf("failed to marshal slack message. %v", err)
			log.Printf(msg)
			return fmt.Errorf(msg)
		}

		request, err := http.NewRequest("POST", slack.Url, bytes.NewBuffer(jsonData))
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
	}

	return nil
}
