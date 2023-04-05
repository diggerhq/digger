package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

var collect_usage_data = true

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
		EventName: "log",
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
	req, _ := http.NewRequest("POST", "https://i2smwjphd4.execute-api.us-east-1.amazonaws.com/prod/", bytes.NewBuffer(jsonData))

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
	config, err := NewDiggerConfig("")
	if err != nil {
		return
	}
	if config.CollectUsageData != nil && !*config.CollectUsageData {
		collect_usage_data = false
	} else if config.CollectUsageData == nil && os.Getenv("COLLECT_USAGE_DATA") == "false" {
		collect_usage_data = false
	} else {
		collect_usage_data = true
	}
}
