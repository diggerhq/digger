package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDuplicateCommandWindow(t *testing.T) {
	for value, want := range map[string]time.Duration{
		"":    2 * time.Hour,
		"30m": 30 * time.Minute,
		"0":   0,
		"-1h": 2 * time.Hour,
		"bad": 2 * time.Hour,
	} {
		t.Setenv("DIGGER_DUPLICATE_COMMAND_WINDOW", value)
		assert.Equal(t, want, DuplicateCommandWindow(), "value %q", value)
	}
}
