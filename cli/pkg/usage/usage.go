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

var telemetry = true
var source = "unknown"

type UsageRecord struct {
	UserId    interface{} `json:"userid"`
	EventName string      `json:"event_name"`
	Action    string      `json:"action"`
	Token     string      `json:"token"`
}

func SendUsageRecord(repoOwner string, eventName string, action string) error {
	h := sha256.New()
	h.Write([]byte(repoOwner))
	sha := h.Sum(nil)
	shaStr := hex.EncodeToString(sha)
	payload := UsageRecord{
		UserId:    shaStr,
		EventName: eventName,
		Action:    action,
		Token:     "diggerABC@@1998fE",
	}
	return sendPayload(payload)
}

func SendLogRecord(repoOwner string, message string) error {
	h := sha256.New()
	h.Write([]byte(repoOwner))
	sha := h.Sum(nil)
	shaStr := hex.EncodeToString(sha)
	payload := UsageRecord{
		UserId:    shaStr,
		EventName: "log from " + source,
		Action:    message,
		Token:     "diggerABC@@1998fE",
	}
	return sendPayload(payload)
}

func sendPayload(payload interface{}) error {
	if !telemetry {
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
		log.Printf("Error sending telemetry: %v. If you are using digger in a firewalled environment, "+
			"please consider whitelisting analytics.digger.dev. You can also disable this message by setting "+
			"telemetry: false in digger.yml", err)
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

	config, _, _, err := configuration.LoadDiggerConfig(currentDir, false, nil)
	if err != nil {
		return
	}
	if !config.Telemetry {
		telemetry = false
	} else if os.Getenv("TELEMETRY") == "false" {
		telemetry = false
	} else {
		telemetry = true
	}
}

func ReportErrorAndExit(repoOwner string, message string, exitCode int) {
	log.Println(message)
	err := SendLogRecord(repoOwner, message)
	if err != nil {
		log.Printf("Failed to send log record. %s\n", err)
	}
	os.Exit(exitCode)
}
