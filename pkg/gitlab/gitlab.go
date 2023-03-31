package gitlab

import (
	"digger/pkg/digger"
	"digger/pkg/utils"
	"fmt"
	"github.com/caarlos0/env/v7"
)

// based on https://docs.gitlab.com/ee/ci/variables/predefined_variables.html

type Context struct {
	PipelineSource   PipelineSourceType `env:"CI_PIPELINE_SOURCE"`
	PipelineId       *int               `env:"CI_PIPELINE_ID"`
	PipelineIId      *int               `env:"CI_PIPELINE_IID"`
	MergeRequestId   *int               `env:"CI_MERGE_REQUEST_ID"`
	MergeRequestIId  *int               `env:"CI_MERGE_REQUEST_IID"`
	ProjectName      string             `env:"CI_PROJECT_NAME"`
	ProjectNamespace string             `env:"CI_PROJECT_NAMESPACE"`
	Token            string             `env:"CI_JOB_TOKEN"`
}

type PipelineSourceType string

func (t PipelineSourceType) String() string {
	return string(t)
}

const (
	Push                     = "push"
	Web                      = "web"
	Schedule                 = "schedule"
	Api                      = "api"
	External                 = "external"
	Chat                     = "chat"
	WebIDE                   = "webide"
	ExternalPullRequestEvent = "external_pull_request_event"
	ParentPipeline           = "parent_pipeline"
	Trigger                  = "trigger"
	Pipeline                 = "pipeline"
)

func ParseGitLabContext() (*Context, error) {
	var parsedGitLabContext Context

	if err := env.Parse(&parsedGitLabContext); err != nil {
		fmt.Printf("%+v\n", err)
	}

	fmt.Printf("%+v\n", parsedGitLabContext)
	return &parsedGitLabContext, nil
}

func NewGitLabService(ghToken string, repoName string, owner string) CIService {
	client := GitLabClient("gitlab")
	return &GitLabService{
		Client:   client,
		RepoName: repoName,
		Owner:    owner,
	}
}

func ProcessGitLabEvent(gitlabEvent GitLabEvent, diggerConfig *digger.DiggerConfig, service CIService) ([]digger.Project, int, error) {
	var impactedProjects []digger.Project
	var prNumber int

	print("ProcessGitLabEvent")

	return impactedProjects, prNumber, nil
}

type GitLabClient string

type GitLabService struct {
	Client   GitLabClient
	RepoName string
	Owner    string
}

func (gitlabService GitLabService) GetChangedFiles(mergeRequest int) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (gitlabService GitLabService) PublishComment(mergeRequest int, comment string) {
	//TODO implement me
	panic("implement me")
}

type CIService interface {
	GetChangedFiles(prNumber int) ([]string, error)
	PublishComment(prNumber int, comment string)
}

type GitLabEvent struct {
	Name string
}

func ConvertGitLabEventToCommands(event GitLabEvent, impactedProjects []digger.Project) ([]digger.ProjectCommand, error) {
	//commandsPerProject := make([]digger.ProjectCommand, 0)

	return nil, nil
}

func RunCommandsPerProject(commandsPerProject []digger.ProjectCommand, projectNamespace string, projectName string, eventName string, prNumber int, diggerConfig *digger.DiggerConfig, service CIService, lock utils.Lock, workingDir string) error {

	return nil
}
