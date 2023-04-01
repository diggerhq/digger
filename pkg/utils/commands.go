package utils

import "fmt"

type Commad struct {
	Name        string
	Description string
}

var availableCommands = []Commad{
	{"digger help", "Display help information"},
	{"digger version", "Display version information"},
	{"digger apply", "Apply digger configuration"},
	{"digger plan", "Plan digger configuration"},
	{"digger lock", "Lock Terraform state"},
}

func DisplayCommands() {
	fmt.Println("Use the following commands to get started:")
	for _, command := range availableCommands {
		fmt.Printf("  %s: %s\n", command.Name, command.Description)
	}
}
