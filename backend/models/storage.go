package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/dchest/uniuri"
	"github.com/diggerhq/digger/backend/queries"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func (db *Database) GetProjectsFromContext(c *gin.Context, orgIdKey string) ([]Project, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)

	slog.Info("getting projects from context", "organisationId", loggedInOrganisationId)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return nil, false
	}

	var projects []Project

	err := db.GormDB.Preload("Organisation").Preload("Repo").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", loggedInOrganisationId).Find(&projects).Error

	if err != nil {
		slog.Error("error fetching projects from database", "error", err)
		return nil, false
	}

	slog.Info("fetched projects from context", "count", len(projects))
	return projects, true
}

func (db *Database) GetProjectsRemainingInFreePLan(orgId uint) (uint, uint, uint, error) {

	var countOfMonitoredProjects int64
	err := db.GormDB.Model(&Project{}).Where("organisation_id = ? AND drift_enabled = ?", orgId, true).Count(&countOfMonitoredProjects).Error
	if err != nil {
		slog.Error("Error fetching project count", "error", err)
		return 0, 0, 0, err
	}
	remainingFreeProjects := uint(math.Max(0, float64(MaxFreePlanProjectsPerOrg)-float64(countOfMonitoredProjects)))
	billableProjectsCount := uint(math.Max(0, float64(countOfMonitoredProjects)-float64(MaxFreePlanProjectsPerOrg)))
	return uint(countOfMonitoredProjects), remainingFreeProjects, billableProjectsCount, nil
}

func (db *Database) GetReposFromContext(c *gin.Context, orgIdKey string) ([]Repo, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)

	slog.Info("getting repos from context", "organisationId", loggedInOrganisationId)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return nil, false
	}

	var repos []Repo

	err := db.GormDB.Preload("Organisation").
		Joins("INNER JOIN organisations ON repos.organisation_id = organisations.id").
		Where("repos.organisation_id = ?", loggedInOrganisationId).Find(&repos).Error

	if err != nil {
		slog.Error("error fetching repos from database", "error", err)
		return nil, false
	}

	slog.Info("fetched repos from context", "count", len(repos))
	return repos, true
}

func (db *Database) GetPoliciesFromContext(c *gin.Context, orgIdKey string) ([]Policy, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)

	slog.Info("getting policies from context", "organisationId", loggedInOrganisationId)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return nil, false
	}

	var policies []Policy

	err := db.GormDB.Preload("Organisation").Preload("Repo").Preload("Project").
		Joins("LEFT JOIN projects ON projects.id = policies.project_id").
		Joins("LEFT JOIN repos ON projects.repo_id = repos.id").
		Joins("LEFT JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", loggedInOrganisationId).Find(&policies).Error

	if err != nil {
		slog.Error("error fetching policies from database", "error", err)
		return nil, false
	}

	slog.Info("fetched policies from context", "count", len(policies))
	return policies, true
}

func (db *Database) GetProjectRunsForOrg(orgId int) ([]ProjectRun, error) {
	var runs []ProjectRun

	slog.Info("fetching project runs", "organisationId", orgId)

	err := db.GormDB.Preload("Project").Preload("Project.Organisation").Preload("Project.Repo").
		Joins("INNER JOIN projects ON projects.id = project_runs.project_id").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", orgId).Order("created_at desc").Limit(100).Find(&runs).Error

	if err != nil {
		slog.Error("error fetching project runs from database", "error", err, "organisationId", orgId)
		return nil, fmt.Errorf("unknown error occurred while fetching database, %v", err)
	}

	slog.Info("fetched project runs", "count", len(runs), "organisationId", orgId)
	return runs, nil
}

func (db *Database) GetProjectRunsFromContext(c *gin.Context, orgIdKey string) ([]ProjectRun, bool) {
	loggedInOrganisationId := c.GetUint(orgIdKey)

	slog.Info("getting project runs from context", "organisationId", loggedInOrganisationId)

	if loggedInOrganisationId == 0 {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return nil, false
	}

	runs, err := db.GetProjectRunsForOrg(int(loggedInOrganisationId))
	if err != nil {
		return nil, false
	}
	return runs, true
}

func (db *Database) GetProjectByRunId(c *gin.Context, runId uint, orgIdKey string) (*ProjectRun, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)
	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return nil, false
	}

	slog.Info("getting project by run id",
		"runId", runId,
		"organisationId", loggedInOrganisationId)

	var projectRun ProjectRun

	err := db.GormDB.Preload("Project").Preload("Project.Organisation").Preload("Project.Repo").
		Joins("INNER JOIN projects ON projects.id = project_runs.project_id").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", loggedInOrganisationId).
		Where("project_runs.id = ?", runId).First(&projectRun).Error

	if err != nil {
		slog.Error("error fetching project run from database",
			"error", err,
			"runId", runId,
			"organisationId", loggedInOrganisationId)
		return nil, false
	}

	return &projectRun, true
}

func (db *Database) GetProjectByProjectId(c *gin.Context, projectId uint, orgIdKey string) (*Project, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)
	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return nil, false
	}

	slog.Info("getting project by project id",
		"projectId", projectId,
		"organisationId", loggedInOrganisationId)

	var project Project

	err := db.GormDB.Preload("Organisation").Preload("Repo").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", loggedInOrganisationId).
		Where("projects.id = ?", projectId).First(&project).Error

	if err != nil {
		slog.Error("error fetching project from database",
			"error", err,
			"projectId", projectId,
			"organisationId", loggedInOrganisationId)
		return nil, false
	}

	return &project, true
}

func (db *Database) GetProject(projectId uint) (*Project, error) {
	slog.Info("getting project by id", "projectId", projectId)

	var project Project

	err := db.GormDB.Preload("Organisation").Preload("Repo").
		Where("id = ?", projectId).
		First(&project).Error

	if err != nil {
		slog.Error("error fetching project from database",
			"error", err,
			"projectId", projectId)
		return nil, err
	}

	return &project, nil
}

// GetProjectByName return project for specified org and repo
// if record doesn't exist return nil
func (db *Database) GetProjectByName(orgId any, repoFullName string, name string) (*Project, error) {
	slog.Info("getting project by name",
		slog.Group("project",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"name", name))

	var project Project

	err := db.GormDB.Preload("Organisation").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", orgId).
		Where("repo_full_name = ?", repoFullName).
		Where("projects.name = ?", name).First(&project).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Debug("project not found",
				"orgId", orgId,
				"repoFullName", repoFullName,
				"name", name)
			return nil, nil
		}
		slog.Error("error fetching project from database",
			"error", err,
			"orgId", orgId,
			"repoFullname", repoFullName,
			"name", name)
		return nil, err
	}

	return &project, nil
}

// GetProjectByRepo return projects for specified org and repo
func (db *Database) GetProjectByRepo(orgId any, repo *Repo) ([]Project, error) {
	slog.Info("getting projects by repo",
		slog.Group("context",
			"orgId", orgId,
			"repoId", repo.ID,
			"repoName", repo.Name))

	projects := make([]Project, 0)

	err := db.GormDB.Preload("Organisation").Preload("Repo").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", orgId).
		Where("repos.id = ?", repo.ID).Find(&projects).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Debug("no projects found for repo",
				"orgId", orgId,
				"repoId", repo.ID,
				"repoName", repo.Name)
			return nil, nil
		}
		slog.Error("error fetching projects by repo from database",
			"error", err,
			"orgId", orgId,
			"repoId", repo.ID,
			"repoName", repo.Name)
		return nil, err
	}

	slog.Info("found projects by repo",
		"count", len(projects),
		"orgId", orgId,
		"repoId", repo.ID)
	return projects, nil
}

func (db *Database) GetPolicyByPolicyId(c *gin.Context, policyId uint, orgIdKey string) (*Policy, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)
	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return nil, false
	}

	slog.Info("getting policy by policy id",
		"policyId", policyId,
		"organisationId", loggedInOrganisationId)

	var policy Policy

	err := db.GormDB.Preload("Project").Preload("Project.Organisation").Preload("Project.Repo").
		Joins("INNER JOIN projects ON projects.id = policies.project_id").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", loggedInOrganisationId).
		Where("policies.id = ?", policyId).First(&policy).Error

	if err != nil {
		slog.Error("error fetching policy from database",
			"error", err,
			"policyId", policyId,
			"organisationId", loggedInOrganisationId)
		return nil, false
	}

	return &policy, true
}

func (db *Database) GetDefaultRepo(c *gin.Context, orgIdKey string) (*Repo, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)
	if !exists {
		slog.Warn("not allowed to access this resource", "orgIdKey", orgIdKey)
		return nil, false
	}

	slog.Info("getting default repo", "organisationId", loggedInOrganisationId)
	var repo Repo

	err := db.GormDB.Preload("Organisation").
		Joins("INNER JOIN organisations ON repos.organisation_id = organisations.id").
		Where("organisations.id = ?", loggedInOrganisationId).First(&repo).Error

	if err != nil {
		slog.Error("error fetching default repo from database",
			"error", err,
			"organisationId", loggedInOrganisationId)
		return nil, false
	}

	return &repo, true
}

// GetRepo returns digger repo by organisationId and repo name (diggerhq-digger)
// it will return an empty object if record doesn't exist in database
func (db *Database) GetRepo(orgIdKey any, repoName string) (*Repo, error) {
	slog.Info("getting repo by name",
		"orgId", orgIdKey,
		"repoName", repoName)

	var repo Repo

	err := db.GormDB.Preload("Organisation").
		Joins("INNER JOIN organisations ON repos.organisation_id = organisations.id").
		Where("organisations.id = ? AND repos.name=?", orgIdKey, repoName).First(&repo).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Debug("repo not found",
				"orgId", orgIdKey,
				"repoName", repoName)
			return nil, fmt.Errorf("repo not found %v", repoName)
		}
		slog.Error("failed to find digger repo",
			"error", err,
			"orgId", orgIdKey,
			"repoName", repoName)
		return nil, err
	}

	slog.Debug("found repo",
		"repoId", repo.ID,
		"orgId", orgIdKey,
		"repoName", repoName)
	return &repo, nil
}

func (db *Database) GetRepoByFullName(orgIdKey any, repoFullName string) (*Repo, error) {
	slog.Info("getting repo by full name",
		"orgId", orgIdKey,
		"repoName", repoFullName)

	var repo Repo

	err := db.GormDB.Preload("Organisation").
		Joins("INNER JOIN organisations ON repos.organisation_id = organisations.id").
		Where("organisations.id = ? AND repos.repo_full_name=?", orgIdKey, repoFullName).First(&repo).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Debug("repo not found",
				"orgId", orgIdKey,
				"repoFullName", repoFullName)
			return nil, fmt.Errorf("repo not found %v", repoFullName)
		}
		slog.Error("failed to find digger repo",
			"error", err,
			"orgId", orgIdKey,
			"repoFullName", repoFullName)
		return nil, err
	}

	slog.Debug("found repo",
		"repoId", repo.ID,
		"orgId", orgIdKey,
		"repoName", repoFullName)
	return &repo, nil
}

// GetGithubAppInstallationByIdAndRepo repoFullName should be in the following format: org/repo_name, for example "diggerhq/github-job-scheduler"
func (db *Database) GetRepoByInstallationIdAndRepoFullName(installationId string, repoFullName string) (*Repo, error) {
	repo := Repo{}
	result := db.GormDB.Where("github_app_installation_id = ?  AND repo_full_name=?", installationId, repoFullName).Find(&repo)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}

	if repo.ID == 0 {
		return nil, nil
	}
	return &repo, nil
}

// GetRepoById returns digger repo by organisationId and repo name (diggerhq-digger)
func (db *Database) GetRepoById(orgIdKey any, repoId any) (*Repo, error) {
	var repo Repo

	err := db.GormDB.Preload("Organisation").
		Joins("INNER JOIN organisations ON repos.organisation_id = organisations.id").
		Where("organisations.id = ? AND repos.ID=?", orgIdKey, repoId).First(&repo).Error

	if err != nil {
		slog.Error("failed to find digger repo by id", "error", err, "orgId", orgIdKey, "repoId", repoId)
		return nil, err
	}
	return &repo, nil
}

// GithubRepoAdded handles github drift that github repo has been added to the app installation
func (db *Database) GithubRepoAdded(installationId int64, appId int64, login string, accountId int64, repoFullName string) (*GithubAppInstallation, error) {
	// check if item exist already
	item := &GithubAppInstallation{}
	result := db.GormDB.Where("github_installation_id = ? AND repo=? AND github_app_id=?", installationId, repoFullName, appId).First(item)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to find github installation in database. %v", result.Error)
		}
	}

	if result.RowsAffected == 0 {
		var err error
		item, err = db.CreateGithubAppInstallation(installationId, appId, login, int(accountId), repoFullName)
		if err != nil {
			return nil, fmt.Errorf("failed to save github installation item to database. %v", err)
		}
	} else {
		slog.Info("record for installation already exists",
			"installationId", installationId,
			"repo", repoFullName,
			"status", "active")
		item.Status = GithubAppInstallActive
		item.UpdatedAt = time.Now()
		err := db.GormDB.Save(item).Error
		if err != nil {
			return nil, fmt.Errorf("failed to update github installation in the database. %v", err)
		}
	}
	return item, nil
}

func (db *Database) GithubRepoRemoved(installationId int64, appId int64, repoFullName string) (*GithubAppInstallation, error) {
	item := &GithubAppInstallation{}
	err := db.GormDB.Where("github_installation_id = ? AND status=? AND github_app_id=? AND repo=?", installationId, GithubAppInstallActive, appId, repoFullName).First(item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("record not found for installation",
				"installationId", installationId,
				"appId", appId,
				"repo", repoFullName,
				"status", "active")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find github installation in database. %v", err)
	}
	item.Status = GithubAppInstallDeleted
	item.UpdatedAt = time.Now()
	err = db.GormDB.Save(item).Error
	if err != nil {
		return nil, fmt.Errorf("failed to update github installation in the database. %v", err)
	}
	return item, nil
}

func (db *Database) GetGithubAppInstallationByOrgAndRepo(orgId any, repo string, status GithubAppInstallStatus) (*GithubAppInstallation, error) {
	link, err := db.GetGithubInstallationLinkForOrg(orgId)
	if err != nil {
		return nil, err
	}

	installation := GithubAppInstallation{}
	result := db.GormDB.Where("github_installation_id = ? AND status=? AND repo=?", link.GithubInstallationId, status, repo).Find(&installation)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}

	// If not found, the values will be default values, which means ID will be 0
	if installation.ID == 0 {
		return nil, nil
	}
	return &installation, nil
}

// GetGithubAppInstallationByIdAndRepo repoFullName should be in the following format: org/repo_name, for example "diggerhq/github-job-scheduler"
func (db *Database) GetGithubAppInstallationByIdAndRepo(installationId int64, repoFullName string) (*GithubAppInstallation, error) {
	installation := GithubAppInstallation{}
	result := db.GormDB.Where("github_installation_id = ? AND status=? AND repo=?", installationId, GithubAppInstallActive, repoFullName).Find(&installation)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}

	// If not found, the values will be default values, which means ID will be 0
	if installation.Model.ID == 0 {
		return nil, fmt.Errorf("GithubAppInstallation with id=%v doesn't exist", installationId)
	}
	return &installation, nil
}

func (db *Database) GetGithubAppInstallations(installationId int64) ([]GithubAppInstallation, error) {
	var installations []GithubAppInstallation
	result := db.GormDB.Where("github_installation_id = ? AND status=?", installationId, GithubAppInstallActive).Find(&installations)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	return installations, nil
}

// GetGithubAppInstallationLink repoFullName should be in the following format: org/repo_name, for example "diggerhq/github-job-scheduler"
func (db *Database) GetGithubAppInstallationLink(installationId int64) (*GithubAppInstallationLink, error) {
	var link GithubAppInstallationLink
	result := db.GormDB.Preload("Organisation").Where("github_installation_id = ? AND status=?", installationId, GithubAppInstallationLinkActive).Find(&link)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}

	// If not found, the values will be default values, which means ID will be 0
	if link.Model.ID == 0 {
		return nil, nil
	}
	return &link, nil
}

func (db *Database) CreateVCSConnection(name string, vcsType DiggerVCSType, githubId int64, ClientID string, ClientSecretEncrypted string, WebhookSecretEncrypted string, PrivateKeyEncrypted string, PrivateKeyBase64Encrypted string, Org string, url string, bitbucketAccessTokenEnc string, bitbucketWebhookSecretEnc string, gitlabWebhookSecret string, gitlabAccessToken string, orgId uint) (*VCSConnection, error) {
	app := VCSConnection{
		Name:                            name,
		VCSType:                         vcsType,
		GithubId:                        githubId,
		ClientID:                        ClientID,
		ClientSecretEncrypted:           ClientSecretEncrypted,
		WebhookSecretEncrypted:          WebhookSecretEncrypted,
		PrivateKeyEncrypted:             PrivateKeyEncrypted,
		PrivateKeyBase64Encrypted:       PrivateKeyBase64Encrypted,
		Org:                             Org,
		GithubAppUrl:                    url,
		BitbucketWebhookSecretEncrypted: bitbucketWebhookSecretEnc,
		BitbucketAccessTokenEncrypted:   bitbucketAccessTokenEnc,
		GitlabWebhookSecretEncrypted:    gitlabWebhookSecret,
		GitlabAccessTokenEncrypted:      gitlabAccessToken,
		OrganisationID:                  orgId,
	}
	result := db.GormDB.Save(&app)
	if result.Error != nil {
		return nil, result.Error
	}
	slog.Info("VCS connection created successfully",
		"name", app.Name,
		"url", app.GithubAppUrl)
	return &app, nil
}

func (db *Database) GetVCSConnectionById(id string) (*VCSConnection, error) {
	app := VCSConnection{}
	result := db.GormDB.Where("id = ?", id).Find(&app)
	if result.Error != nil {
		slog.Error("failed to find VCS connection by id",
			"id", id,
			"error", result.Error)
		return nil, result.Error
	}
	return &app, nil
}

// GetGithubApp return GithubApp by Id
func (db *Database) GetVCSConnection(gitHubAppId any) (*VCSConnection, error) {
	app := VCSConnection{}
	result := db.GormDB.Where("github_id = ?", gitHubAppId).Find(&app)
	if result.Error != nil {
		slog.Error("failed to find VCS connection by GitHub app id",
			"githubAppId", gitHubAppId,
			"error", result.Error)
		return nil, result.Error
	}
	return &app, nil
}

func (db *Database) CreateGithubInstallationLink(org *Organisation, installationId int64) (*GithubAppInstallationLink, error) {
	l := GithubAppInstallationLink{}
	// check if there is already a link to another org, and throw an error in this case
	result := db.GormDB.Preload("Organisation").Where("github_installation_id = ? AND status=?", installationId, GithubAppInstallationLinkActive).Find(&l)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	if result.RowsAffected > 0 {
		if l.OrganisationId != org.ID {
			return nil, fmt.Errorf("GitHub app installation %v already linked to another org ", installationId)
		}
		slog.Info("installation already linked to organization",
			"installationId", installationId,
			"orgName", org.Name)
		// record already exist, do nothing
		return &l, nil
	}

	var list []GithubAppInstallationLink
	// if there are other installation for this org, we need to make them inactive
	result = db.GormDB.Preload("Organisation").Where("github_installation_id <> ? AND organisation_id = ? AND status=?", installationId, org.ID, GithubAppInstallationLinkActive).Find(&list)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	for _, item := range list {
		item.Status = GithubAppInstallationLinkInactive
		db.GormDB.Save(&item)
	}

	link := GithubAppInstallationLink{Organisation: org, GithubInstallationId: installationId, Status: GithubAppInstallationLinkActive}
	result = db.GormDB.Save(&link)
	if result.Error != nil {
		return nil, result.Error
	}
	slog.Info("GitHub app installation link created successfully",
		"orgName", org.Name,
		"installationId", installationId)
	return &link, nil
}

func (db *Database) GetGithubInstallationLinkForOrg(orgId any) (*GithubAppInstallationLink, error) {
	l := GithubAppInstallationLink{}
	result := db.GormDB.Where("organisation_id = ? AND status=?", orgId, GithubAppInstallationLinkActive).Find(&l)
	if result.Error != nil {
		return nil, result.Error
	}
	if l.ID == 0 {
		return nil, fmt.Errorf("GithubAppInstallationLink not found for orgId: %v\n", orgId)
	}
	return &l, nil
}

func (db *Database) GetGithubInstallationLinkForInstallationId(installationId any) (*GithubAppInstallationLink, error) {
	l := GithubAppInstallationLink{}
	result := db.GormDB.Where("github_installation_id = ? AND status=?", installationId, GithubAppInstallationLinkActive).Find(&l)
	if result.Error != nil {
		return nil, result.Error
	}
	return &l, nil
}

func (db *Database) MakeGithubAppInstallationLinkInactive(link *GithubAppInstallationLink) (*GithubAppInstallationLink, error) {
	link.Status = GithubAppInstallationLinkInactive
	result := db.GormDB.Save(link)
	if result.Error != nil {
		slog.Error("failed to update GitHub app installation link",
			"id", link.ID,
			"error", result.Error)
		return nil, result.Error
	}
	return link, nil
}

func (db *Database) CreateDiggerJobLink(diggerJobId string, repoFullName string) (*GithubDiggerJobLink, error) {
	link := GithubDiggerJobLink{Status: DiggerJobLinkCreated, DiggerJobId: diggerJobId, RepoFullName: repoFullName}
	result := db.GormDB.Save(&link)
	if result.Error != nil {
		slog.Error("failed to create digger job link",
			"diggerJobId", diggerJobId,
			"repoFullName", repoFullName,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Info("digger job link created successfully",
		"diggerJobId", diggerJobId,
		"repoFullName", repoFullName)
	return &link, nil
}

func (db *Database) GetDiggerJobLink(diggerJobId string) (*GithubDiggerJobLink, error) {
	link := GithubDiggerJobLink{}
	result := db.GormDB.Where("digger_job_id = ?", diggerJobId).Find(&link)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		slog.Error("failed to get digger job link",
			"diggerJobId", diggerJobId,
			"error", result.Error)
		return nil, result.Error
	}
	return &link, nil
}

func (db *Database) UpdateDiggerJobLink(diggerJobId string, repoFullName string, githubJobId int64) (*GithubDiggerJobLink, error) {
	jobLink := GithubDiggerJobLink{}
	// check if there is already a link to another org, and throw an error in this case
	result := db.GormDB.Where("digger_job_id = ? AND repo_full_name=? ", diggerJobId, repoFullName).Find(&jobLink)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("failed to update digger job link",
				"diggerJobId", diggerJobId,
				"repoFullName", repoFullName,
				"error", result.Error)
			return nil, result.Error
		}
	}
	if result.RowsAffected == 1 {
		jobLink.GithubJobId = githubJobId
		result = db.GormDB.Save(&jobLink)
		if result.Error != nil {
			return nil, result.Error
		}
		slog.Info("digger job link updated successfully",
			"diggerJobId", diggerJobId,
			"repoFullName", repoFullName,
			"githubJobId", githubJobId)
		return &jobLink, nil
	}
	return &jobLink, nil
}

func (db *Database) GetOrganisationById(orgId any) (*Organisation, error) {
	slog.Info("getting organisation by id",
		"orgId", orgId,
		"orgIdType", fmt.Sprintf("%T", orgId))

	org := Organisation{}
	err := db.GormDB.Where("id = ?", orgId).First(&org).Error
	if err != nil {
		return nil, fmt.Errorf("Error fetching organisation: %v", err)
	}
	return &org, nil
}

func (db *Database) GetDiggerBatch(batchId *uuid.UUID) (*DiggerBatch, error) {
	batch := &DiggerBatch{}
	result := db.GormDB.Where("id=? ", batchId).Find(batch)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	return batch, nil
}

func (db *Database) CreateDiggerBatch(vcsType DiggerVCSType, githubInstallationId int64, repoOwner string, repoName string, repoFullname string, PRNumber int, diggerConfig string, branchName string, batchType scheduler.DiggerCommand, commentId *int64, gitlabProjectId int, aiSummaryCommentId string, reportTerraformOutputs bool, coverAllImpactedProjects bool, VCSConnectionId *uint) (*DiggerBatch, error) {
	uid := uuid.New()
	batch := &DiggerBatch{
		ID:                       uid,
		VCS:                      vcsType,
		VCSConnectionId:          VCSConnectionId,
		GithubInstallationId:     githubInstallationId,
		RepoOwner:                repoOwner,
		RepoName:                 repoName,
		RepoFullName:             repoFullname,
		PrNumber:                 PRNumber,
		CommentId:                commentId,
		Status:                   scheduler.BatchJobCreated,
		BranchName:               branchName,
		DiggerConfig:             diggerConfig,
		BatchType:                batchType,
		GitlabProjectId:          gitlabProjectId,
		AiSummaryCommentId:       aiSummaryCommentId,
		ReportTerraformOutputs:   reportTerraformOutputs,
		CoverAllImpactedProjects: coverAllImpactedProjects,
	}
	result := db.GormDB.Save(batch)
	if result.Error != nil {
		return nil, result.Error
	}

	slog.Info("digger batch created successfully",
		"batchId", batch.ID,
		"repoFullName", repoFullname,
		"prNumber", PRNumber)
	return batch, nil
}

func (db *Database) UpdateDiggerBatch(batch *DiggerBatch) error {
	result := db.GormDB.Save(batch)
	if result.Error != nil {
		return result.Error
	}
	slog.Info("batch updated successfully", "batchId", batch.ID)
	return nil
}

func (db *Database) UpdateBatchStatus(batch *DiggerBatch) error {
	if batch.Status == scheduler.BatchJobInvalidated || batch.Status == scheduler.BatchJobFailed || batch.Status == scheduler.BatchJobSucceeded {
		return nil
	}

	batchId := batch.ID
	var diggerJobs []DiggerJob
	result := db.GormDB.Where("batch_id=?", batchId).Find(&diggerJobs)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("failed to get digger jobs by batch id",
				"batchId", batchId,
				"error", result.Error)
		}
		return result.Error
	}

	allJobsSucceeded := true
	for _, job := range diggerJobs {
		if job.Status != scheduler.DiggerJobSucceeded {
			allJobsSucceeded = false
		}
	}
	if allJobsSucceeded == true {
		batch.Status = scheduler.BatchJobSucceeded
		slog.Info("all jobs succeeded, marking batch as succeeded",
			"batchId", batchId,
			"jobCount", len(diggerJobs))
	}
	return nil
}

func (db *Database) CreateDiggerJob(batchId uuid.UUID, serializedJob []byte, workflowFile string) (*DiggerJob, error) {
	if serializedJob == nil || len(serializedJob) == 0 {
		return nil, fmt.Errorf("serializedJob can't be empty")
	}
	jobId := uniuri.New()
	batchIdStr := batchId.String()

	summary := &DiggerJobSummary{}
	result := db.GormDB.Save(summary)
	if result.Error != nil {
		return nil, result.Error
	}

	workflowUrl := "#"
	job := &DiggerJob{DiggerJobID: jobId, Status: scheduler.DiggerJobCreated,
		BatchID: &batchIdStr, SerializedJobSpec: serializedJob, DiggerJobSummary: *summary, WorkflowRunUrl: &workflowUrl, WorkflowFile: workflowFile}
	result = db.GormDB.Save(job)
	if result.Error != nil {
		return nil, result.Error
	}

	slog.Info("digger job created successfully",
		"diggerJobId", job.DiggerJobID,
		"id", job.ID,
		"batchId", batchIdStr,
		"workflowFile", workflowFile)
	return job, nil
}

func (db *Database) ListDiggerRunsForProject(projectName string, repoId uint) ([]DiggerRun, error) {
	var runs []DiggerRun

	err := db.GormDB.Preload("PlanStage").Preload("ApplyStage").
		Where("project_name = ? AND repo_id = ?", projectName, repoId).Order("created_at desc").Find(&runs).Error

	if err != nil {
		slog.Error("error fetching digger runs for project",
			"error", err,
			"projectName", projectName,
			"repoId", repoId)
		return nil, err
	}

	slog.Info("fetched digger runs for project",
		"count", len(runs),
		"projectName", projectName,
		"repoId", repoId)
	return runs, nil
}

func (db *Database) CreateDiggerRun(Triggertype string, PrNumber int, Status DiggerRunStatus, CommitId string, DiggerConfig string, GithubInstallationId int64, RepoId uint, ProjectName string, RunType RunType, planStageId *uint, applyStageId *uint) (*DiggerRun, error) {
	dr := &DiggerRun{
		Triggertype:          Triggertype,
		PrNumber:             &PrNumber,
		Status:               Status,
		CommitId:             CommitId,
		DiggerConfig:         DiggerConfig,
		GithubInstallationId: GithubInstallationId,
		RepoId:               RepoId,
		ProjectName:          ProjectName,
		RunType:              RunType,
		PlanStageId:          planStageId,
		ApplyStageId:         applyStageId,
		IsApproved:           false,
	}
	result := db.GormDB.Save(dr)
	if result.Error != nil {
		slog.Error("failed to create digger run",
			"runId", dr.ID,
			"error", result.Error,
			"projectName", ProjectName,
			"repoId", RepoId)
		return nil, result.Error
	}
	slog.Info("digger run created successfully",
		"runId", dr.ID,
		"projectName", ProjectName,
		"prNumber", PrNumber,
		"runType", RunType)
	return dr, nil
}

func (db *Database) CreateDiggerRunStage(batchId string) (*DiggerRunStage, error) {
	drs := &DiggerRunStage{
		BatchID: &batchId,
	}
	result := db.GormDB.Save(drs)
	if result.Error != nil {
		slog.Error("failed to create digger run stage",
			"stageId", drs.ID,
			"batchId", batchId,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Info("digger run stage created successfully",
		"stageId", drs.ID,
		"batchId", batchId)
	return drs, nil
}

func (db *Database) GetLastDiggerRunForProject(projectName string) (*DiggerRun, error) {
	diggerRun := &DiggerRun{}
	result := db.GormDB.Where("project_name = ? AND status <> ?", projectName, RunQueued).Order("created_at Desc").First(diggerRun)
	if result.Error != nil {
		slog.Error("error fetching last digger run for project",
			"projectName", projectName,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Debug("retrieved last digger run for project",
		"projectName", projectName,
		"runId", diggerRun.ID,
		"status", diggerRun.Status)
	return diggerRun, nil
}

func (db *Database) GetDiggerRun(id uint) (*DiggerRun, error) {
	dr := &DiggerRun{}
	result := db.GormDB.Preload("Repo").
		Preload("ApplyStage").
		Preload("PlanStage").
		Where("id=? ", id).Find(dr)
	if result.Error != nil {
		slog.Error("failed to get digger run",
			"runId", id,
			"error", result.Error)
		return nil, result.Error
	}
	return dr, nil
}

func (db *Database) CreateDiggerRunQueueItem(diggerRunId uint, projectId uint) (*DiggerRunQueueItem, error) {
	drq := &DiggerRunQueueItem{
		DiggerRunId: diggerRunId,
		ProjectId:   projectId,
	}
	result := db.GormDB.Save(drq)
	if result.Error != nil {
		slog.Error("failed to create digger run queue item",
			"queueItemId", drq.ID,
			"diggerRunId", diggerRunId,
			"projectId", projectId,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Info("digger run queue item created successfully",
		"queueItemId", drq.ID,
		"diggerRunId", diggerRunId,
		"projectId", projectId)
	return drq, nil
}

func (db *Database) GetDiggerRunQueueItem(id uint) (*DiggerRunQueueItem, error) {
	dr := &DiggerRunQueueItem{}
	result := db.GormDB.Preload("DiggerRun").Where("id=? ", id).Find(dr)
	if result.Error != nil {
		slog.Error("failed to get digger run queue item",
			"queueItemId", id,
			"error", result.Error)
		return nil, result.Error
	}
	return dr, nil
}

func (db *Database) GetDiggerJobFromRunStage(stage DiggerRunStage) (*DiggerJob, error) {
	job := &DiggerJob{}
	result := db.GormDB.Preload("Batch").Take(job, "batch_id = ?", stage.BatchID)
	if result.Error != nil {
		slog.Error("failed to get digger job from run stage",
			"stageId", stage.ID,
			"batchId", stage.BatchID,
			"error", result.Error)
		return nil, result.Error
	}
	return job, nil
}

func (db *Database) UpdateDiggerRun(diggerRun *DiggerRun) error {
	result := db.GormDB.Save(diggerRun)
	if result.Error != nil {
		slog.Error("failed to update digger run",
			"runId", diggerRun.ID,
			"error", result.Error)
		return result.Error
	}
	slog.Info("digger run updated successfully",
		"runId", diggerRun.ID,
		"status", diggerRun.Status)
	return nil
}

func (db *Database) DequeueRunItem(queueItem *DiggerRunQueueItem) error {
	slog.Info("dequeuing digger run queue item", "queueItemId", queueItem.ID)

	result := db.GormDB.Delete(queueItem)
	if result.Error != nil {
		slog.Error("failed to delete digger run queue item",
			"queueItemId", queueItem.ID,
			"error", result.Error)
		return result.Error
	}
	slog.Info("digger run queue item deleted successfully", "queueItemId", queueItem.ID)
	return nil
}

func (db *Database) GetFirstRunQueueForEveryProject() ([]DiggerRunQueueItem, error) {
	var runqueues []DiggerRunQueueItem
	query := `WITH RankedRuns AS (
  SELECT
    digger_run_queue_items.digger_run_id,
    digger_run_queue_items.project_id,
    digger_run_queue_items.created_at,
    ROW_NUMBER() OVER (PARTITION BY digger_run_queue_items.project_id ORDER BY digger_run_queue_items.created_at  ASC) AS QueuePosition
  FROM
    digger_run_queue_items
)
SELECT
  RankedRuns.digger_run_id ,
  RankedRuns.project_id ,
  RankedRuns.created_at
FROM
  RankedRuns
WHERE
  QueuePosition = 1`

	// 1. Fetch the front of the queue for every projectID
	tx := db.GormDB.
		Raw(query).
		Find(&runqueues)

	if tx.Error != nil {
		slog.Error("error fetching front of queue for projects", "error", tx.Error)
		return nil, tx.Error
	}

	// 2. Preload Project and DiggerRun for every DiggerrunQueue item (front of queue)
	var runqueuesWithData []DiggerRunQueueItem
	diggerRunIds := lo.Map(runqueues, func(run DiggerRunQueueItem, index int) uint {
		return run.DiggerRunId
	})

	slog.Debug("fetched front of queue items", "count", len(runqueues))

	tx = db.GormDB.Preload("DiggerRun").Preload("DiggerRun.Repo").
		Preload("DiggerRun.PlanStage").Preload("DiggerRun.ApplyStage").
		Preload("DiggerRun.PlanStage.Batch").Preload("DiggerRun.ApplyStage.Batch").
		Where("digger_run_queue_items.digger_run_id in ?", diggerRunIds).Find(&runqueuesWithData)

	if tx.Error != nil {
		slog.Error("error preloading data for queue items", "error", tx.Error)
		return nil, tx.Error
	}

	slog.Info("fetched queue items with preloaded data",
		"count", len(runqueuesWithData),
		"projectCount", len(diggerRunIds))
	return runqueuesWithData, nil
}

func (db *Database) UpdateDiggerJobSummary(diggerJobId string, resourcesCreated uint, resourcesUpdated uint, resourcesDeleted uint) (*DiggerJob, error) {
	diggerJob, err := db.GetDiggerJob(diggerJobId)
	if err != nil {
		slog.Error("could not get digger job for summary update",
			"diggerJobId", diggerJobId,
			"error", err)
		return nil, fmt.Errorf("Could not get digger job")
	}

	var jobSummary *DiggerJobSummary
	jobSummary = &diggerJob.DiggerJobSummary
	jobSummary.ResourcesCreated = resourcesCreated
	jobSummary.ResourcesUpdated = resourcesUpdated
	jobSummary.ResourcesDeleted = resourcesDeleted

	result := db.GormDB.Save(&jobSummary)
	if result.Error != nil {
		slog.Error("failed to update digger job summary",
			"diggerJobId", diggerJobId,
			"error", result.Error)
		return nil, result.Error
	}

	slog.Info("digger job summary updated successfully",
		"diggerJobId", diggerJobId,
		slog.Group("resources",
			"created", resourcesCreated,
			"updated", resourcesUpdated,
			"deleted", resourcesDeleted))
	return diggerJob, nil
}

func (db *Database) UpdateDiggerJob(job *DiggerJob) error {
	result := db.GormDB.Save(job)
	if result.Error != nil {
		slog.Error("failed to update digger job",
			"diggerJobId", job.DiggerJobID,
			"id", job.ID,
			"error", result.Error)
		return result.Error
	}
	slog.Info("digger job updated successfully",
		"diggerJobId", job.DiggerJobID,
		"id", job.ID,
		"status", job.Status)
	return nil
}

func (db *Database) GetDiggerJobsForBatch(batchId uuid.UUID) ([]DiggerJob, error) {
	jobs := make([]DiggerJob, 0)

	var where *gorm.DB
	where = db.GormDB.Where("digger_jobs.batch_id = ?", batchId)

	result := where.Preload("Batch").Preload("DiggerJobSummary").Find(&jobs)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("error fetching digger jobs for batch",
				"batchId", batchId,
				"error", result.Error)
			return nil, result.Error
		}
	}

	slog.Debug("fetched digger jobs for batch",
		"batchId", batchId,
		"jobCount", len(jobs))
	return jobs, nil
}

func (db *Database) GetJobsByRepoName(orgId uint, repoFullName string) ([]queries.JobQueryResult, error) {
	var results []queries.JobQueryResult

	query := `
		SELECT
			j.id, j.created_at, j.updated_at, j.deleted_at,
			j.digger_job_id, j.status, j.workflow_run_url,
			j.workflow_file, j.terraform_output, db.pr_number, db.repo_full_name, db.branch_name
		FROM digger_jobs j, digger_batches db, organisations o, github_app_installation_links l
		WHERE o.id = l.organisation_id
			AND l.github_installation_id = db.github_installation_id
			AND db.id = j.batch_id
		  	AND o.id = ?
			AND db.repo_full_name = ?
		ORDER BY j.created_at
	`

	slog.Info("fetching jobs by repo name",
		"orgId", orgId,
		"repoFullName", repoFullName)

	err := db.GormDB.Raw(query, orgId, repoFullName).Scan(&results).Error
	if err != nil {
		slog.Error("error fetching jobs by repo name",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"error", err)
	} else {
		slog.Debug("fetched jobs by repo name",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"jobCount", len(results))
	}

	return results, err
}

func (db *Database) GetDiggerJobsForBatchWithStatus(batchId uuid.UUID, status []scheduler.DiggerJobStatus) ([]DiggerJob, error) {
	jobs := make([]DiggerJob, 0)

	var where *gorm.DB
	where = db.GormDB.Where("digger_jobs.batch_id = ?", batchId).Where("status IN ?", status)

	result := where.Preload("Batch").Preload("DiggerJobSummary").Find(&jobs)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("error fetching digger jobs for batch with status",
				"batchId", batchId,
				"status", status,
				"error", result.Error)
			return nil, result.Error
		}
	}

	slog.Debug("fetched digger jobs for batch with status",
		"batchId", batchId,
		"status", status,
		"jobCount", len(jobs))
	return jobs, nil
}

func (db *Database) GetDiggerJobsWithStatus(status scheduler.DiggerJobStatus) ([]DiggerJob, error) {
	jobs := make([]DiggerJob, 0)

	var where *gorm.DB
	where = db.GormDB.Where("status = ?", status)

	result := where.Preload("Batch").Find(&jobs)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("error fetching digger jobs with status",
				"status", status,
				"error", result.Error)
			return nil, result.Error
		}
	}

	slog.Debug("fetched digger jobs with status",
		"status", status,
		"jobCount", len(jobs))
	return jobs, nil
}

func (db *Database) GetPendingParentDiggerJobs(batchId *uuid.UUID) ([]DiggerJob, error) {
	jobs := make([]DiggerJob, 0)

	joins := db.GormDB.Joins("LEFT JOIN digger_job_parent_links ON digger_jobs.digger_job_id = digger_job_parent_links.digger_job_id").Preload("Batch")

	var where *gorm.DB
	if batchId != nil {
		where = joins.Where("digger_jobs.status = ? AND digger_job_parent_links.id IS NULL AND digger_jobs.batch_id = ?", scheduler.DiggerJobCreated, *batchId)
	} else {
		where = joins.Where("digger_jobs.status = ? AND digger_job_parent_links.id IS NULL", scheduler.DiggerJobCreated)
	}

	result := where.Find(&jobs)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("error fetching pending parent digger jobs",
				"batchId", batchId,
				"error", result.Error)
			return nil, result.Error
		}
	}

	slog.Debug("fetched pending parent digger jobs",
		"batchId", batchId,
		"jobCount", len(jobs))
	return jobs, nil
}

func (db *Database) GetDiggerJob(jobId string) (*DiggerJob, error) {
	job := &DiggerJob{}
	result := db.GormDB.Preload("Batch").Preload("DiggerJobSummary").Where("digger_job_id=? ", jobId).Find(job)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("error fetching digger job",
				"jobId", jobId,
				"error", result.Error)
			return nil, result.Error
		}
	}
	return job, nil
}

func (db *Database) GetDiggerJobParentLinksByParentId(parentId *string) ([]DiggerJobParentLink, error) {
	var jobParentLinks []DiggerJobParentLink
	result := db.GormDB.Where("parent_digger_job_id=?", parentId).Find(&jobParentLinks)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("failed to get digger job links by parent job id",
				"parentJobId", parentId,
				"error", result.Error)
			return nil, result.Error
		}
	}

	slog.Debug("fetched digger job parent links",
		"parentJobId", parentId,
		"linkCount", len(jobParentLinks))
	return jobParentLinks, nil
}

func (db *Database) CreateDiggerJobParentLink(parentJobId string, jobId string) error {
	jobParentLink := DiggerJobParentLink{ParentDiggerJobId: parentJobId, DiggerJobId: jobId}
	result := db.GormDB.Create(&jobParentLink)
	if result.Error != nil {
		slog.Error("failed to create digger job parent link",
			"parentJobId", parentJobId,
			"childJobId", jobId,
			"error", result.Error)
		return result.Error
	}

	slog.Info("created digger job parent link",
		"parentJobId", parentJobId,
		"childJobId", jobId)
	return nil
}

func (db *Database) GetDiggerJobParentLinksChildId(childId *string) ([]DiggerJobParentLink, error) {
	var jobParentLinks []DiggerJobParentLink
	result := db.GormDB.Where("digger_job_id=?", childId).Find(&jobParentLinks)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("failed to get digger job links by child job id",
				"childJobId", childId,
				"error", result.Error)
			return nil, result.Error
		}
	}

	slog.Debug("fetched digger job parent links for child",
		"childJobId", childId,
		"linkCount", len(jobParentLinks))
	return jobParentLinks, nil
}

func (db *Database) GetOrganisation(tenantId any) (*Organisation, error) {
	org := &Organisation{}
	result := db.GormDB.Take(org, "external_id = ?", tenantId)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Debug("organisation not found", "tenantId", tenantId)
			return nil, nil
		} else {
			slog.Error("error fetching organisation",
				"tenantId", tenantId,
				"error", result.Error)
			return nil, result.Error
		}
	}

	slog.Debug("fetched organisation", "tenantId", tenantId, "orgId", org.ID)
	return org, nil
}

func (db *Database) CreateUser(email string, externalSource string, externalId string, orgId uint, username string) (*User, error) {
	user := &User{
		Email:          email,
		ExternalId:     externalId,
		ExternalSource: externalSource,
		OrganisationId: &orgId,
		Username:       username,
	}
	result := db.GormDB.Save(user)
	if result.Error != nil {
		slog.Error("failed to create user",
			"externalId", externalId,
			"email", email,
			"orgId", orgId,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Info("user created successfully",
		"userId", user.ID,
		"externalId", externalId,
		"email", email,
		"orgId", orgId)
	return user, nil
}

func (db *Database) CreateOrganisation(name string, externalSource string, tenantId string) (*Organisation, error) {
	org := &Organisation{Name: name, ExternalSource: externalSource, ExternalId: tenantId}
	result := db.GormDB.Save(org)
	if result.Error != nil {
		slog.Error("failed to create organisation",
			"name", name,
			"externalSource", externalSource,
			"tenantId", tenantId,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Info("organisation created successfully",
		"name", name,
		"orgId", org.ID,
		"externalSource", externalSource,
		"tenantId", tenantId)
	return org, nil
}

func (db *Database) CreateProject(name string, directory string, org *Organisation, repoFullName string, isGenerated bool, isInMainBranch bool) (*Project, error) {
	project := &Project{
		Name:           name,
		Directory:      directory,
		Organisation:   org,
		RepoFullName:   repoFullName,
		Status:         ProjectActive,
		IsGenerated:    isGenerated,
		IsInMainBranch: isInMainBranch,
	}
	result := db.GormDB.Save(project)
	if result.Error != nil {
		slog.Error("failed to create project",
			"name", name,
			"orgId", org.ID,
			"repoFullName", repoFullName,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Info("project created successfully",
		slog.Group("project",
			"id", project.ID,
			"name", name,
			"orgId", org.ID,
			"repoFullName", repoFullName,
			"isGenerated", isGenerated,
			"isInMainBranch", isInMainBranch))
	return project, nil
}

func (db *Database) UpdateProject(project *Project) error {
	result := db.GormDB.Save(project)
	if result.Error != nil {
		slog.Error("failed to update project",
			"projectId", project.ID,
			"error", result.Error)
		return result.Error
	}
	slog.Info("project updated successfully",
		"projectId", project.ID,
		"name", project.Name,
		"status", project.Status)
	return nil
}

func (db *Database) CreateRepo(name string, repoFullName string, repoOrganisation string, repoName string, repoUrl string, org *Organisation, diggerConfig string, installationId int64, githubAppId int64, defaultBranch string, cloneUrl string) (*Repo, error) {
	var repo Repo
	// check if repo exist already, do nothing in this case
	result := db.GormDB.Where("name = ? AND organisation_id=?", name, org.ID).Find(&repo)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Error("error checking for existing repo",
				"name", name,
				"orgId", org.ID,
				"error", result.Error)
			return nil, result.Error
		}
	}
	if result.RowsAffected > 0 {
		// record already exist, do nothing
		slog.Info("repo already exists, skipping creation",
			"name", name,
			"repoId", repo.ID,
			"orgId", org.ID)
		return &repo, nil
	}

	repo = Repo{
		Name:                    name,
		Organisation:            org,
		DiggerConfig:            diggerConfig,
		RepoFullName:            repoFullName,
		RepoOrganisation:        repoOrganisation,
		RepoName:                repoName,
		RepoUrl:                 repoUrl,
		GithubAppInstallationId: installationId,
		GithubAppId:             githubAppId,
		DefaultBranch:           defaultBranch,
		CloneUrl:                cloneUrl,
	}
	result = db.GormDB.Save(&repo)
	if result.Error != nil {
		slog.Error("failed to create repo",
			"name", name,
			"repoFullName", repoFullName,
			"orgId", org.ID,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Info("repo created successfully",
		slog.Group("repo",
			"id", repo.ID,
			"name", name,
			"repoFullName", repoFullName,
			"orgId", org.ID,
			"repoUrl", repoUrl))
	return &repo, nil
}

func (db *Database) GetToken(tenantId any) (*Token, error) {
	token := &Token{}
	result := db.GormDB.Take(token, "value = ?", tenantId)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Debug("token not found", "tenantId", tenantId)
			return nil, nil
		} else {
			slog.Error("error fetching token",
				"tenantId", tenantId,
				"error", result.Error)
			return nil, result.Error
		}
	}
	slog.Debug("token found", "tenantId", tenantId)
	return token, nil
}

func (db *Database) CreateDiggerJobToken(organisationId uint) (*JobToken, error) {
	// create a digger job token
	// prefixing token to make easier to retire this type of tokens later
	token := "cli:" + uuid.New().String()
	jobToken := &JobToken{
		Value:          token,
		OrganisationID: organisationId,
		Type:           CliJobAccessType,
		Expiry:         time.Now().Add(time.Hour * 2), // some jobs can take >30 mins (k8s cluster)
	}

	err := db.GormDB.Create(jobToken).Error
	if err != nil {
		slog.Error("failed to create job token",
			"organisationId", organisationId,
			"error", err)
		return nil, err
	}

	slog.Info("job token created successfully",
		"tokenId", jobToken.ID,
		"organisationId", organisationId,
		"expiry", jobToken.Expiry.Format(time.RFC3339))
	return jobToken, nil
}

func (db *Database) GetJobToken(tenantId any) (*JobToken, error) {
	token := &JobToken{}
	result := db.GormDB.Take(token, "value = ?", tenantId)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Debug("job token not found", "tenantId", tenantId)
			return nil, nil
		} else {
			slog.Error("error fetching job token",
				"tenantId", tenantId,
				"error", result.Error)
			return nil, result.Error
		}
	}

	slog.Debug("job token found",
		"tokenId", token.ID,
		"organisationId", token.OrganisationID,
		"type", token.Type)
	return token, nil
}

func (db *Database) DeleteJobTokenArtefacts(jobTokenId uint) error {
	artefact := JobArtefact{}
	result := db.GormDB.Where("job_token_id = ?", jobTokenId).Delete(&artefact)
	if result.Error != nil {
		slog.Error("failed to delete job token artefacts",
			"jobTokenId", jobTokenId,
			"error", result.Error)
		return result.Error
	}

	slog.Info("job token artefacts deleted successfully",
		"jobTokenId", jobTokenId,
		"rowsAffected", result.RowsAffected)
	return nil
}

func (db *Database) GetJobArtefact(jobTokenId uint) (*JobArtefact, error) {
	var artefact JobArtefact
	if err := DB.GormDB.Where("job_token_id = ?", jobTokenId).First(&artefact).Error; err != nil {
		slog.Error("failed to get job artefact",
			"jobTokenId", jobTokenId,
			"error", err)
		return nil, err
	}

	slog.Debug("job artefact found",
		"jobTokenId", jobTokenId,
		"artefactId", artefact.ID)
	return &artefact, nil
}

func (db *Database) CreateGithubAppInstallation(installationId int64, githubAppId int64, login string, accountId int, repoFullName string) (*GithubAppInstallation, error) {
	installation := &GithubAppInstallation{
		GithubInstallationId: installationId,
		GithubAppId:          githubAppId,
		Login:                login,
		AccountId:            accountId,
		Repo:                 repoFullName,
		Status:               GithubAppInstallActive,
	}
	result := db.GormDB.Save(installation)
	if result.Error != nil {
		slog.Error("failed to create GitHub app installation",
			"installationId", installationId,
			"githubAppId", githubAppId,
			"repoFullName", repoFullName,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Info("GitHub app installation created successfully",
		slog.Group("github",
			"installationId", installationId,
			"githubAppId", githubAppId,
			"login", login,
			"accountId", accountId,
			"repoFullName", repoFullName))
	return installation, nil
}

func validateDiggerConfigYaml(configYaml string) (*configuration.DiggerConfig, error) {
	diggerConfig, _, _, err := configuration.LoadDiggerConfigFromString(configYaml, "./")
	if err != nil {
		slog.Error("failed to validate digger config YAML", "error", err)
		return nil, fmt.Errorf("validation error, %w", err)
	}
	slog.Debug("digger config YAML validated successfully")
	return diggerConfig, nil
}

func (db *Database) RefreshProjectsFromRepo(orgId string, config configuration.DiggerConfigYaml, repoFullName string) error {
	slog.Debug("UpdateRepoDiggerConfig, repo", "repoFullName", repoFullName)

	org, err := db.GetOrganisationById(orgId)
	if err != nil {
		return fmt.Errorf("error retrieving org by name: %v", err)
	}

	err = db.GormDB.Transaction(func(tx *gorm.DB) error {
		for _, dc := range config.Projects {
			slog.Debug("refreshing for project", "name", dc.Name, "dir", dc.Dir)
			projectName := dc.Name
			projectDirectory := dc.Dir
			p, err := db.GetProjectByName(orgId, repoFullName, projectName)
			if err != nil {
				return fmt.Errorf("error retrieving project by name: %v", err)
			}
			if p == nil {
				_, err := db.CreateProject(projectName, projectDirectory, org, repoFullName, false, true)
				if err != nil {
					return fmt.Errorf("could not create project: %v", err)
				}
			} else {
				p.Directory = projectDirectory
				db.GormDB.Save(p)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error while updating projects from config: %v", err)
	}
	return nil
}

func (db *Database) CreateDiggerLock(resource string, lockId int, orgId uint) (*DiggerLock, error) {
	lock := &DiggerLock{
		Resource:       resource,
		LockId:         lockId,
		OrganisationID: orgId,
	}
	result := db.GormDB.Save(lock)
	if result.Error != nil {
		slog.Error("failed to create digger lock",
			"resource", resource,
			"lockId", lockId,
			"orgId", orgId,
			"error", result.Error)
		return nil, result.Error
	}

	slog.Info("digger lock created successfully",
		"lockId", lock.LockId,
		"resource", lock.Resource,
		"orgId", orgId)
	return lock, nil
}

func (db *Database) GetDiggerLock(resource string) (*DiggerLock, error) {
	lock := &DiggerLock{}
	result := db.GormDB.Where("resource=? ", resource).First(lock)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			slog.Debug("no lock found for resource", "resource", resource)
		} else {
			slog.Error("error fetching digger lock",
				"resource", resource,
				"error", result.Error)
		}
		return nil, result.Error
	}

	slog.Debug("found lock for resource",
		"resource", resource,
		"lockId", lock.LockId,
		"orgId", lock.OrganisationID)
	return lock, nil
}

func (db *Database) UpsertRepoCache(orgId uint, repoFullName string, diggerYmlStr string, diggerConfig configuration.DiggerConfig) (*RepoCache, error) {
	var repoCache RepoCache

	configMarshalled, err := json.Marshal(diggerConfig)
	if err != nil {
		slog.Error("could not marshal digger config",
			"repoFullName", repoFullName,
			"orgId", orgId,
			"error", err)
		return nil, fmt.Errorf("could not marshal config: %v", err)
	}

	// check if repo exist already, do nothing in this case
	result := db.GormDB.Where("org_id = ? AND repo_full_name=?", orgId, repoFullName).Find(&repoCache)
	if result.Error != nil {
		slog.Error("error checking for existing repo cache",
			"repoFullName", repoFullName,
			"orgId", orgId,
			"error", result.Error)
		return nil, result.Error
	}

	if result.RowsAffected > 0 {
		// record already exist, do nothing
		slog.Info("updating existing repo cache",
			"repoFullName", repoFullName,
			"orgId", orgId,
			"repoCacheId", repoCache.ID)

		repoCache.DiggerConfig = configMarshalled
		repoCache.DiggerYmlStr = diggerYmlStr
		result = db.GormDB.Save(&repoCache)
	} else {
		// create record here
		slog.Info("creating new repo cache",
			"repoFullName", repoFullName,
			"orgId", orgId)

		repoCache = RepoCache{
			OrgId:        orgId,
			RepoFullName: repoFullName,
			DiggerYmlStr: diggerYmlStr,
			DiggerConfig: configMarshalled,
		}
		result = db.GormDB.Save(&repoCache)
		if result.Error != nil {
			slog.Error("failed to create repo cache",
				"repoFullName", repoFullName,
				"orgId", orgId,
				"error", result.Error)
			return nil, result.Error
		}
	}

	slog.Info("repo cache operation completed successfully",
		"repoFullName", repoFullName,
		"orgId", orgId,
		"repoCacheId", repoCache.ID,
		"configSize", len(configMarshalled))
	return &repoCache, nil
}

func (db *Database) GetRepoCache(orgId uint, repoFullName string) (*RepoCache, error) {
	var repoCache RepoCache

	slog.Debug("fetching repo cache",
		"repoFullName", repoFullName,
		"orgId", orgId)

	err := db.GormDB.
		Where("org_id = ? AND repo_full_name = ?", orgId, repoFullName).First(&repoCache).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Debug("repo cache not found",
				"repoFullName", repoFullName,
				"orgId", orgId)
			return nil, fmt.Errorf("repo cache not found %v", err)
		}
		slog.Error("failed to find repo cache",
			"repoFullName", repoFullName,
			"orgId", orgId,
			"error", err)
		return nil, err
	}

	slog.Debug("repo cache found",
		"repoFullName", repoFullName,
		"orgId", orgId,
		"repoCacheId", repoCache.ID,
		"configSize", len(repoCache.DiggerConfig))
	return &repoCache, nil
}

func (db *Database) GetDiggerBatchesForPR(repoFullName string, prNumber int) ([]DiggerBatch, error) {
	// Step 1: Get all batches for the PR
	batches := make([]DiggerBatch, 0)
	result := db.GormDB.Where("repo_full_name = ? AND pr_number = ?", repoFullName, prNumber).Find(&batches)
	if result.Error != nil {
		slog.Error("error fetching batches for PR",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"error", result.Error)
		return nil, result.Error
	}
	slog.Info("fetched all digger batches for PR",
		"prNumber", prNumber,
		"repoFullName", repoFullName,
		"batchCount", len(batches),
		"jobCount", len(batches))

	return batches, nil

}
