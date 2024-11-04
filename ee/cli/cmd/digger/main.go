package main

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/libs/license"
	"log"
	"os"
)

/*
Exit codes:
0 - No errors
1 - Failed to read digger digger_config
2 - Failed to create lock provider
3 - Failed to find auth token
4 - Failed to initialise CI context
5 -
6 - failed to process CI event
7 - failed to convert event to command
8 - failed to execute command
10 - No CI detected
*/

func main() {
	err := license.LicenseKeyChecker{}.Check()
	if err != nil {
		log.Printf("error checking license %v", err)
		os.Exit(1)
	}
	if len(os.Args) == 1 {
		os.Args = append([]string{os.Args[0]}, "default")
	}
	if err := rootCmd.Execute(); err != nil {
		usage.ReportErrorAndExit("", fmt.Sprintf("Error occurred during command exec: %v", err), 8)
	}

}

func init() {
	log.SetOutput(os.Stdout)

	if os.Getenv("DEBUG") == "true" {
		log.SetFlags(log.Ltime | log.Lshortfile)
	}
}
