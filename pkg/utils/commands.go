package utils

import (
	"errors"
	"fmt"
	"regexp"
)

type Commad struct {
	Name        string
	Description string
}

var availableCommands = []Commad{
	{"digger help", "Display help information"},
	{"digger version", "Display version information"},
	{"digger apply", "Apply the Terraform  configuration"},
	{"digger plan", "Plan the Terraform  configuration"},
	{"digger lock", "Lock Terraform project"},
	{"digger unlock", "Unlock the Terraform project"},
}

func DisplayCommands() {
	fmt.Println("Use the following commands to get started:")
	for _, command := range availableCommands {
		fmt.Printf("  %s: %s\n", command.Name, command.Description)
	}
}

// display commands as string
func GetCommands() string {
	var commands string
	for _, command := range availableCommands {
		commands += fmt.Sprintf("  %s: %s\n", command.Name, command.Description)
	}
	return commands
}

// TODO move func to lib-orchestrator library after gitlab and azure moves there
func ParseProjectName(comment string) string {
	re := regexp.MustCompile(`-p ([0-9a-zA-Z\-_]+)`)
	match := re.FindStringSubmatch(comment)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

// TODO move func to lib-orchestrator library after gitlab and azure moves there
func ParseWorkspace(comment string) (string, error) {
	re := regexp.MustCompile(`-w(?:\s+(\S+)|$)`)
	matches := re.FindAllStringSubmatch(comment, -1)

	if len(matches) == 0 {
		return "", nil
	}

	if len(matches) > 1 {
		return "", errors.New("more than one -w flag found")
	}

	if len(matches[0]) < 2 || matches[0][1] == "" {
		return "", errors.New("no value found after -w flag")
	}

	return matches[0][1], nil
}
