package drift

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
)

const (
	// Slack collapses messages beyond ~4000 characters, so larger content is
	// split into parts of at most this size (pre-existing behaviour).
	slackMessageLimit = 4000
	// Max plan characters posted as threaded replies via a bot token. Slack
	// rejects single messages above 40000 chars; threads keep the channel to
	// one message, so the cap here bounds the number of replies instead.
	threadPlanLimit = 40000

	defaultSlackApiBase = "https://slack.com/api"
)

// Pause between thread replies: Slack allows roughly one chat.postMessage
// per second per channel. Package-level so tests can zero it.
var threadPostInterval = time.Second

type SlackNotification struct {
	Url string
	// Optional: when BotToken and Channel are set the notification is sent
	// via chat.postMessage and the plan goes into a thread under the header
	// message instead of flooding the channel.
	BotToken string
	Channel  string
	// Overridable for tests; defaults to https://slack.com/api
	ApiBase string
}

// cutAtRuneBoundary slices s to at most max bytes without splitting a
// multi-byte UTF-8 rune.
func cutAtRuneBoundary(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}

// sanitizeFences breaks up ``` sequences inside plan text (heredocs, string
// values) with a zero-width space so they cannot terminate the enclosing
// Slack code block early.
func sanitizeFences(s string) string {
	return strings.ReplaceAll(s, "```", "`\u200b``")
}

// TruncatePlan cuts plan down to at most maxChars characters (at a line
// boundary where possible) and appends a note saying how much was omitted.
func TruncatePlan(plan string, maxChars int) (string, bool) {
	if len(plan) <= maxChars {
		return plan, false
	}
	cut := cutAtRuneBoundary(plan, maxChars)
	// only retreat to a line boundary when it doesn't throw away most of the
	// window (a plan can be one giant line, e.g. jsonencode output)
	if idx := strings.LastIndex(cut, "\n"); idx >= maxChars/2 {
		cut = cut[:idx]
	}
	omitted := len(plan) - len(cut)
	return cut + fmt.Sprintf("\n... plan truncated (%d of %d characters shown, %d omitted) — full plan in the workflow logs", len(cut), len(plan), omitted), true
}

// splitIntoFencedChunks splits text into chunks at line boundaries, each
// wrapped in a ``` code fence. Chunks including their fences stay within
// chunkSize characters.
func splitIntoFencedChunks(text string, chunkSize int) []string {
	budget := chunkSize - len("```\n") - len("\n```")
	var chunks []string
	var part strings.Builder
	for _, line := range strings.Split(text, "\n") {
		// oversized single line: hard-split it
		for len(line) > budget {
			if part.Len() > 0 {
				chunks = append(chunks, "```\n"+part.String()+"\n```")
				part.Reset()
			}
			head := cutAtRuneBoundary(line, budget)
			chunks = append(chunks, "```\n"+head+"\n```")
			line = line[len(head):]
		}
		if part.Len()+len(line)+1 > budget && part.Len() > 0 {
			chunks = append(chunks, "```\n"+part.String()+"\n```")
			part.Reset()
		}
		if part.Len() > 0 {
			part.WriteString("\n")
		}
		part.WriteString(line)
	}
	if part.Len() > 0 {
		chunks = append(chunks, "```\n"+part.String()+"\n```")
	}
	return chunks
}

func driftHeader(projectName string, repoFullName string, lastChange *core_drift.LastChange) string {
	lastChangeLine := ""
	if lastChange != nil {
		lastChangeLine = fmt.Sprintf("*Last change by:* %s (`%s`, %s)\n", lastChange.Author, lastChange.Commit, lastChange.When)
	}
	return fmt.Sprintf(
		":warning: *Infrastructure Drift Detected* :warning:\n\n"+
			"*Project:* `%s`\n"+
			"*Repository:* `%s`\n"+
			"%s",
		projectName, repoFullName, lastChangeLine,
	)
}

func (slack *SlackNotification) useThreads() bool {
	return slack.BotToken != "" && slack.Channel != ""
}

// SendNotificationForProject sends a compact summary per drifted project.
// The plan itself never goes into the channel — with many repos and
// projects that becomes pure noise — it is either posted into the summary
// message's thread (bot-token mode) or left in the workflow logs.
func (slack *SlackNotification) SendNotificationForProject(projectName string, repoFullName string, plan string, lastChange *core_drift.LastChange) error {
	header := driftHeader(projectName, repoFullName, lastChange)

	if slack.useThreads() {
		return slack.sendThreaded(header, plan)
	}

	message := header + "\n_Full Terraform plan in the workflow logs._"
	err := SendSlackMessage(slack.Url, message)
	if err != nil {
		slog.Error("failed to send slack drift request", "error", err)
	}
	return err
}

// sendThreaded posts the header to the channel and the plan as replies in
// its thread, so the channel only ever sees a single message per project.
func (slack *SlackNotification) sendThreaded(header string, plan string) error {
	ts, err := slack.postMessage(header+"\n_Terraform plan in thread._", "")
	if err != nil {
		return err
	}

	plan, truncated := TruncatePlan(plan, threadPlanLimit)
	if truncated {
		slog.Warn("drift plan truncated for slack thread notification")
	}
	for i, chunk := range splitIntoFencedChunks(sanitizeFences(plan), slackMessageLimit) {
		if i > 0 {
			// stay under Slack's ~1 message/second/channel posting limit
			time.Sleep(threadPostInterval)
		}
		if _, err := slack.postMessage(chunk, ts); err != nil {
			return err
		}
	}
	return nil
}

// postMessage calls chat.postMessage; when threadTs is non-empty the message
// is posted as a reply in that thread. Retries on 429 honoring Retry-After.
// Returns the created message's ts.
func (slack *SlackNotification) postMessage(text string, threadTs string) (string, error) {
	apiBase := slack.ApiBase
	if apiBase == "" {
		apiBase = defaultSlackApiBase
	}

	payload := map[string]string{
		"channel": slack.Channel,
		"text":    text,
	}
	if threadTs != "" {
		payload["thread_ts"] = threadTs
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal slack message: %v", err)
	}

	const maxAttempts = 3
	for attempt := 1; ; attempt++ {
		request, err := http.NewRequest("POST", apiBase+"/chat.postMessage", bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("failed to create slack request: %v", err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+slack.BotToken)

		resp, err := (&http.Client{}).Do(request)
		if err != nil {
			return "", fmt.Errorf("failed to send slack request: %v", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("failed to read slack response: %v", readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxAttempts {
			delay := 2 * time.Second
			if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs > 0 && secs <= 30 {
				delay = time.Duration(secs) * time.Second
			}
			slog.Warn("slack rate limited, retrying", "attempt", attempt, "delay", delay)
			time.Sleep(delay)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("slack api returned status %v: %s", resp.Status, body)
		}

		var apiResp struct {
			Ok    bool   `json:"ok"`
			Ts    string `json:"ts"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(body, &apiResp); err != nil {
			return "", fmt.Errorf("failed to parse slack response: %v", err)
		}
		if !apiResp.Ok {
			return "", fmt.Errorf("slack api error: %v", apiResp.Error)
		}
		return apiResp.Ts, nil
	}
}

func (slack *SlackNotification) SendErrorNotificationForProject(projectName string, repoFullName string, err error) error {
	message := fmt.Sprintf(
		":warning: *Error While Drift Processing* :warning:\n\n"+
			"*Project:* `%s`\n"+
			"*Repository:* `%s`\n\n"+
			"*Error Details:*\n```\n%v\n```\n\n"+
			"_Please check the workflow logs for more information._",
		projectName, repoFullName, err,
	)

	if slack.useThreads() {
		_, sendErr := slack.postMessage(message, "")
		return sendErr
	}
	return SendSlackMessage(slack.Url, message)
}

func (slack *SlackNotification) Flush() error {
	return nil
}
