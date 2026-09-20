package digger

import (
	"fmt"
	"strings"

	orchestrator "github.com/diggerhq/digger/libs/scheduler"
)

const FailOnChangesFlag = "--fail-on-changes"

// ParseFailOnChangesFlag returns the command with --fail-on-changes removed and whether it was
// there, looking only at the first non blank line, which is the command line
// Leaves non digger commands (a "run ..." step) untouched
func ParseFailOnChangesFlag(rawCommand string) (string, bool, error) {
	commandLine := ""
	for _, line := range strings.Split(rawCommand, "\n") {
		if strings.TrimSpace(line) != "" {
			commandLine = strings.TrimSpace(line)
			break
		}
	}
	if !strings.HasPrefix(strings.ToLower(commandLine), "digger ") {
		return rawCommand, false, nil
	}

	fields := strings.Fields(commandLine)
	kept := make([]string, 0, len(fields))
	failOnChanges := false
	for _, field := range fields {
		name, _, hasValue := strings.Cut(strings.ToLower(field), "=")
		if name != FailOnChangesFlag && name != strings.TrimPrefix(FailOnChangesFlag, "-") {
			kept = append(kept, field)
			continue
		}
		if hasValue {
			return commandLine, false, fmt.Errorf("%v takes no value, drop the '=' part of '%v'", FailOnChangesFlag, field)
		}
		failOnChanges = true
	}
	if !failOnChanges {
		return rawCommand, false, nil
	}

	command := strings.Join(kept, " ")
	loweredCommand := strings.ToLower(command)
	if loweredCommand != "digger plan" && !strings.HasPrefix(loweredCommand, "digger plan ") {
		return command, false, fmt.Errorf("%v is only supported on 'digger plan', got '%v'", FailOnChangesFlag, commandLine)
	}

	return command, true, nil
}

// ApplyCommandFlags moves flags off the command strings onto the job itself, so that
// everything downstream keeps matching bare commands such as "digger plan".
func ApplyCommandFlags(jobs []orchestrator.Job) error {
	for i := range jobs {
		// jobs sharing a workflow share this slice with the digger_config, don't rewrite it
		commands := make([]string, len(jobs[i].Commands))
		for j, rawCommand := range jobs[i].Commands {
			command, failOnChanges, err := ParseFailOnChangesFlag(rawCommand)
			if err != nil {
				return err
			}
			commands[j] = command
			jobs[i].FailOnChanges = jobs[i].FailOnChanges || failOnChanges
		}
		jobs[i].Commands = commands
	}
	return nil
}
