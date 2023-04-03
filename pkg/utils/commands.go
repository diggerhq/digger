package utils

import "fmt"

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
	{"digger unlock","Unlock the Terraform project"},
}

func DisplayCommands() {
	fmt.Println("Use the following commands to get started:")
	for _, command := range availableCommands {
		fmt.Printf("  %s: %s\n", command.Name, command.Description)
	}
}
