package utils

import (
	"fmt"
	"github.com/robfig/cron/v3"
	"time"
)

func MatchesCrontab(cronString string, timestamp time.Time, resolution time.Duration) (bool, error) {
	// Parse the crontab string
	schedule, err := cron.ParseStandard(cronString)
	if err != nil {
		return false, fmt.Errorf("failed to parse crontab string: %w", err)
	}

	// Round down the timestamp to the nearest minute
	roundedTime := timestamp.Truncate(resolution)

	// Check if the rounded time matches the schedule
	nextTime := schedule.Next(roundedTime.Add(-resolution))
	return nextTime.Equal(roundedTime), nil
}
