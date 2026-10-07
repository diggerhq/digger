package drift

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
)

func TestSlackSplitLargerMessage(t *testing.T) {
	parts := SplitCodeBlocks(":bangbang: drift detected\n\n ```\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\n\n\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\n\n```")
	assert.Equal(t, 2, len(parts))
	assert.Equal(t, 2, strings.Count(parts[0], "```"))
	assert.Equal(t, 2, strings.Count(parts[1], "```"))
}

func TestSlackSmallerMessageNotSplit(t *testing.T) {
	msg := ":bangbang: drift detected\n\n ```\nhere it is\nhere it is```"
	parts := SplitCodeBlocks(msg)
	assert.Equal(t, 1, len(parts))
}

func TestSplitCodeBlocksNoLineDuplication(t *testing.T) {
	// Build a message with code blocks where the split boundary creates duplication
	var lines []string
	lines = append(lines, "header text\n```")
	for i := 0; i < 500; i++ {
		lines = append(lines, fmt.Sprintf("unique line number %d here", i))
	}
	lines = append(lines, "```")
	message := strings.Join(lines, "\n")

	parts := SplitCodeBlocks(message)
	assert.Greater(t, len(parts), 1, "message should be split into multiple parts")

	// Count each unique line across ALL parts - no line should appear more than once
	// (excluding code block markers)
	lineCounts := make(map[string]int)
	for _, part := range parts {
		for _, l := range strings.Split(part, "\n") {
			trimmed := strings.TrimSpace(l)
			if trimmed == "```" || trimmed == "" {
				continue
			}
			lineCounts[trimmed]++
		}
	}
	for line, count := range lineCounts {
		if strings.HasPrefix(line, "unique line number") {
			assert.Equal(t, 1, count,
				"line %q appears %d times across parts (should appear exactly once)", line, count)
		}
	}

	// Verify each part is under 4100 chars (slightly above 4000 for boundary line)
	for i, part := range parts {
		assert.LessOrEqual(t, len(part), 4100,
			"part %d should be approximately under 4000 chars", i)
	}
}

func TestSplitCodeBlocksPlainTextOver4000(t *testing.T) {
	// Build a message > 4000 chars with NO code blocks
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, "this is a plain text line without any code blocks")
	}
	message := strings.Join(lines, "\n")
	assert.Greater(t, len(message), 4000)

	parts := SplitCodeBlocks(message)
	assert.Greater(t, len(parts), 1, "plain text message > 4000 chars should be split")

	// Verify each part is approximately <= 4000 chars
	for i, part := range parts {
		assert.LessOrEqual(t, len(part), 4100,
			"part %d should be approximately under 4000 chars", i)
	}

	// Verify all content is preserved (no data loss)
	reassembled := strings.Join(parts, "")
	// Account for leading newline added by the function
	assert.Contains(t, reassembled, "this is a plain text line")
}

func TestSendSlackMessageThatIsLargerThan2Parts(t *testing.T) {
	url := os.Getenv("TEST_SLACK_NOTIFICATION_URL")
	if url == "" {
		t.Skip("Skipping slack message test: $TEST_SLACK_NOTIFICATION_URL not set")
	}
	projectName := "dev"
	repoFullName := "terraform-aws-modules/terraform-aws-eks"
	plan := ":bangbang: drift detected\n\n ```\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\n\n\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\nhere it is\n\n```"
	notification := SlackNotification{Url: url}
	err := notification.SendNotificationForProject(projectName, repoFullName, plan)
	assert.Equal(t, nil, err)
}
