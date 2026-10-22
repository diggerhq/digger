package policy

import (
	"sync"
	"time"

	"github.com/diggerhq/digger/libs/git_utils"
	"github.com/samber/lo"
	"log/slog"
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
	GitTimeout        time.Duration
}

const managementRepoBranch = "main"

// policyFileNames are the only files policy resolution ever reads, so the
// management repo checkout is restricted to them.
var policyFileNames = []string{"access.rego", "plan.rego", "apply.rego"}

// A job checks policies several times per project. The management repo is
// fetched once per process and the checkout reused for every lookup.
var managementRepoCheckouts = struct {
	sync.Mutex
	dirs map[string]string
}{dirs: map[string]string{}}

func (p DiggerRepoPolicyProvider) managementRepoDir() (string, error) {
	managementRepoCheckouts.Lock()
	defer managementRepoCheckouts.Unlock()

	key := p.ManagementRepoUrl + "@" + managementRepoBranch
	if dir, ok := managementRepoCheckouts.dirs[key]; ok {
		return dir, nil
	}

	start := time.Now()
	dir, err := git_utils.CloneGitRepoSparseWithTimeout(p.ManagementRepoUrl, managementRepoBranch, p.GitToken, "", p.gitTimeout(), policyFileNames)
	if err != nil {
		return "", err
	}
	// TODO(measurement): remove before release; kept to compare fetch times on real jobs.
	slog.Info("Management repo policies fetched", "repoUrl", p.ManagementRepoUrl, "duration", time.Since(start))
	managementRepoCheckouts.dirs[key] = dir
	return dir, nil
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
	basePath, err := p.managementRepoDir()
	if err != nil {
		return "", err
	}
	return resolvePolicyFile(basePath, repo, projectName, projectDir, fileName)
}

// resolvePolicyFile returns the contents of the first fileName found, starting
// with the project directory prefixes and ending with the well-known
// policies/ locations. Returns "" when no file exists at any location.
func resolvePolicyFile(basePath string, repo string, projectName string, projectDir string, fileName string) (string, error) {
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
		contents, err := getContents(pathPrefix)
		if err == nil {
			return contents, nil
		}
		if os.IsNotExist(err) {
			continue
		}
		return "", err
	}
	return "", nil
}

func (p DiggerRepoPolicyProvider) gitTimeout() time.Duration {
	if p.GitTimeout <= 0 {
		return 30 * time.Second
	}
	return p.GitTimeout
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

func (p DiggerRepoPolicyProvider) GetApplyPolicy(organisation string, repository string, projectname string, projectDir string) (string, error) {
	slog.Debug("GetApplyPolicy called",
		"organisation", organisation,
		"repository", repository,
		"projectname", projectname,
		"projectDir", projectDir,
		"managementRepoUrl", p.ManagementRepoUrl)
	policy, err := p.getPolicyFileContents(repository, projectname, projectDir, "apply.rego")
	if err != nil {
		slog.Error("Error getting apply policy file", "error", err)
		return policy, err
	}
	slog.Debug("GetApplyPolicy result",
		"policyLength", len(policy),
		"policyEmpty", policy == "",
		"policyContent", policy)
	return policy, nil
}

func (p DiggerRepoPolicyProvider) GetDriftPolicy() (string, error) {
	return "", nil

}

func (p DiggerRepoPolicyProvider) GetOrganisation() string {
	return ""
}
