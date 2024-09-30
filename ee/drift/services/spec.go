package services

import (
	"fmt"
	utils2 "github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/ee/drift/utils"
	"github.com/diggerhq/digger/libs/spec"
	"os"
)

func GetRunNameFromJob(spec spec.Spec) (*string, error) {
	jobSpec := spec.Job
	diggerCommand := fmt.Sprintf("digger %v", jobSpec.JobType)
	jobIdShort := spec.JobId[:8]
	projectName := jobSpec.ProjectName
	//requestedBy := jobSpec.RequestedBy
	//prNumber := *jobSpec.PullRequestNumber

	runName := fmt.Sprintf("[%v] %v %v (driftapp)", jobIdShort, diggerCommand, projectName)
	return &runName, nil
}

func GetVCSToken(vcsType string, repoFullName string, repoOwner string, repoName string, installationId int64, gh utils2.GithubClientProvider) (*string, error) {
	var token string
	switch vcsType {
	case "github":
		_, ghToken, err := utils.GetGithubService(
			gh,
			installationId,
			repoFullName,
			repoOwner,
			repoName,
		)
		if err != nil {
			return nil, fmt.Errorf("TriggerWorkflow: could not retrieve token: %v", err)
		}
		token = *ghToken
	case "gitlab":
		token = os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")
	default:
		return nil, fmt.Errorf("unknown batch VCS: %v", vcsType)
	}

	return &token, nil
}
