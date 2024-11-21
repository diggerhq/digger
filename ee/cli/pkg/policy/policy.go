package policy

import (
	"github.com/diggerhq/digger/ee/cli/pkg/utils"
	"github.com/samber/lo"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
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

// GetPrefixesForPath
// @path is the total path example /dev/vpc/subnets
// @filename is the name of the file to search for example access.rego
// returns the list of prefixes in priority order example:
// /dev/vpc/subnets/access.rego
// /dev/vpc/access.rego
// /dev/access.rego
func GetPrefixesForPath(path string, fileName string) []string {
	var prefixes []string
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	for i := range parts {
		prefixes = append(prefixes, filepath.Join(parts[:i+1]...))
	}

	slices.Reverse(prefixes)
	prefixes = lo.FilterMap(prefixes, func(item string, index int) (string, bool) {
		// if input path was absolute then result should be absolute and ignore last item ""
		if parts[0] == "" {
			return string(filepath.Separator) + item + string(filepath.Separator) + fileName, index < len(prefixes)-1
		} else {
			return item + string(filepath.Separator) + fileName, index < len(prefixes)
		}
	})

	return prefixes
}

func (p DiggerRepoPolicyProvider) getPolicyFileContents(repo string, projectName string, projectDir string, fileName string) (string, error) {
	var contents string
	err := utils.CloneGitRepoAndDoAction(p.ManagementRepoUrl, "main", p.GitToken, func(basePath string) error {
		// we start with the project directory path prefixes as the highest priority
		prefixes := GetPrefixesForPath(path.Join(basePath, projectDir), fileName)

		// we also add a known location as a least priority item
		orgAccesspath := path.Join(basePath, "policies", fileName)
		repoAccesspath := path.Join(basePath, "policies", repo, fileName)
		projectAccessPath := path.Join(basePath, "policies", repo, projectName, fileName)
		prefixes = append(prefixes, projectAccessPath)
		prefixes = append(prefixes, repoAccesspath)
		prefixes = append(prefixes, orgAccesspath)

		for _, pathPrefix := range prefixes {
			var err error
			contents, err = getContents(pathPrefix)
			if err == nil {
				return nil
			}
			if os.IsNotExist(err) {
				continue
			} else {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return "", err
	}
	return contents, nil
}

// GetPolicy fetches policy for particular project,  if not found then it will fallback to org level policy
func (p DiggerRepoPolicyProvider) GetAccessPolicy(organisation string, repo string, projectName string, projectDir string) (string, error) {
	policy, err := p.getPolicyFileContents(repo, projectName, projectDir, "access.rego")
	if err != nil {
		return policy, err
	}
	if policy == "" {
		return DefaultAccessPolicy, nil
	}
	return policy, err
}

func (p DiggerRepoPolicyProvider) GetPlanPolicy(organisation string, repository string, projectname string, projectDir string) (string, error) {
	policy, err := p.getPolicyFileContents(repository, projectname, projectDir, "plan.rego")
	if err != nil {
		return policy, err
	}
	return policy, nil
}

func (p DiggerRepoPolicyProvider) GetDriftPolicy() (string, error) {
	return "", nil

}

func (p DiggerRepoPolicyProvider) GetOrganisation() string {
	return ""
}
