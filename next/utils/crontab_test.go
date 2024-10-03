package utils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestCrontTabMatching(t *testing.T) {
	cronString := "*/15 * * * *" // Every 15 minutes
	timestamp := time.Date(2023, 5, 1, 12, 30, 30, 0, time.UTC)

	matches, err := MatchesCrontab(cronString, timestamp, time.Minute)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	assert.True(t, matches)

	cronString = "*/15 * * * *" // Every 15 minutes
	timestamp = time.Date(2022, 5, 1, 12, 12, 30, 0, time.UTC)

	matches, err = MatchesCrontab(cronString, timestamp, time.Minute)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	assert.False(t, matches)

}
