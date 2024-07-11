/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/google/go-github/v61/github"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/spf13/cobra"
)

var viperExec *viper.Viper

type execConfig struct {
	Project string `mapstructure:"project"`
	Command string `mapstructure:"command"`
}

func getRepoUsername() (string, error) {
	// Execute 'git config --get remote.origin.url' to get the URL of the origin remote
	cmd := exec.Command("git", "config", "--get", "user.name")
	out, err := cmd.Output()
	return string(out), err
}

func getDefaultBranch() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	out, err := cmd.Output()
	return strings.ReplaceAll(strings.TrimSpace(string(out)), "refs/remotes/origin/", ""), err
}

func getPrBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func getRepoFullname() (string, error) {
	// Execute 'git config --get remote.origin.url' to get the URL of the origin remote
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Convert the output to a string and trim any whitespace
	originURL := strings.TrimSpace(string(out))

	// Extract the organization/user name and repository name from the URL
	var repoFullname string
	if strings.HasPrefix(originURL, "git@") {
		// Format: git@github.com:orgName/repoName.git
		parts := strings.Split(originURL, ":")
		repoFullname = parts[1]
	}

	return repoFullname, nil
}

func GetSpec(diggerUrl string, authToken string, command string, actor string, projectMarshalled string, diggerConfigMarshalled string, repoFullName string, defaultBanch string, prBranch string) ([]byte, error) {
	payload := spec.GetSpecPayload{
		Command:       command,
		RepoFullName:  repoFullName,
		Actor:         actor,
		DefaultBranch: defaultBanch,
		PrBranch:      prBranch,
		DiggerConfig:  diggerConfigMarshalled,
		Project:       projectMarshalled,
	}
	u, err := url.Parse(diggerUrl)
	if err != nil {
		log.Fatalf("Not able to parse digger cloud url: %v", err)
	}
	u.Path = filepath.Join("get-spec")

	request := payload.ToMapStruct()

	jsonData, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("Not able to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", u.String(), bytes.NewBuffer(jsonData))

	if err != nil {
		return nil, fmt.Errorf("error while creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	client := http.DefaultClient
	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("error while sending request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status when reporting a project: %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not read response body: %v", err)
	}

	return body, nil
}
func pushToBranch(prBranch string) error {
	cmd := exec.Command("git", "push", "origin", prBranch)
	_, err := cmd.Output()
	return err
}

func GetWorkflowIdAndUrlFromDiggerJobId(client *github.Client, repoOwner string, repoName string, diggerJobID string) (*int64, *string, error) {
	timeFilter := time.Now().Add(-5 * time.Minute)
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(context.Background(), repoOwner, repoName, &github.ListWorkflowRunsOptions{
		Created: ">=" + timeFilter.Format(time.RFC3339),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error listing workflow runs %v", err)
	}

	for _, workflowRun := range runs.WorkflowRuns {
		println(*workflowRun.ID)
		workflowjobs, _, err := client.Actions.ListWorkflowJobs(context.Background(), repoOwner, repoName, *workflowRun.ID, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("error listing workflow jobs for run %v %v", workflowRun.ID, err)
		}

		for _, workflowjob := range workflowjobs.Jobs {
			for _, step := range workflowjob.Steps {
				if strings.Contains(*step.Name, diggerJobID) {
					return workflowRun.ID, workflowRun.LogsURL, nil
				}
			}
		}

	}
	return nil, nil, fmt.Errorf("workflow not found")
}

// validateCmd represents the validate command
var execCmd = &cobra.Command{
	Use:   "exec [flags]",
	Short: "Execute a command on a project",
	Long:  `Execute a command on a project`,
	Run: func(cmd *cobra.Command, args []string) {
		var execConfig execConfig
		viperExec.Unmarshal(&execConfig)
		log.Printf("%v - %v ", execConfig.Project, execConfig.Command)
		config, _, _, err := digger_config.LoadDiggerConfig("./", true, nil)
		if err != nil {
			log.Printf("Invalid digger config file: %v. Exiting.", err)
			os.Exit(1)
		}

		diggerHostname := os.Getenv("DIGGER_BACKEND_URL")
		actor, err := getRepoUsername()
		if err != nil {
			log.Printf("could not get repo actor: %v", err)
			os.Exit(1)
		}
		repoFullname, err := getRepoFullname()
		if err != nil {
			log.Printf("could not get repo full name: %v", err)
			os.Exit(1)
		}

		defaultBanch, err := getDefaultBranch()
		if err != nil {
			log.Printf("could not get default branch: %v", err)
			os.Exit(1)
		}

		prBranch, err := getPrBranch()
		if err != nil {
			log.Printf("could not get pr branch: %v", err)
			os.Exit(1)
		}

		projectName := execConfig.Project
		command := execConfig.Command
		projectConfig := config.GetProject(projectName)
		if projectConfig == nil {
			log.Printf("project %v not found in config, does it exist?", projectName)
			os.Exit(1)
		}

		projectMarshalled, err := json.Marshal(projectConfig)
		if err != nil {
			log.Printf("could not marshall project: %v", err)
			os.Exit(1)
		}

		configMarshalled, err := json.Marshal(config)
		if err != nil {
			log.Printf("could not marshall config: %v", err)
			os.Exit(1)
		}

		specBytes, err := GetSpec(diggerHostname, "abc123", command, actor, string(projectMarshalled), string(configMarshalled), repoFullname, defaultBanch, prBranch)
		var spec spec.Spec
		err = json.Unmarshal(specBytes, &spec)

		token := os.Getenv("GITHUB_PAT_TOKEN")
		if token == "" {
			log.Printf("missing variable: GITHUB_PAT_TOKEN")
			os.Exit(1)
		}
		client := github.NewClient(nil).WithAuthToken(token)

		err = pushToBranch(prBranch)
		if err != nil {
			log.Printf("could not push to branchL %v", err)
			os.Exit(1)
		}

		inputs := orchestrator_scheduler.WorkflowInput{
			Spec:    string(specBytes),
			RunName: fmt.Sprintf("digger %v manual run by %v", command, spec.VCS.Actor),
		}
		_, err = client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), spec.VCS.RepoOwner, spec.VCS.RepoName, spec.VCS.WorkflowFile, github.CreateWorkflowDispatchEventRequest{
			Ref:    spec.Job.Branch,
			Inputs: inputs.ToMap(),
		})

		if err != nil {
			log.Printf("error while triggering workflow: %v", err)
		} else {
			log.Printf("workflow has triggered successfully! waiting for results ...")
		}

		repoOwner, repoName, _ := strings.Cut(repoFullname, "/")
		var logsUrl *string
		var runId *int64
		for {
			runId, logsUrl, err = GetWorkflowIdAndUrlFromDiggerJobId(client, repoOwner, repoName, spec.JobId)
			if err == nil {
				break
			}
		}

		log.Printf("logs url: %v runId %v", logsUrl, runId)
		//logs, _, err := client.Actions.GetWorkflowJobLogs(context.Background(), repoOwner, repoName, *runId, 3)
		//if err != nil {
		//	fmt.Printf("Error getting job logs: %v\n", err)
		//	return
		//}
		//
		//// Stream the logs
		//io.Copy(os.Stdout, logs)
		//logs.Close()

	},
}

func init() {
	flags := []pflag.Flag{
		{Name: "project", Usage: "the project to run command on"},
		{Name: "command", Usage: "the command to run"},
	}

	viperExec = viper.New()
	viperExec.SetEnvPrefix("DIGGER")
	viperExec.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viperExec.AutomaticEnv()

	for _, flag := range flags {
		execCmd.Flags().String(flag.Name, "", flag.Usage)
		execCmd.MarkFlagRequired(flag.Name)
		viperExec.BindPFlag(flag.Name, execCmd.Flags().Lookup(flag.Name))
	}

	rootCmd.AddCommand(execCmd)

}
