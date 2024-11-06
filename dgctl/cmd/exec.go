/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/dgctl/utils"
	"github.com/diggerhq/digger/libs/backendapi"
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
		repoFullname = strings.ReplaceAll(repoFullname, ".git", "")
	}

	return repoFullname, nil
}

func GetUrlContents(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("%v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%v", err)
	}

	content := string(body)
	return content, nil
}

func GetSpec(diggerUrl string, authToken string, command string, actor string, projectMarshalled string, diggerConfigMarshalled string, repoFullName string) ([]byte, error) {
	payload := spec.GetSpecPayload{
		Command:      command,
		RepoFullName: repoFullName,
		Actor:        actor,
		DiggerConfig: diggerConfigMarshalled,
		Project:      projectMarshalled,
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
		return nil, fmt.Errorf("unexpected status when getting spec: %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Could not read response body: %v", err)
	}

	return body, nil
}

func GetWorkflowIdAndUrlFromDiggerJobId(client *github.Client, repoOwner string, repoName string, diggerJobID string) (*int64, *int64, *string, error) {
	timeFilter := time.Now().Add(-5 * time.Minute)
	runs, _, err := client.Actions.ListRepositoryWorkflowRuns(context.Background(), repoOwner, repoName, &github.ListWorkflowRunsOptions{
		Created: ">=" + timeFilter.Format(time.RFC3339),
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error listing workflow runs %v", err)
	}

	for _, workflowRun := range runs.WorkflowRuns {
		workflowjobs, _, err := client.Actions.ListWorkflowJobs(context.Background(), repoOwner, repoName, *workflowRun.ID, nil)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error listing workflow jobs for run %v %v", workflowRun.ID, err)
		}

		for _, workflowjob := range workflowjobs.Jobs {
			for _, step := range workflowjob.Steps {
				if strings.Contains(*step.Name, diggerJobID) {
					url := fmt.Sprintf("https://github.com/%v/%v/actions/runs/%v", repoOwner, repoName, *workflowRun.ID)
					return workflowRun.ID, workflowjob.ID, &url, nil
				}
			}
		}

	}
	return nil, nil, nil, fmt.Errorf("workflow not found")
}

func cleanupDiggerOutput(output string) string {

	startingDelimiter := "<========= DIGGER RUNNING IN MANUAL MODE =========>"
	endingDelimiter := "<========= DIGGER COMPLETED =========>"

	startPos := 0
	endPos := len(output)
	// removes output of terraform -version command that terraform-exec executes on every run
	i := strings.Index(output, startingDelimiter)
	if i != -1 {
		startPos = i + len(startingDelimiter)
	}

	e := strings.Index(output, endingDelimiter)
	if e != -1 {
		endPos = e
	}

	// This should not happen but in case we get here we avoid slice bounds out of range exception by resetting endPos
	if endPos <= startPos {
		endPos = len(output)
	}
	return output[startPos:endPos]
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

		if execConfig.Command != "digger plan" {
			log.Printf("ERROR: currently only 'digger plan' supported with exec command")
			os.Exit(1)
		}

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

		specBytes, err := GetSpec(diggerHostname, "abc123", command, actor, string(projectMarshalled), string(configMarshalled), repoFullname)
		if err != nil {
			log.Printf("failed to get spec from backend: %v", err)
			os.Exit(1)
		}
		var spec spec.Spec
		err = json.Unmarshal(specBytes, &spec)

		// attach zip archive to backend
		backendToken := spec.Job.BackendJobToken
		zipLocation, err := utils.ArchiveGitRepo("./")
		if err != nil {
			log.Printf("error archiving zip repo: %v", err)
			os.Exit(1)
		}
		backendApi := backendapi.DiggerApi{DiggerHost: diggerHostname, AuthToken: backendToken}
		statusCode, respBody, err := backendApi.UploadJobArtefact(zipLocation)
		if err != nil {
			log.Printf("could not attach zip artefact: %v", err)
			os.Exit(1)
		}
		if *statusCode != 200 {
			log.Printf("unexpected status code from backend: %v", *statusCode)
			log.Printf("server response: %v", *respBody)
			os.Exit(1)
		}

		token := os.Getenv("GITHUB_PAT_TOKEN")
		if token == "" {
			log.Printf("missing variable: GITHUB_PAT_TOKEN")
			os.Exit(1)
		}
		client := github.NewClient(nil).WithAuthToken(token)
		githubUrl := spec.VCS.GithubEnterpriseHostname
		if githubUrl != "" {
			githubEnterpriseBaseUrl := fmt.Sprintf("https://%v/api/v3/", githubUrl)
			githubEnterpriseUploadUrl := fmt.Sprintf("https://%v/api/uploads/", githubUrl)
			client, err = client.WithEnterpriseURLs(githubEnterpriseBaseUrl, githubEnterpriseUploadUrl)
			if err != nil {
				log.Printf("could not instantiate github enterprise url: %v", err)
				os.Exit(1)
			}
		}

		repoOwner, repoName, _ := strings.Cut(repoFullname, "/")
		repository, _, err := client.Repositories.Get(context.Background(), repoOwner, repoName)
		if err != nil {
			log.Fatalf("Failed to get repository: %v", err)
		}

		inputs := orchestrator_scheduler.WorkflowInput{
			Spec:    string(specBytes),
			RunName: fmt.Sprintf("digger %v manual run by %v", command, spec.VCS.Actor),
		}
		_, err = client.Actions.CreateWorkflowDispatchEventByFileName(context.Background(), spec.VCS.RepoOwner, spec.VCS.RepoName, spec.VCS.WorkflowFile, github.CreateWorkflowDispatchEventRequest{
			Ref:    *repository.DefaultBranch,
			Inputs: inputs.ToMap(),
		})

		if err != nil {
			log.Printf("error while triggering workflow: %v", err)
		} else {
			log.Printf("workflow has triggered successfully! waiting for results ...")
		}

		var logsUrl *string
		var runId *int64
		var jobId *int64
		for {
			runId, jobId, logsUrl, err = GetWorkflowIdAndUrlFromDiggerJobId(client, repoOwner, repoName, spec.JobId)
			if err == nil {
				break
			}
			time.Sleep(time.Second * 1)
		}

		log.Printf("waiting for logs to be available, you can view job in this url: %v runId %v", *logsUrl, *runId)
		log.Printf("......")

		for {
			j, _, err := client.Actions.GetWorkflowJobByID(context.Background(), repoOwner, repoName, *jobId)
			if err != nil {
				log.Printf("GetWorkflowJobByID error: %v please view the logs in the job directly", err)
				os.Exit(1)
			}
			if *j.Status == "completed" {
				break
			}
			time.Sleep(time.Second * 1)
		}

		logs, _, err := client.Actions.GetWorkflowJobLogs(context.Background(), repoOwner, repoName, *jobId, 1)

		log.Printf("streaming logs from remote job:")
		logsContent, err := GetUrlContents(logs.String())

		if err != nil {
			log.Printf("error while fetching logs: %v", err)
			os.Exit(1)
		}
		cleanedLogs := cleanupDiggerOutput(logsContent)
		log.Printf("logsContent is: %v", cleanedLogs)
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
