package service_clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type MachineConfig struct {
	Name   string `json:"name"`
	Config struct {
		Image string `json:"image"`
		Env   struct {
			CloneUrl     string `json:"DIGGER_GITHUB_REPO_CLONE_URL"`
			Branch       string `json:"DIGGER_GITHUB_REPO_CLONE_BRANCH"`
			GithubToken  string `json:"DIGGER_GITHUB_TOKEN"`
			RepoFullName string `json:"DIGGER_REPO_FULL_NAME"`
			OrgId        string `json:"DIGGER_ORG_ID"`
		} `json:"env"`
		Guest struct {
			CPUs     int    `json:"cpus"`
			CPUKind  string `json:"cpu_kind"`
			MemoryMB int    `json:"memory_mb"`
		} `json:"guest"`
		AutoDestroy bool `json:"auto_destroy"`
	} `json:"config"`
}

type MachineResponse struct {
	ID string `json:"id"`
}

type QueuedResponse struct {
	Queued string `json:"queued"`
}

func TriggerProjectsRefreshService(cloneUrl string, branch string, githubToken string, repoFullName string, orgId string) (*MachineResponse, error) {

	slog.Debug("awaiting machine fetch")

	// Prepare machine configuration
	machineConfig := MachineConfig{
		Name: fmt.Sprintf("hello-%d", time.Now().UnixMilli()),
	}

	machineConfig.Config.Image = "registry.fly.io/projects-refresh-service:latest"
	machineConfig.Config.Env.CloneUrl = cloneUrl
	machineConfig.Config.Env.Branch = branch
	machineConfig.Config.Env.GithubToken = githubToken
	machineConfig.Config.Env.RepoFullName = repoFullName
	machineConfig.Config.Env.OrgId = orgId

	machineConfig.Config.Guest.CPUs = 1
	machineConfig.Config.Guest.CPUKind = "shared"
	machineConfig.Config.Guest.MemoryMB = 256
	machineConfig.Config.AutoDestroy = true

	// Marshal JSON payload
	payload, err := json.Marshal(machineConfig)
	if err != nil {
		slog.Error("Error creating machine config", "error", err)
		return nil, err
	}

	// Create HTTP request
	apiURL := fmt.Sprintf("https://api.machines.dev/v1/apps/%s/machines", os.Getenv("JOB_APP"))
	req2, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
	if err != nil {
		slog.Error("Error creating request", "error", err)
		return nil, err
	}

	// Set headers
	req2.Header.Set("Authorization", "Bearer "+os.Getenv("FLY_PROJECTS_SVC_API_TOKEN"))
	req2.Header.Set("Content-Type", "application/json")

	// Make HTTP request
	client := &http.Client{}
	resp, err := client.Do(req2)
	if err != nil {
		slog.Error("Error making request", "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	// Parse response
	var machineResp MachineResponse
	if err := json.NewDecoder(resp.Body).Decode(&machineResp); err != nil {
		slog.Error("Error parsing response", "error", err)
		return nil, err
	}

	slog.Debug("triggered projects refresh service", "machineId", machineResp.ID)
	return &MachineResponse{ID: machineResp.ID}, nil
}
