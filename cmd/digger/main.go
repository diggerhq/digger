package main

import (
	"digger/pkg/ci_runner"
	"digger/pkg/domain"
	"digger/pkg/lock"
	"digger/pkg/scm"
	"digger/pkg/service"
	"log"
)

func main() {
	// Retrieve config
	diggerConf, err := domain.NewDiggerConfig("")
	if err != nil {
		log.Fatalf("got an error while retrieving digger config: %v", err)
	}

	// Get CI event
	ciRunner := ci_runner.Current()
	event, err := ciRunner.CurrentEvent(diggerConf)
	if err != nil {
		log.Fatalf("got an error while retrieving the event: %v", err)
	}

	// Bootstrap the dependencies needed
	scmProvider, err := scm.GetProvider()
	if err != nil {
		log.Fatalf("got an error while getting the SCM provider: %v", err)
	}
	lockProvider, err := lock.GetProvider()
	if err != nil {
		log.Fatalf("got an error while getting the Lock provider: %v", err)
	}

	// Inject the dependencies
	prs := service.NewCmdProcessor().
		WithParsedEvent(event).
		WithSCMProvider(scmProvider).
		WithLockProvider(lockProvider)

	// Process the pull request event
	err = prs.Process()
	if err != nil {
		log.Fatalf("got an error while processing event: %v", err)
	}
}
