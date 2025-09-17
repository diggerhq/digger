package main

import (
    "fmt"
    "os"

    "github.com/diggerhq/digger/opentaco/cmd/taco/commands"
)

var (
    Version = "dev"
    Commit  = "unknown"
)

func main() {
    // Set version information in commands package
    commands.Version = Version
    commands.Commit = Commit
    
    if err := commands.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
