package drift

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	core_drift "github.com/diggerhq/digger/cli/pkg/core/drift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendSlackMessageThatIsLargerThan2Parts(t *testing.T) {
	url := os.Getenv("TEST_SLACK_NOTIFICATION_URL")
	if url == "" {
		t.Skip("Skipping slack message test: $TEST_SLACK_NOTIFICATION_URL not set")
	}
	projectName := "dev"
	repoFullName := "terraform-aws-modules/terraform-aws-eks"
	plan := strings.Repeat("here it is\n", 800) // ~9k chars
	notification := SlackNotification{Url: url}
	lastChange := &core_drift.LastChange{Author: "Jane Doe", Commit: "abc1234", When: "3 days ago"}
	err := notification.SendNotificationForProject(projectName, repoFullName, plan, lastChange)
	assert.Equal(t, nil, err)
}

func TestTruncatePlanShortPlanUntouched(t *testing.T) {
	plan := "short plan"
	got, truncated := TruncatePlan(plan, 100)
	assert.False(t, truncated)
	assert.Equal(t, plan, got)
}

func TestTruncatePlanCutsAtLineBoundaryWithNotice(t *testing.T) {
	plan := strings.Repeat("resource line here\n", 1000) // ~19k chars
	got, truncated := TruncatePlan(plan, 5000)
	assert.True(t, truncated)
	assert.Less(t, len(got), 5200)
	assert.Contains(t, got, "plan truncated")
	assert.Contains(t, got, "full plan in the workflow logs")
	// cut on a line boundary: the last plan line before the notice is intact
	lines := strings.Split(got, "\n")
	assert.Equal(t, "resource line here", lines[len(lines)-2])
}

func TestSplitIntoFencedChunksPreservesAllLines(t *testing.T) {
	text := strings.Repeat("line of plan output\n", 500) // ~10k chars
	chunks := splitIntoFencedChunks(strings.TrimRight(text, "\n"), 4000)
	assert.GreaterOrEqual(t, len(chunks), 3)
	total := 0
	for _, c := range chunks {
		assert.True(t, strings.HasPrefix(c, "```\n"))
		assert.True(t, strings.HasSuffix(c, "\n```"))
		assert.LessOrEqual(t, len(c), 4000) // fences included in the budget
		total += strings.Count(c, "line of plan output")
	}
	assert.Equal(t, 500, total)
}

func TestSendNotificationThreadedPostsPlanInThread(t *testing.T) {
	type call struct {
		Channel  string `json:"channel"`
		Text     string `json:"text"`
		ThreadTs string `json:"thread_ts"`
	}
	var calls []call
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat.postMessage", r.URL.Path)
		require.Equal(t, "Bearer xoxb-test", r.Header.Get("Authorization"))
		var c call
		require.NoError(t, json.NewDecoder(r.Body).Decode(&c))
		calls = append(calls, c)
		fmt.Fprintf(w, `{"ok": true, "ts": "111.%d"}`, len(calls))
	}))
	defer server.Close()

	threadPostInterval = 0 // no pacing sleeps in tests
	notification := SlackNotification{BotToken: "xoxb-test", Channel: "C123", ApiBase: server.URL}
	plan := strings.Repeat("aws_instance.web must be replaced\n", 300) // ~10k chars -> 3 thread chunks
	lastChange := &core_drift.LastChange{Author: "Jane Doe", Commit: "abc1234", When: "3 days ago"}

	err := notification.SendNotificationForProject("prod", "org/repo", plan, lastChange)
	assert.NoError(t, err)

	require.GreaterOrEqual(t, len(calls), 4)
	// header goes to the channel, not a thread
	assert.Equal(t, "", calls[0].ThreadTs)
	assert.Equal(t, "C123", calls[0].Channel)
	assert.Contains(t, calls[0].Text, "Last change by")
	assert.Contains(t, calls[0].Text, "Jane Doe")
	assert.NotContains(t, calls[0].Text, "aws_instance") // plan is not in the header
	// every plan chunk is a reply in the header's thread
	for _, c := range calls[1:] {
		assert.Equal(t, "111.1", c.ThreadTs)
		assert.Contains(t, c.Text, "aws_instance")
	}
}

func TestSendNotificationWebhookSendsCompactSummaryWithoutPlan(t *testing.T) {
	var payloads []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m struct {
			Text string `json:"text"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&m))
		payloads = append(payloads, m.Text)
		w.WriteHeader(200)
	}))
	defer server.Close()

	notification := SlackNotification{Url: server.URL}
	plan := strings.Repeat("resource \"random_string\" \"x\" will be created\n", 4000) // ~180k chars like a big monorepo plan
	lastChange := &core_drift.LastChange{Author: "Jane Doe", Commit: "abc1234", When: "3 days ago"}
	err := notification.SendNotificationForProject("big-project", "org/repo", plan, lastChange)
	assert.NoError(t, err)

	// exactly one compact message per project, plan never hits the channel
	require.Equal(t, 1, len(payloads))
	assert.NotContains(t, payloads[0], "random_string")
	assert.Contains(t, payloads[0], "Last change by")
	assert.Contains(t, payloads[0], "workflow logs")
	// only the warning emoji survives
	assert.Contains(t, payloads[0], ":warning:")
	for _, emoji := range []string{":file_folder:", ":books:", ":memo:", ":bust_in_silhouette:"} {
		assert.NotContains(t, payloads[0], emoji)
	}
}
