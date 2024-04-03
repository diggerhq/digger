package models

import (
	"gorm.io/gorm"
	"time"
)

type Organisation struct {
	gorm.Model
	Name           string `gorm:"uniqueIndex:idx_organisation"`
	ExternalSource string `gorm:"uniqueIndex:idx_external_source"`
	ExternalId     string `gorm:"uniqueIndex:idx_external_source"`
}

type Repo struct {
	gorm.Model
	Name             string `gorm:"uniqueIndex:idx_org_repo"`
	RepoFullName     string
	RepoOrganisation string
	RepoName         string
	RepoUrl          string
	OrganisationID   uint `gorm:"uniqueIndex:idx_org_repo"`
	Organisation     *Organisation
	DiggerConfig     string
}

type ProjectRun struct {
	gorm.Model
	ProjectID uint
	Project   *Project
	StartedAt int64
	EndedAt   int64
	Status    string
	Command   string
	Output    string
}

func (p *ProjectRun) MapToJsonStruct() interface{} {
	return struct {
		Id          uint
		ProjectID   uint
		ProjectName string
		StartedAt   time.Time
		EndedAt     time.Time
		Status      string
		Command     string
		Output      string
	}{
		Id:          p.ID,
		ProjectID:   p.ProjectID,
		ProjectName: p.Project.Name,
		StartedAt:   time.UnixMilli(p.StartedAt),
		EndedAt:     time.UnixMilli(p.EndedAt),
		Status:      p.Status,
		Command:     p.Command,
		Output:      p.Output,
	}
}

type ProjectStatus int

const (
	ProjectActive   ProjectStatus = 1
	ProjectInactive ProjectStatus = 2
)

type Project struct {
	gorm.Model
	Name              string `gorm:"uniqueIndex:idx_project"`
	OrganisationID    uint   `gorm:"uniqueIndex:idx_project"`
	Organisation      *Organisation
	RepoID            uint `gorm:"uniqueIndex:idx_project"`
	Repo              *Repo
	ConfigurationYaml string // TODO: probably needs to be deleted
	Status            ProjectStatus
}

func (p *Project) MapToJsonStruct() interface{} {
	lastRun, _ := DB.GetLastDiggerRunForProject(p.ID)
	status := RunSucceeded
	if lastRun != nil {
		status = lastRun.Status
	}
	return struct {
		Id                    uint   `json:"id"`
		Name                  string `json:"name"`
		Directory             string `json:"directory"`
		OrganisationID        uint   `json:"organisation_id"`
		OrganisationName      string `json:"organisation_name"`
		RepoID                uint   `json:"repo_id"`
		RepoFullName          string `json:"repo_full_name"`
		RepoName              string `json:"repo_name"`
		RepoOrg               string `json:"repo_org"`
		RepoUrl               string `json:"repo_url"`
		LastActivityTimestamp string `json:"last_activity_timestamp"`
		LastActivityAuthor    string `json:"last_activity_author"`
		LastActivityStatus    string `json:"last_activity_status"`
	}{
		Id:                    p.ID,
		Name:                  p.Name,
		OrganisationID:        p.OrganisationID,
		RepoID:                p.RepoID,
		OrganisationName:      p.Organisation.Name,
		RepoFullName:          p.Repo.RepoFullName,
		RepoName:              p.Repo.RepoName,
		RepoOrg:               p.Repo.RepoOrganisation,
		RepoUrl:               p.Repo.RepoUrl,
		LastActivityTimestamp: p.UpdatedAt.String(),
		LastActivityAuthor:    "unknown",
		LastActivityStatus:    string(status),
	}

}

type Token struct {
	gorm.Model
	Value          string `gorm:"uniqueIndex:idx_token"`
	OrganisationID uint
	Organisation   *Organisation
	Type           string
}

const (
	AccessPolicyType = "access"
	AdminPolicyType  = "admin"
)
