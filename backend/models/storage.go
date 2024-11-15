package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dchest/uniuri"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"log"
	"net/http"
	"time"
)

func (db *Database) GetProjectsFromContext(c *gin.Context, orgIdKey string) ([]Project, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)

	log.Printf("getProjectsFromContext, org id: %v\n", loggedInOrganisationId)

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
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, false
	}

	log.Printf("getProjectsFromContext, number of projects:%d\n", len(projects))
	return projects, true
}

func (db *Database) GetReposFromContext(c *gin.Context, orgIdKey string) ([]Repo, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)

	log.Printf("GetReposFromContext, org id: %v\n", loggedInOrganisationId)

	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return nil, false
	}

	var repos []Repo

	err := db.GormDB.Preload("Organisation").
		Joins("INNER JOIN organisations ON repos.organisation_id = organisations.id").
		Where("repos.organisation_id = ?", loggedInOrganisationId).Find(&repos).Error

	if err != nil {
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, false
	}

	log.Printf("GetReposFromContext, number of repos:%d\n", len(repos))
	return repos, true
}

func (db *Database) GetPoliciesFromContext(c *gin.Context, orgIdKey string) ([]Policy, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)

	log.Printf("getPoliciesFromContext, org id: %v\n", loggedInOrganisationId)

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
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, false
	}

	log.Printf("getPoliciesFromContext, number of policies:%d\n", len(policies))
	return policies, true
}

func (db *Database) GetProjectRunsForOrg(orgId int) ([]ProjectRun, error) {
	var runs []ProjectRun

	err := db.GormDB.Preload("Project").Preload("Project.Organisation").Preload("Project.Repo").
		Joins("INNER JOIN projects ON projects.id = project_runs.project_id").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", orgId).Order("created_at desc").Limit(100).Find(&runs).Error

	if err != nil {
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, fmt.Errorf("unknown error occurred while fetching database, %v\n", err)
	}

	log.Printf("getProjectRunsFromContext, number of runs:%d\n", len(runs))
	return runs, nil
}

func (db *Database) GetProjectRunsFromContext(c *gin.Context, orgIdKey string) ([]ProjectRun, bool) {
	loggedInOrganisationId := c.GetUint(orgIdKey)

	log.Printf("getProjectRunsFromContext, org id: %v\n", loggedInOrganisationId)

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

	log.Printf("GetProjectByRunId, org id: %v\n", loggedInOrganisationId)
	var projectRun ProjectRun

	err := db.GormDB.Preload("Project").Preload("Project.Organisation").Preload("Project.Repo").
		Joins("INNER JOIN projects ON projects.id = project_runs.project_id").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", loggedInOrganisationId).
		Where("project_runs.id = ?", runId).First(&projectRun).Error

	if err != nil {
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
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

	log.Printf("GetProjectByProjectId, org id: %v\n", loggedInOrganisationId)
	var project Project

	err := db.GormDB.Preload("Organisation").Preload("Repo").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", loggedInOrganisationId).
		Where("projects.id = ?", projectId).First(&project).Error

	if err != nil {
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, false
	}

	return &project, true
}

func (db *Database) GetProject(projectId uint) (*Project, error) {
	log.Printf("GetProject, project id: %v\n", projectId)
	var project Project

	err := db.GormDB.Preload("Organisation").Preload("Repo").
		Where("id = ?", projectId).
		First(&project).Error

	if err != nil {
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, err
	}

	return &project, nil
}

// GetProjectByName return project for specified org and repo
// if record doesn't exist return nil
func (db *Database) GetProjectByName(orgId any, repo *Repo, name string) (*Project, error) {
	log.Printf("GetProjectByName, org id: %v, project name: %v\n", orgId, name)
	var project Project

	err := db.GormDB.Preload("Organisation").Preload("Repo").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", orgId).
		Where("repos.id = ?", repo.ID).
		Where("projects.name = ?", name).First(&project).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, err
	}

	return &project, nil
}

// GetProjectByRepo return projects for specified org and repo
func (db *Database) GetProjectByRepo(orgId any, repo *Repo) ([]Project, error) {
	log.Printf("GetProjectByRepo, org id: %v, repo name: %v\n", orgId, repo.Name)
	projects := make([]Project, 0)

	err := db.GormDB.Preload("Organisation").Preload("Repo").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", orgId).
		Where("repos.id = ?", repo.ID).Find(&projects).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, err
	}

	return projects, nil
}

func (db *Database) GetPolicyByPolicyId(c *gin.Context, policyId uint, orgIdKey string) (*Policy, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)
	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return nil, false
	}

	log.Printf("getPolicyByPolicyId, org id: %v\n", loggedInOrganisationId)
	var policy Policy

	err := db.GormDB.Preload("Project").Preload("Project.Organisation").Preload("Project.Repo").
		Joins("INNER JOIN projects ON projects.id = policies.project_id").
		Joins("INNER JOIN repos ON projects.repo_id = repos.id").
		Joins("INNER JOIN organisations ON projects.organisation_id = organisations.id").
		Where("projects.organisation_id = ?", loggedInOrganisationId).
		Where("policies.id = ?", policyId).First(&policy).Error

	if err != nil {
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, false
	}

	return &policy, true
}

func (db *Database) GetDefaultRepo(c *gin.Context, orgIdKey string) (*Repo, bool) {
	loggedInOrganisationId, exists := c.Get(orgIdKey)
	if !exists {
		log.Print("Not allowed to access this resource")
		return nil, false
	}

	log.Printf("getDefaultRepo, org id: %v\n", loggedInOrganisationId)
	var repo Repo

	err := db.GormDB.Preload("Organisation").
		Joins("INNER JOIN organisations ON repos.organisation_id = organisations.id").
		Where("organisations.id = ?", loggedInOrganisationId).First(&repo).Error

	if err != nil {
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, false
	}

	return &repo, true
}

// GetRepo returns digger repo by organisationId and repo name (diggerhq-digger)
// it will return an empty object if record doesn't exist in database
func (db *Database) GetRepo(orgIdKey any, repoName string) (*Repo, error) {
	var repo Repo

	err := db.GormDB.Preload("Organisation").
		Joins("INNER JOIN organisations ON repos.organisation_id = organisations.id").
		Where("organisations.id = ? AND repos.name=?", orgIdKey, repoName).First(&repo).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("repo not found %v", repoName)
		}
		log.Printf("Failed to find digger repo for orgId: %v, and repoName: %v, error: %v\n", orgIdKey, repoName, err)
		return nil, err
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
		log.Printf("Failed to find digger repo for orgId: %v, and repoId: %v, error: %v\n", orgIdKey, repoId, err)
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
		log.Printf("Record for installation_id: %d, repo: %s, with status=active exist already.", installationId, repoFullName)
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
			log.Printf("Record not found for installationId: %d, status=active, githubAppId: %d and repo: %s", installationId, appId, repoFullName)
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

func (db *Database) CreateGithubAppConnection(name string, githubId int64, ClientID string, ClientSecretEncrypted string, WebhookSecretEncrypted string, PrivateKeyEncrypted string, PrivateKeyBase64Encrypted string, Org string, url string, orgId uint) (*GithubAppConnection, error) {
	app := GithubAppConnection{
		Name:                      name,
		GithubId:                  githubId,
		ClientID:                  ClientID,
		ClientSecretEncrypted:     ClientSecretEncrypted,
		WebhookSecretEncrypted:    WebhookSecretEncrypted,
		PrivateKeyEncrypted:       PrivateKeyEncrypted,
		PrivateKeyBase64Encrypted: PrivateKeyBase64Encrypted,
		Org:                       Org,
		GithubAppUrl:              url,
		OrganisationID:            orgId,
	}
	result := db.GormDB.Save(&app)
	if result.Error != nil {
		return nil, result.Error
	}
	log.Printf("CreateGithubApp (name: %v, url: %v) has been created successfully\n", app.Name, app.GithubAppUrl)
	return &app, nil
}

func (db *Database) GetGithubAppConnectionById(id string) (*GithubAppConnection, error) {
	app := GithubAppConnection{}
	result := db.GormDB.Where("id = ?", id).Find(&app)
	if result.Error != nil {
		log.Printf("Failed to find GitHub App for id: %v, error: %v\n", id, result.Error)
		return nil, result.Error
	}
	return &app, nil
}

// GetGithubApp return GithubApp by Id
func (db *Database) GetGithubAppConnection(gitHubAppId any) (*GithubAppConnection, error) {
	app := GithubAppConnection{}
	result := db.GormDB.Where("github_id = ?", gitHubAppId).Find(&app)
	if result.Error != nil {
		log.Printf("Failed to find GitHub App for id: %v, error: %v\n", gitHubAppId, result.Error)
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
		log.Printf("installation %v has been linked to the org %v already.", installationId, org.Name)
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
	log.Printf("GithubAppInstallationLink (org: %v, installationId: %v) has been created successfully\n", org.Name, installationId)
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
		log.Printf("Failed to update GithubAppInstallationLink, id: %v, error: %v", link.ID, result.Error)
		return nil, result.Error
	}
	return link, nil
}

func (db *Database) CreateDiggerJobLink(diggerJobId string, repoFullName string) (*GithubDiggerJobLink, error) {
	link := GithubDiggerJobLink{Status: DiggerJobLinkCreated, DiggerJobId: diggerJobId, RepoFullName: repoFullName}
	result := db.GormDB.Save(&link)
	if result.Error != nil {
		log.Printf("Failed to create GithubDiggerJobLink, %v, repo: %v \n", diggerJobId, repoFullName)
		return nil, result.Error
	}
	log.Printf("GithubDiggerJobLink %v, (repo: %v) has been created successfully\n", diggerJobId, repoFullName)
	return &link, nil
}

func (db *Database) GetDiggerJobLink(diggerJobId string) (*GithubDiggerJobLink, error) {
	link := GithubDiggerJobLink{}
	result := db.GormDB.Where("digger_job_id = ?", diggerJobId).Find(&link)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.Printf("Failed to get DiggerJobLink, %v", diggerJobId)
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
			log.Printf("Failed to update GithubDiggerJobLink, %v, repo: %v \n", diggerJobId, repoFullName)
			return nil, result.Error
		}
	}
	if result.RowsAffected == 1 {
		jobLink.GithubJobId = githubJobId
		result = db.GormDB.Save(&jobLink)
		if result.Error != nil {
			return nil, result.Error
		}
		log.Printf("GithubDiggerJobLink %v, (repo: %v) has been updated successfully\n", diggerJobId, repoFullName)
		return &jobLink, nil
	}
	return &jobLink, nil
}

func (db *Database) GetOrganisationById(orgId any) (*Organisation, error) {
	log.Printf("GetOrganisationById, orgId: %v, type: %T \n", orgId, orgId)
	org := Organisation{}
	err := db.GormDB.Where("id = ?", orgId).First(&org).Error
	if err != nil {
		return nil, fmt.Errorf("Error fetching organisation: %v\n", err)
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

func (db *Database) CreateDiggerBatch(vcsType DiggerVCSType, githubInstallationId int64, repoOwner string, repoName string, repoFullname string, PRNumber int, diggerConfig string, branchName string, batchType scheduler.DiggerCommand, commentId *int64, gitlabProjectId int) (*DiggerBatch, error) {
	uid := uuid.New()
	batch := &DiggerBatch{
		ID:                   uid,
		VCS:                  vcsType,
		GithubInstallationId: githubInstallationId,
		RepoOwner:            repoOwner,
		RepoName:             repoName,
		RepoFullName:         repoFullname,
		PrNumber:             PRNumber,
		CommentId:            commentId,
		Status:               scheduler.BatchJobCreated,
		BranchName:           branchName,
		DiggerConfig:         diggerConfig,
		BatchType:            batchType,
		GitlabProjectId:      gitlabProjectId,
	}
	result := db.GormDB.Save(batch)
	if result.Error != nil {
		return nil, result.Error
	}

	log.Printf("DiggerBatch (id: %v) has been created successfully\n", batch.ID)
	return batch, nil
}

func (db *Database) UpdateDiggerBatch(batch *DiggerBatch) error {
	result := db.GormDB.Save(batch)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("batch %v has been updated successfully\n", batch.ID)
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
			log.Printf("Failed to get DiggerJob by batch id: %v, error: %v\n", batchId, result.Error)
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

	log.Printf("DiggerJob %v, (id: %v) has been created successfully\n", job.DiggerJobID, job.ID)
	return job, nil
}

func (db *Database) ListDiggerRunsForProject(projectName string, repoId uint) ([]DiggerRun, error) {
	var runs []DiggerRun

	err := db.GormDB.Preload("PlanStage").Preload("ApplyStage").
		Where("project_name = ? AND repo_id=  ?", projectName, repoId).Order("created_at desc").Find(&runs).Error

	if err != nil {
		log.Printf("Unknown error occurred while fetching database, %v\n", err)
		return nil, err
	}

	log.Printf("ListDiggerRunsForProject, number of runs:%d\n", len(runs))
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
		log.Printf("Failed to create DiggerRun: %v, error: %v\n", dr.ID, result.Error)
		return nil, result.Error
	}
	log.Printf("DiggerRun %v, has been created successfully\n", dr.ID)
	return dr, nil
}

func (db *Database) CreateDiggerRunStage(batchId string) (*DiggerRunStage, error) {
	drs := &DiggerRunStage{
		BatchID: &batchId,
	}
	result := db.GormDB.Save(drs)
	if result.Error != nil {
		log.Printf("Failed to create DiggerRunStage: %v, error: %v\n", drs.ID, result.Error)
		return nil, result.Error
	}
	log.Printf("DiggerRunStage %v, has been created successfully\n", drs.ID)
	return drs, nil
}

func (db *Database) GetLastDiggerRunForProject(projectName string) (*DiggerRun, error) {
	diggerRun := &DiggerRun{}
	result := db.GormDB.Where("project_name = ? AND status <> ?", projectName, RunQueued).Order("created_at Desc").First(diggerRun)
	if result.Error != nil {
		log.Printf("error while fetching last digger run: %v", result.Error)
		return nil, result.Error
	}
	return diggerRun, nil
}

func (db *Database) GetDiggerRun(id uint) (*DiggerRun, error) {
	dr := &DiggerRun{}
	result := db.GormDB.Preload("Repo").
		Preload("ApplyStage").
		Preload("PlanStage").
		Where("id=? ", id).Find(dr)
	if result.Error != nil {
		return nil, result.Error
	}
	return dr, nil
}

func (db *Database) CreateDiggerRunQueueItem(diggeRrunId uint, projectId uint) (*DiggerRunQueueItem, error) {
	drq := &DiggerRunQueueItem{
		DiggerRunId: diggeRrunId,
		ProjectId:   projectId,
	}
	result := db.GormDB.Save(drq)
	if result.Error != nil {
		log.Printf("Failed to create DiggerRunQueueItem: %v, error: %v\n", drq.ID, result.Error)
		return nil, result.Error
	}
	log.Printf("DiggerRunQueueItem %v, has been created successfully\n", drq.ID)
	return drq, nil
}

func (db *Database) GetDiggerRunQueueItem(id uint) (*DiggerRunQueueItem, error) {
	dr := &DiggerRunQueueItem{}
	result := db.GormDB.Preload("DiggerRun").Where("id=? ", id).Find(dr)
	if result.Error != nil {
		return nil, result.Error
	}
	return dr, nil
}

func (db *Database) GetDiggerJobFromRunStage(stage DiggerRunStage) (*DiggerJob, error) {
	job := &DiggerJob{}
	result := db.GormDB.Preload("Batch").Take(job, "batch_id = ?", stage.BatchID)
	if result.Error != nil {
		return nil, result.Error
	}
	return job, nil
}

func (db *Database) UpdateDiggerRun(diggerRun *DiggerRun) error {
	result := db.GormDB.Save(diggerRun)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("diggerRun %v has been updated successfully\n", diggerRun.ID)
	return nil
}

func (db *Database) DequeueRunItem(queueItem *DiggerRunQueueItem) error {
	log.Printf("DiggerRunQueueItem Deleting: %v", queueItem.ID)
	result := db.GormDB.Delete(queueItem)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("diggerRunQueueItem %v has been deleted successfully\n", queueItem.ID)
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
		fmt.Printf("%v", tx.Error)
		return nil, tx.Error
	}

	// 2. Preload Project and DiggerRun for every DiggerrunQueue item (front of queue)
	var runqueuesWithData []DiggerRunQueueItem
	diggerRunIds := lo.Map(runqueues, func(run DiggerRunQueueItem, index int) uint {
		return run.DiggerRunId
	})

	tx = db.GormDB.Preload("DiggerRun").Preload("DiggerRun.Repo").
		Preload("DiggerRun.PlanStage").Preload("DiggerRun.ApplyStage").
		Preload("DiggerRun.PlanStage.Batch").Preload("DiggerRun.ApplyStage.Batch").
		Where("digger_run_queue_items.digger_run_id in ?", diggerRunIds).Find(&runqueuesWithData)

	if tx.Error != nil {
		fmt.Printf("%v", tx.Error)
		return nil, tx.Error
	}

	return runqueuesWithData, nil
}

func (db *Database) UpdateDiggerJobSummary(diggerJobId string, resourcesCreated uint, resourcesUpdated uint, resourcesDeleted uint) (*DiggerJob, error) {
	diggerJob, err := db.GetDiggerJob(diggerJobId)
	if err != nil {
		return nil, fmt.Errorf("Could not get digger job")
	}
	var jobSummary *DiggerJobSummary
	jobSummary = &diggerJob.DiggerJobSummary
	jobSummary.ResourcesCreated = resourcesCreated
	jobSummary.ResourcesUpdated = resourcesUpdated
	jobSummary.ResourcesDeleted = resourcesDeleted

	result := db.GormDB.Save(&jobSummary)
	if result.Error != nil {
		return nil, result.Error
	}

	log.Printf("DiggerJob %v summary has been updated successfully\n", diggerJobId)
	return diggerJob, nil
}

func (db *Database) UpdateDiggerJob(job *DiggerJob) error {
	result := db.GormDB.Save(job)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("DiggerJob %v, (id: %v) has been updated successfully\n", job.DiggerJobID, job.ID)
	return nil
}

func (db *Database) GetDiggerJobsForBatch(batchId uuid.UUID) ([]DiggerJob, error) {
	jobs := make([]DiggerJob, 0)

	var where *gorm.DB
	where = db.GormDB.Where("digger_jobs.batch_id = ?", batchId)

	result := where.Preload("Batch").Preload("DiggerJobSummary").Find(&jobs)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	return jobs, nil
}

func (db *Database) GetDiggerJobsForBatchWithStatus(batchId uuid.UUID, status []scheduler.DiggerJobStatus) ([]DiggerJob, error) {
	jobs := make([]DiggerJob, 0)

	var where *gorm.DB
	where = db.GormDB.Where("digger_jobs.batch_id = ?", batchId).Where("status IN ?", status)

	result := where.Preload("Batch").Preload("DiggerJobSummary").Find(&jobs)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	return jobs, nil
}

func (db *Database) GetDiggerJobsWithStatus(status scheduler.DiggerJobStatus) ([]DiggerJob, error) {
	jobs := make([]DiggerJob, 0)

	var where *gorm.DB
	where = db.GormDB.Where("status = ?", status)

	result := where.Preload("Batch").Find(&jobs)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
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
			return nil, result.Error
		}
	}
	return jobs, nil
}

func (db *Database) GetDiggerJob(jobId string) (*DiggerJob, error) {
	job := &DiggerJob{}
	result := db.GormDB.Preload("Batch").Preload("DiggerJobSummary").Where("digger_job_id=? ", jobId).Find(job)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
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
			log.Printf("Failed to get DiggerJobLink by parent job id: %v, error: %v\n", parentId, result.Error)
			return nil, result.Error
		}
	}
	return jobParentLinks, nil
}

func (db *Database) CreateDiggerJobParentLink(parentJobId string, jobId string) error {
	jobParentLink := DiggerJobParentLink{ParentDiggerJobId: parentJobId, DiggerJobId: jobId}
	result := db.GormDB.Create(&jobParentLink)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (db *Database) GetDiggerJobParentLinksChildId(childId *string) ([]DiggerJobParentLink, error) {
	var jobParentLinks []DiggerJobParentLink
	result := db.GormDB.Where("digger_job_id=?", childId).Find(&jobParentLinks)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			log.Printf("Failed to get DiggerJobLink by parent job id: %v, error: %v\n", childId, result.Error)
			return nil, result.Error
		}
	}
	return jobParentLinks, nil
}

func (db *Database) GetOrganisation(tenantId any) (*Organisation, error) {
	org := &Organisation{}
	result := db.GormDB.Take(org, "external_id = ?", tenantId)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		} else {
			return nil, result.Error
		}
	}
	return org, nil
}

func (db *Database) CreateOrganisation(name string, externalSource string, tenantId string) (*Organisation, error) {
	org := &Organisation{Name: name, ExternalSource: externalSource, ExternalId: tenantId}
	result := db.GormDB.Save(org)
	if result.Error != nil {
		log.Printf("Failed to create organisation: %v, error: %v\n", name, result.Error)
		return nil, result.Error
	}
	log.Printf("Organisation %s, (id: %v) has been created successfully\n", name, org.ID)
	return org, nil
}

func (db *Database) CreateProject(name string, org *Organisation, repo *Repo, isGenerated bool, isInMainBranch bool) (*Project, error) {
	project := &Project{
		Name:           name,
		Organisation:   org,
		Repo:           repo,
		Status:         ProjectActive,
		IsGenerated:    isGenerated,
		IsInMainBranch: isInMainBranch,
	}
	result := db.GormDB.Save(project)
	if result.Error != nil {
		log.Printf("Failed to create project: %v, error: %v\n", name, result.Error)
		return nil, result.Error
	}
	log.Printf("Project %s, (id: %v) has been created successfully\n", name, project.ID)
	return project, nil
}

func (db *Database) UpdateProject(project *Project) error {
	result := db.GormDB.Save(project)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("project %v has been updated successfully\n", project.ID)
	return nil
}

func (db *Database) CreateRepo(name string, repoFullName string, repoOrganisation string, repoName string, repoUrl string, org *Organisation, diggerConfig string) (*Repo, error) {
	var repo Repo
	// check if repo exist already, do nothing in this case
	result := db.GormDB.Where("name = ? AND organisation_id=?", name, org.ID).Find(&repo)
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}
	if result.RowsAffected > 0 {
		// record already exist, do nothing
		return &repo, nil
	}
	repo = Repo{
		Name:             name,
		Organisation:     org,
		DiggerConfig:     diggerConfig,
		RepoFullName:     repoFullName,
		RepoOrganisation: repoOrganisation,
		RepoName:         repoName,
		RepoUrl:          repoUrl,
	}
	result = db.GormDB.Save(&repo)
	if result.Error != nil {
		log.Printf("Failed to create repo: %v, error: %v\n", name, result.Error)
		return nil, result.Error
	}
	log.Printf("Repo %s, (id: %v) has been created successfully\n", name, repo.ID)
	return &repo, nil
}

func (db *Database) GetToken(tenantId any) (*Token, error) {
	token := &Token{}
	result := db.GormDB.Take(token, "value = ?", tenantId)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		} else {
			return nil, result.Error
		}
	}
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
		log.Printf("failed to create token: %v", err)
		return nil, err
	}
	return jobToken, nil
}

func (db *Database) GetJobToken(tenantId any) (*JobToken, error) {
	token := &JobToken{}
	result := db.GormDB.Take(token, "value = ?", tenantId)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		} else {
			return nil, result.Error
		}
	}
	return token, nil
}

func (db *Database) DeleteJobTokenArtefacts(jobTokenId uint) error {
	artefact := JobArtefact{}
	result := db.GormDB.Where("job_token_id = ?", jobTokenId).Delete(&artefact)
	if result.Error != nil {
		return result.Error
	}
	log.Printf("DeleteJobTokenArtefacts %v has been deleted successfully\n", jobTokenId)
	return nil

}

func (db *Database) GetJobArtefact(jobTokenId uint) (*JobArtefact, error) {
	var artefact JobArtefact
	if err := DB.GormDB.Where("job_token_id = ?", jobTokenId).First(&artefact).Error; err != nil {
		return nil, err
	}
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
		log.Printf("Failed to create GithubAppInstallation: %v, error: %v\n", installationId, result.Error)
		return nil, result.Error
	}
	log.Printf("GithubAppInstallation (installationId: %v, githubAppId: %v, login: %v, accountId: %v, repoFullName: %v) has been created successfully\n", installationId, githubAppId, login, accountId, repoFullName)
	return installation, nil
}

func validateDiggerConfigYaml(configYaml string) (*configuration.DiggerConfig, error) {
	diggerConfig, _, _, err := configuration.LoadDiggerConfigFromString(configYaml, "./")
	if err != nil {
		return nil, fmt.Errorf("validation error, %w", err)
	}
	return diggerConfig, nil
}

func (db *Database) UpdateRepoDiggerConfig(orgId any, config configuration.DiggerConfigYaml, repo *Repo, isMainBranch bool) error {
	log.Printf("UpdateRepoDiggerConfig, repo: %v\n", repo)

	org, err := db.GetOrganisationById(orgId)
	if err != nil {
		return err
	}
	err = db.GormDB.Transaction(func(tx *gorm.DB) error {
		if isMainBranch {
			// we reset all projects already in main branch to create new projects
			repoProjects, err := db.GetProjectByRepo(orgId, repo)
			if err != nil {
				return fmt.Errorf("could not get repo projects: %v", err)
			}
			for _, rp := range repoProjects {
				rp.IsInMainBranch = false
				err = db.UpdateProject(&rp)
				if err != nil {
					return fmt.Errorf("could not update existing main branch projects: %v", err)
				}
			}
		}

		for _, dc := range config.Projects {
			projectName := dc.Name
			p, err := db.GetProjectByName(orgId, repo, projectName)
			if err != nil {
				return fmt.Errorf("error retrieving project by name: %v", err)
			}
			if p == nil {
				_, err := db.CreateProject(projectName, org, repo, dc.Generated, isMainBranch)
				if err != nil {
					return fmt.Errorf("could not create project: %v", err)
				}
			} else {
				if isMainBranch == true {
					p.IsInMainBranch = isMainBranch
				}
				p.IsGenerated = dc.Generated
				db.UpdateProject(p)
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
		return nil, result.Error
	}

	log.Printf("CreateDiggerLock (id: %v %v) has been created successfully\n", lock.LockId, lock.Resource)
	return lock, nil
}

func (db *Database) GetDiggerLock(resource string) (*DiggerLock, error) {
	lock := &DiggerLock{}
	result := db.GormDB.Where("resource=? ", resource).First(lock)
	if result.Error != nil {
		return nil, result.Error
	}
	return lock, nil
}

func (db *Database) UpsertRepoCache(orgId uint, repoFullName string, diggerYmlStr string, diggerConfig configuration.DiggerConfig) (*RepoCache, error) {
	var repoCache RepoCache

	configMarshalled, err := json.Marshal(diggerConfig)
	if err != nil {
		log.Printf("could not marshal config: %v", err)
		return nil, fmt.Errorf("could not marshal config: %v", err)
	}

	// check if repo exist already, do nothing in this case
	result := db.GormDB.Where("org_id = ? AND repo_full_name=?", orgId, repoFullName).Find(&repoCache)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected > 0 {
		// record already exist, do nothing
		repoCache.DiggerConfig = configMarshalled
		repoCache.DiggerYmlStr = diggerYmlStr
		result = db.GormDB.Save(&repoCache)
	} else {
		// create record here
		repoCache = RepoCache{
			OrgId:        orgId,
			RepoFullName: repoFullName,
			DiggerYmlStr: diggerYmlStr,
			DiggerConfig: configMarshalled,
		}
		result = db.GormDB.Save(&repoCache)
		if result.Error != nil {
			log.Printf("Failed to create repo cache: %v, error: %v\n", repoFullName, result.Error)
			return nil, result.Error
		}
	}

	log.Printf("Repo cache %s, (id: %v) has been created successfully\n", repoFullName, repoCache.ID)
	return &repoCache, nil
}

func (db *Database) GetRepoCache(orgId uint, repoFullName string) (*RepoCache, error) {
	var repoCache RepoCache

	err := db.GormDB.
		Where("org_id = ? AND repo_full_name = ?", orgId, repoFullName).First(&repoCache).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("repo cache not found %v", err)
		}
		log.Printf("Failed to find digger repo for orgId: %v, and repoName: %v, error: %v\n", orgId, repoFullName, err)
		return nil, err
	}
	return &repoCache, nil
}
