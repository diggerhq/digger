package service

import (
	"digger/pkg/domain"
	"errors"

	"golang.org/x/exp/slices"
)

var (
	ErrEventActionNotRecognized     = errors.New("action is not valid")
	ErrLockActionNotRecognized      = errors.New("lock action is not valid")
	ErrTerraformActionNotRecognized = errors.New("terraform action is not valid")
)

var (
	lockActions = []domain.Action{domain.Lock, domain.Unlock}
	tfActions   = []domain.Action{domain.Plan, domain.Apply}
)

type CmdProcessor struct {
	scmProvider  domain.SCMProvider
	lockProvider domain.LockProvider
	parsedEvent  *domain.ParsedEvent
}

func NewCmdProcessor() *CmdProcessor {
	return &CmdProcessor{}
}

func (prs *CmdProcessor) WithParsedEvent(pe *domain.ParsedEvent) *CmdProcessor {
	prs.parsedEvent = pe
	return prs
}

func (prs *CmdProcessor) WithSCMProvider(scm domain.SCMProvider) *CmdProcessor {
	prs.scmProvider = scm
	return prs
}

func (prs *CmdProcessor) WithLockProvider(lp domain.LockProvider) *CmdProcessor {
	prs.lockProvider = lp
	return prs
}

func (prs *CmdProcessor) Process() error {
	event := prs.parsedEvent

	for _, p := range event.ProjectsInScope {
		for _, a := range p.Actions {
			prs.processProjectAction(&p, a)
		}
	}

	return nil
}

func (prs *CmdProcessor) processProjectAction(project *domain.ProjectCommand, action domain.Action) error {
	event := prs.parsedEvent
	hasLock, _, err := prs.lockProvider.Get() // check project lock
	if err != nil {
		return err
	}

	if hasLock { // && not current PR
		comment := "Project is currently locked by PR #xx" //TODO: modify that sentence when ready
		return prs.scmProvider.PublishComment(comment, event.PRDetails)
	}

	// At this point we know there is no lock
	// Send to the right event processor based on the action
	if slices.Contains(lockActions, action) {
		return prs.processLockAction(project, action)
	}

	if slices.Contains(tfActions, action) {
		return prs.processTerraformAction(project, action)
	}

	return nil
}

func (prs *CmdProcessor) processLockAction(project *domain.ProjectCommand, action domain.Action) error {
	var err error

	switch action {
	case domain.Lock:
		err = prs.lockProvider.Lock()
	case domain.Unlock:
		err = prs.lockProvider.Unlock()
	}

	return err
}

func (prs *CmdProcessor) processTerraformAction(project *domain.ProjectCommand, action domain.Action) error {
	var output *domain.TerraformOutput
	var err error

	switch action {
	case domain.Plan:
		output, err = project.Runner.Plan(nil)
	case domain.Apply:
		output, err = project.Runner.Apply(nil)
	}

	if err != nil {
		return err
	}

	// Parse output & publish
	parsedOut := output.Stdout //TODO: parse output
	return prs.scmProvider.PublishComment(parsedOut, prs.parsedEvent.PRDetails)
}
