package utils

import "fmt"

const version = "0.1.6"

// GetVersion returns the current version of the package
func GetVersion() string {
	verOutput := fmt.Sprintf("you are using digger version %s", version)
	return verOutput
}
