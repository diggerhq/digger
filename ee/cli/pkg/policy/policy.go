package policy

import (
	"fmt"
	"github.com/diggerhq/digger/ee/cli/pkg/utils"
	"log"
	"os"
	"path"
)

const DefaultAccessPolicy = `
package digger
default allow = true
allow = (count(input.planPolicyViolations) == 0)
`

type DiggerRepoPolicyProvider struct {
	ManagementRepoUrl string
	GitToken          string
}

func getContents(filePath string) (string, error) {
	if _, err := os.Stat(filePath); err != nil {
		return "", err
	}

	contents, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(contents), nil
}

func (p DiggerRepoPolicyProvider) getPolicyFileContents(repo string, projectName string, fileName string) (string, error) {
	var contents string
	err := utils.CloneGitRepoAndDoAction(p.ManagementRepoUrl, "main", p.GitToken, func(basePath string) error {
		orgAccesspath := path.Join(basePath, "policies", fileName)
		repoAccesspath := path.Join(basePath, "policies", repo, fileName)
		projectAccessPath := path.Join(basePath, "policies", repo, projectName, fileName)

		log.Printf("loading repo orgAccess %v repoAccess %v projectAcces %v", orgAccesspath, repoAccesspath, projectAccessPath)
		var err error
		contents, err = getContents(orgAccesspath)
		if os.IsNotExist(err) {
			contents, err = getContents(repoAccesspath)
			if os.IsNotExist(err) {
				contents, err = getContents(projectAccessPath)
				if os.IsNotExist(err) {
					return nil
				} else {
					fmt.Errorf("could not find any matching policy for %v,%v", repo, projectName)
				}
			} else {
				return err
			}
		} else {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return contents, nil
}

// GetPolicy fetches policy for particular project,  if not found then it will fallback to org level policy
func (p DiggerRepoPolicyProvider) GetAccessPolicy(organisation string, repo string, projectName string) (string, error) {
	return p.getPolicyFileContents(repo, projectName, "access.rego")
}

func (p DiggerRepoPolicyProvider) GetPlanPolicy(organisation string, repo string, projectName string) (string, error) {
	return "", nil
}

func (p DiggerRepoPolicyProvider) GetDriftPolicy() (string, error) {
	return "", nil

}

func (p DiggerRepoPolicyProvider) GetOrganisation() string {
	return ""
}
