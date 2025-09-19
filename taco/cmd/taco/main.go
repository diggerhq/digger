package main

// Clean release test after fixing component names
// Testing Release-Please with proper tag recognition
// Should create version 0.1.2 for both components
// Full end-to-end test with cleanup and proper tagging
// Fixed tag collision with existing release workflows
// Should now have clean releases without dgctl contamination
// Testing multi-arch Docker builds and Helm-compatible tags
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
