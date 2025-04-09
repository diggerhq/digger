package main

import (
	"fmt"
	"os"

	"github.com/go-substrate/strate/backend/cmd/st-server/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
