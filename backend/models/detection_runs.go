package models

import (
    "encoding/json"

    configuration "github.com/diggerhq/digger/libs/digger_config"
    "gorm.io/datatypes"
    "gorm.io/gorm"
)

// DetectionRun represents a single append-only snapshot of impacted projects detection for a PR
type DetectionRun struct {
    gorm.Model
    OrganisationID       uint
    RepoFullName         string
    PrNumber             int
    TriggerType          string
    TriggerAction        string
    CommitSHA            string
    DefaultBranch        string
    TargetBranch         string
    LabelsJSON           datatypes.JSON
    ChangedFilesJSON     datatypes.JSON
    ImpactedProjectsJSON datatypes.JSON // required (can be [] when none)
    SourceMappingJSON    datatypes.JSON
}

// CreateDetectionRun inserts an append-only detection run row.
func (db *Database) CreateDetectionRun(run *DetectionRun) error {
    return db.GormDB.Create(run).Error
}

type impactedProjectPayload struct {
    Name       string `json:"name"`
    Dir        string `json:"dir"`
    Workspace  string `json:"workspace"`
    Layer      uint   `json:"layer"`
    Workflow   string `json:"workflow"`
    Terragrunt bool   `json:"terragrunt"`
    OpenTofu   bool   `json:"opentofu"`
    Pulumi     bool   `json:"pulumi"`
}

type projectSourcePayload struct {
    ImpactingLocations []string `json:"impacting_locations"`
}

// NewDetectionRun builds a DetectionRun with JSON-marshalled payloads.
func NewDetectionRun(
    organisationID uint,
    repoFullName string,
    prNumber int,
    triggerType string,
    triggerAction string,
    commitSHA string,
    defaultBranch string,
    targetBranch string,
    labels []string,
    changedFiles []string,
    impactedProjects []configuration.Project,
    sourceMapping map[string]configuration.ProjectToSourceMapping,
) (*DetectionRun, error) {
    // impacted projects
    ip := make([]impactedProjectPayload, 0, len(impactedProjects))
    for _, p := range impactedProjects {
        ip = append(ip, impactedProjectPayload{
            Name:       p.Name,
            Dir:        p.Dir,
            Workspace:  p.Workspace,
            Layer:      uint(p.Layer),
            Workflow:   p.Workflow,
            Terragrunt: p.Terragrunt,
            OpenTofu:   p.OpenTofu,
            Pulumi:     p.Pulumi,
        })
    }
    ipBytes, err := json.Marshal(ip)
    if err != nil {
        return nil, err
    }

    // source mapping
    sm := make(map[string]projectSourcePayload)
    for k, v := range sourceMapping {
        sm[k] = projectSourcePayload{ImpactingLocations: v.ImpactingLocations}
    }
    smBytes, err := json.Marshal(sm)
    if err != nil {
        return nil, err
    }

    // labels
    var labelsBytes []byte
    if labels != nil {
        labelsBytes, err = json.Marshal(labels)
        if err != nil {
            return nil, err
        }
    } else {
        labelsBytes = []byte("null")
    }

    // changed files
    var cfBytes []byte
    if changedFiles != nil {
        cfBytes, err = json.Marshal(changedFiles)
        if err != nil {
            return nil, err
        }
    } else {
        cfBytes = []byte("null")
    }

    dr := &DetectionRun{
        OrganisationID:       organisationID,
        RepoFullName:         repoFullName,
        PrNumber:             prNumber,
        TriggerType:          triggerType,
        TriggerAction:        triggerAction,
        CommitSHA:            commitSHA,
        DefaultBranch:        defaultBranch,
        TargetBranch:         targetBranch,
        LabelsJSON:           datatypes.JSON(labelsBytes),
        ChangedFilesJSON:     datatypes.JSON(cfBytes),
        ImpactedProjectsJSON: datatypes.JSON(ipBytes),
        SourceMappingJSON:    datatypes.JSON(smBytes),
    }
    return dr, nil
}

