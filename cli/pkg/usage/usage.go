package usage

import (
	"bytes"
	"crypto/sha256"
        "encoding/hex"
	"encoding/json"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"log"
	"net/http"
	"os"
)

var collect_usage_data = false
var source = "unknown"

func hash(input string) string {
    sha256Hash := sha256.New()
    sha256Hash.Write([]byte(input))
    hashedValue := sha256Hash.Sum(nil)
    hashedString := hex.EncodeToString(hashedValue)

    return hashedString
}


type UsageRecord struct {
	UserId    interface{} `json:"userid"`
	EventName string      `json:"event_name"`
	Action    string      `json:"action"`
	Token     string      `json:"token"`
}

func SendUsageRecord(repoOwner string, eventName string, action string) error {
	payload := UsageRecord{
		UserId:    hash(repoOwner),
		EventName: eventName,
		Action:    action,
		Token:     "diggerABC@@1998fE",
	}
	return sendPayload(payload)
}

func SendLogRecord(repoOwner string, message string) error {
	payload := UsageRecord{
		UserId:    hash(repoOwner),
		EventName: "log from " + source,
		Action:    message,
		Token:     "diggerABC@@1998fE",
	}
	return sendPayload(payload)
}

func sendPayload(payload interface{}) error {
	if !collect_usage_data {
		return nil
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling usage record: %v", err)
		return err
	}
	req, _ := http.NewRequest("POST", "https://analytics.digger.dev", bytes.NewBuffer(jsonData))

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Error sending usage record: %v", err)
		return err
	}
	defer resp.Body.Close()
	return nil
}

func init() {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("Failed to get current dir. %s", err)
	}
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

	config, _, _, err := configuration.LoadDiggerConfig(currentDir)
	if err != nil {
		return
	}
	if !config.CollectUsageData {
		collect_usage_data = false
	} else if os.Getenv("COLLECT_USAGE_DATA") == "true" {
		collect_usage_data = true
	} else {
		collect_usage_data = false
	}
}
