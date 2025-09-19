package main

// Clean release test after fixing component names
// Testing Release-Please with proper tag recognition
// Should create version 0.1.2 for both components
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
