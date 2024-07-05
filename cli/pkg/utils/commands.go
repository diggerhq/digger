package utils

import (
	"fmt"
	"log"
)

type Command struct {
	Name        string
	Description string
}

var availableCommands = []Command{
	{"digger help", "Display help information"},
	{"digger version", "Display version information"},
	{"digger apply", "Apply the Terraform  digger_config"},
	{"digger plan", "Plan the Terraform  digger_config"},
	{"digger show-projects", "Show the impacted projects"},
	{"digger lock", "Lock Terraform project"},
	{"digger unlock", "Unlock the Terraform project"},
}

func DisplayCommands() {
	log.Println("Use the following commands to get started:")
	for _, command := range availableCommands {
		log.Printf("  %s: %s\n", command.Name, command.Description)
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
