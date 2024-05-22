package orchestrator

import (
	"fmt"
	"regexp"
	"strings"
)

func ParseProjectName(comment string) string {
	re := regexp.MustCompile(`-p ([0-9a-zA-Z\-_]+)`)
	match := re.FindStringSubmatch(comment)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

type DiggerCommand string

const DiggerCommandPlan DiggerCommand = "plan"
const DiggerCommandApply DiggerCommand = "apply"
const DiggerCommandLock DiggerCommand = "lock"
const DiggerCommandUnlock DiggerCommand = "unlock"

func GetCommandFromComment(comment string) (*DiggerCommand, error) {
	supportedCommands := map[string]DiggerCommand{
		"digger plan":   DiggerCommandPlan,
		"digger apply":  DiggerCommandApply,
		"digger unlock": DiggerCommandUnlock,
		"digger lock":   DiggerCommandLock,
	}
	diggerCommand := strings.ToLower(comment)
	diggerCommand = strings.TrimSpace(diggerCommand)
	for command, value := range supportedCommands {
		if strings.HasPrefix(diggerCommand, command) {
			return &value, nil
		}
	}
	return nil, fmt.Errorf("Unreqcognised command: %v", comment)
}
