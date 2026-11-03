package scheduler

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
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

const DiggerCommandNoop DiggerCommand = "noop"
const DiggerCommandPlan DiggerCommand = "plan"
const DiggerCommandApply DiggerCommand = "apply"
const DiggerCommandLock DiggerCommand = "lock"
const DiggerCommandUnlock DiggerCommand = "unlock"

// IsCommentCommand reports whether a PR comment invokes command: "digger apply -p x"
// matches "digger apply", but "digger applyt" or "lgtm, digger apply" do not.
func IsCommentCommand(comment string, command string) bool {
	comment = strings.ToLower(strings.TrimSpace(comment))
	rest, found := strings.CutPrefix(comment, command)
	return found && (rest == "" || unicode.IsSpace(rune(rest[0])))
}

func GetCommandFromComment(comment string) (*DiggerCommand, error) {
	supportedCommands := map[string]DiggerCommand{
		"digger noop":   DiggerCommandNoop,
		"digger plan":   DiggerCommandPlan,
		"digger apply":  DiggerCommandApply,
		"digger unlock": DiggerCommandUnlock,
		"digger lock":   DiggerCommandLock,
	}
	diggerCommand := strings.ToLower(comment)
	diggerCommand = strings.TrimSpace(diggerCommand)
	for command, value := range supportedCommands {
		if IsCommentCommand(diggerCommand, command) {
			return &value, nil
		}
	}
	return nil, fmt.Errorf("Unrecognised command: %v", comment)
}

func GetCommandFromJob(job Job) (*DiggerCommand, error) {
	supportedCommands := map[string]DiggerCommand{
		"digger noop":   DiggerCommandNoop,
		"digger plan":   DiggerCommandPlan,
		"digger apply":  DiggerCommandApply,
		"digger unlock": DiggerCommandUnlock,
		"digger lock":   DiggerCommandLock,
	}

	if len(job.Commands) == 0 {
		res := DiggerCommandNoop
		return &res, nil
	}

	diggerCommands := job.Commands
	for command, value := range supportedCommands {
		for _, diggerCommand := range diggerCommands {
			if strings.HasPrefix(diggerCommand, command) {
				return &value, nil
			}
		}
	}
	return nil, fmt.Errorf("could not figure out command: %v", job.Commands)
}
