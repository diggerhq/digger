package policy

import (
	"fmt"
	"github.com/diggerhq/digger/ee/cli/pkg/utils"
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

// GetPolicy fetches policy for particular project,  if not found then it will fallback to org level policy
func (p *DiggerRepoPolicyProvider) GetAccessPolicy(organisation string, repo string, projectName string) (string, error) {
	var policycontents []byte
	err := utils.CloneGitRepoAndDoAction(p.ManagementRepoUrl, "main", p.GitToken, func(basePath string) error {
		orgAccesspath := path.Join(basePath, "policies", "access.rego")
		//repoAccesspath := path.Join(basePath, "policies", repo, "access.rego")
		//projectAccessPath := path.Join(basePath, "policies", repo, projectName, "access.rego")
		var regoPath string
		if _, err := os.Stat(orgAccesspath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Could not find org level path")
			} else {
				return err
			}
		} else {
			regoPath = orgAccesspath
		}

		var err error
		policycontents, err = os.ReadFile(regoPath)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return string(policycontents), nil
}

func (p *DiggerRepoPolicyProvider) GetPlanPolicy(organisation string, repo string, projectName string) (string, error) {
	return "", nil
}

func (p *DiggerRepoPolicyProvider) GetDriftPolicy() (string, error) {
	return "", nil

}

func (p *DiggerRepoPolicyProvider) GetOrganisation() string {
	return ""
}
