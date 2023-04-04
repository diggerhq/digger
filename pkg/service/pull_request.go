package service

import (
	"digger/pkg/domain"
	"digger/pkg/tf_runner"
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

type PullRequest struct {
	scmProvider  domain.SCMProvider
	lockProvider domain.LockProvider
	parsedEvent  *domain.ParsedEvent
}

func NewPullRequest() *PullRequest {
	return &PullRequest{}
}

func (prs *PullRequest) WithParsedEvent(pe *domain.ParsedEvent) *PullRequest {
	prs.parsedEvent = pe
	return prs
}

func (prs *PullRequest) WithSCMProvider(scm domain.SCMProvider) *PullRequest {
	prs.scmProvider = scm
	return prs
}

func (prs *PullRequest) WithLockProvider(lp domain.LockProvider) *PullRequest {
	prs.lockProvider = lp
	return prs
}

func (prs *PullRequest) Process() error {
	event := prs.parsedEvent

	for _, p := range event.Projects {
		for _, a := range p.Actions {
			prs.processProjectAction(&p, a)
		}
	}

	return nil
}

func (prs *PullRequest) processProjectAction(project *domain.ProjectCommand, action domain.Action) error {
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

func (prs *PullRequest) processLockAction(project *domain.ProjectCommand, action domain.Action) error {
	var err error

	switch action {
	case domain.Lock:
		err = prs.lockProvider.Lock()
	case domain.Unlock:
		err = prs.lockProvider.Unlock()
	}

	return err
}

func (prs *PullRequest) processTerraformAction(project *domain.ProjectCommand, action domain.Action) error {
	var output *domain.TerraformOutput
	var err error
	tfRunner := loadTerraformRunner(project)

	tfRunner.SetWorkingDir(project.WorkingDir)

	switch action {
	case domain.Plan:
		output, err = tfRunner.Plan(nil)
	case domain.Apply:
		output, err = tfRunner.Apply(nil)
	}

	if err != nil {
		return err
	}

	// Parse output & publish
	parsedOut := output.Stdout //TODO: parse output
	return prs.scmProvider.PublishComment(parsedOut, prs.parsedEvent.PRDetails)
}

func loadTerraformRunner(project *domain.ProjectCommand) domain.TerraformRunner {
	if project.Runner == "terragrunt" {
		return &tf_runner.Terragrunt{}
	}

	return &tf_runner.Terraform{}
}
