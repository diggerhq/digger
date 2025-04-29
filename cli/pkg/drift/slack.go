package drift

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

type SlackNotification struct {
	Url string
}

func SplitCodeBlocks(message string) []string {
	var res []string

	if strings.Count(message, "```") < 2 {
		res = append(res, message)
		return res
	}

	regex := regexp.MustCompile("\n")
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

func (slack SlackNotification) Send(projectName string, plan string) error {
	message := fmt.Sprintf(":bangbang: Drift detected in digger project %v details below: \n\n```\n%v\n```", projectName, plan)
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
			slog.Error("failed to marshal slack message", "error", err)
			return err
		}

		request, err := http.NewRequest("POST", slack.Url, bytes.NewBuffer(jsonData))
		if err != nil {
			slog.Error("failed to create slack drift request", "error", err)
			return err
		}

		request.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(request)
		if err != nil {
			slog.Error("failed to send slack drift request", "error", err)
			return err
		}
		if resp.StatusCode != 200 {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				slog.Error("\"failed to read response body", "error", err)
				return err
			}
			slog.Error("failed to send slack drift request", "status code", resp.Status, "body", body)
			msg := fmt.Sprintf("failed to send slack drift request. %v. Message: %v", resp.Status, body)
			return fmt.Errorf(msg)
		}
		resp.Body.Close()
	}

	return nil
}
