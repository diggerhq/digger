package models

import (
	"time"

	"gorm.io/gorm"
)

type Organisation struct {
	gorm.Model
	Name            string `gorm:"Index:idx_organisation"`
	ExternalSource  string `gorm:"uniqueIndex:idx_external_source"`
	ExternalId      string `gorm:"uniqueIndex:idx_external_source"`
	DriftEnabled    bool   `gorm:"default:false"`
	DriftWebhookUrl string
	DriftCronTab    string `gorm:"default:'0 0 * * *'"`
}

type Repo struct {
	gorm.Model
	Name             string `gorm:"uniqueIndex:idx_org_repo"`
	RepoFullName     string
	RepoOrganisation string
	RepoName         string
	RepoUrl          string
	VCS              DiggerVCSType `gorm:"default:'github'"`
	OrganisationID   uint          `gorm:"uniqueIndex:idx_org_repo"`
	Organisation     *Organisation
	DiggerConfig     string
}

type ProjectRun struct {
	gorm.Model
	ProjectID     uint
	Project       *Project
	StartedAt     int64
	EndedAt       int64
	Status        string
	Command       string
	Output        string
	ActorUsername string
}

func (p *ProjectRun) MapToJsonStruct() interface{} {
	return struct {
		Id            uint      `json:"id"`
		ProjectID     uint      `json:"project_id"`
		ProjectName   string    `json:"project_name"`
		RepoFullName  string    `json:"repo_full_name"`
		RepoUrl       string    `json:"repo_url"`
		ActorUsername string    `json:"actor_username"`
		StartedAt     time.Time `json:"started_at"`
		EndedAt       time.Time `json:"ended_at"`
		Status        string    `json:"status"`
		Command       string    `json:"command"`
		Output        string    `json:"output"`
	}{
		Id:            p.ID,
		ProjectID:     p.ProjectID,
		ProjectName:   p.Project.Name,
		StartedAt:     time.UnixMilli(p.StartedAt),
		EndedAt:       time.UnixMilli(p.EndedAt),
		Status:        p.Status,
		Command:       p.Command,
		Output:        p.Output,
		ActorUsername: p.ActorUsername,
	}
}

type ProjectStatus int

const (
	ProjectActive   ProjectStatus = 1
	ProjectInactive ProjectStatus = 2
)

type DriftStatus string

var DriftStatusNewDrift = "new drift"
var DriftStatusNoDrift = "no drift"
var DriftStatusAcknowledgeDrift = "acknowledged drift"

type Project struct {
	gorm.Model
	Name               string `gorm:"uniqueIndex:idx_project_org"`
	Directory          string
	OrganisationID     uint `gorm:"uniqueIndex:idx_project_org"`
	Organisation       *Organisation
	RepoFullName       string      `gorm:"uniqueIndex:idx_project_org"`
	DriftEnabled       bool        `gorm:"default:false"`
	DriftStatus        DriftStatus `gorm:"default:'no drift'"`
	LatestDriftCheck   time.Time
	DriftTerraformPlan string
	Status             ProjectStatus
	IsGenerated        bool
	IsInMainBranch     bool
}

func (p *Project) MapToJsonStruct() interface{} {
	lastRun, _ := DB.GetLastDiggerRunForProject(p.Name)
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
		IsInMainBranch        bool   `json:"is_in_main_branch"`
		IsGenerated           bool   `json:"is_generated"`
		DriftEnabled          bool   `json:"drift_enabled"`
		DriftStatus           string `json:"drift_status"`
		LatestDriftCheck      string `json:"latest_drift_check"`
		DriftTerraformPlan    string `json:"drift_terraform_plan"`
		LastActivityTimestamp string `json:"last_activity_timestamp"`
		LastActivityAuthor    string `json:"last_activity_author"`
		LastActivityStatus    string `json:"last_activity_status"`
	}{
		Id:                    p.ID,
		Name:                  p.Name,
		Directory:             p.Directory,
		OrganisationID:        p.OrganisationID,
		OrganisationName:      p.Organisation.Name,
		RepoFullName:          p.RepoFullName,
		DriftEnabled:          p.DriftEnabled,
		DriftStatus:           string(p.DriftStatus),
		LatestDriftCheck:      p.LatestDriftCheck.String(),
		DriftTerraformPlan:    p.DriftTerraformPlan,
		LastActivityTimestamp: p.UpdatedAt.String(),
		LastActivityAuthor:    "unknown",
		LastActivityStatus:    string(status),
		IsGenerated:           p.IsGenerated,
		IsInMainBranch:        p.IsInMainBranch,
	}
}
func (r *Repo) MapToJsonStruct() interface{} {
	OrganisationName := func() string {
		if r.Organisation == nil {
			return ""
		}
		return r.Organisation.Name
	}
	return struct {
		Id               uint   `json:"id"`
		Name             string `json:"name"`
		RepoFullName     string `json:"repo_full_name"`
		RepoUrl          string `json:"repo_url"`
		VCS              string `json:"vcs"`
		OrganisationID   uint   `json:"organisation_id"`
		OrganisationName string `json:"organisation_name"`
	}{
		Id:               r.ID,
		Name:             r.RepoName,
		RepoFullName:     r.RepoFullName,
		RepoUrl:          r.RepoUrl,
		VCS:              string(r.VCS),
		OrganisationID:   r.OrganisationID,
		OrganisationName: OrganisationName(),
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
	CliJobAccessType = "cli_access"
)
