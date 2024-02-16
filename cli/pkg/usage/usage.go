package usage

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

var source = "unknown"

type UsageRecord struct {
	UserId    interface{} `json:"userid"`
	EventName string      `json:"event_name"`
	Action    string      `json:"action"`
	Token     string      `json:"token"`
}

func SendUsageRecord(repoOwner string, eventName string, action string) error {
	payload := UsageRecord{
		UserId:    repoOwner,
		EventName: eventName,
		Action:    action,
		Token:     "diggerABC@@1998fE",
	}
	return sendPayload(payload)
}

func SendLogRecord(repoOwner string, message string) error {
	payload := UsageRecord{
		UserId:    repoOwner,
		EventName: "log from " + source,
		Action:    message,
		Token:     "diggerABC@@1998fE",
	}
	return sendPayload(payload)
}

func sendPayload(payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling usage record: %v", err)
		return err
	}
	req, _ := http.NewRequest("POST", "https://analytics.digger.dev", bytes.NewBuffer(jsonData))

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error sending usage record: %v. If you are using digger in a firewalled environment "+
			"please whitelist analytics.digger.dev", err)
		return err
	}
	defer resp.Body.Close()
	return nil
}

func init() {
	notEmpty := func(key string) bool {
		return os.Getenv(key) != ""
	}

	if notEmpty("GITHUB_ACTIONS") {
		source = "github"
	}
	if notEmpty("GITLAB_CI") {
		source = "gitlab"
	}
	if notEmpty("BITBUCKET_BUILD_NUMBER") {
		source = "bitbucket"
	}
	if notEmpty("AZURE_CI") {
		source = "azure"
	}

}
