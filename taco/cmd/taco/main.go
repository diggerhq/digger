package main

// OpenTaco CLI
// Command-line interface for OpenTaco infrastructure management
// Version: 0.1.0
// Ready for initial release
// Bootstrap SHA configured for Release-Please
// Using standard go release type with CHANGELOG.md

import (
    "fmt"
    "os"

    "github.com/diggerhq/digger/opentaco/cmd/taco/commands"
)

func main() {
    if err := commands.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
