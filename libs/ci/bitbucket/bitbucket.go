package bitbucket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

// Define the base URL for the Bitbucket API.
const bitbucketBaseURL = "https://api.bitbucket.org/2.0"

// BitbucketAPI is a struct that holds the required authentication information.
type BitbucketAPI struct {
	AuthToken     string
	HttpClient    http.Client
	RepoWorkspace string
	RepoName      string
}

func (b BitbucketAPI) sendRequest(method, url string, body []byte) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b.AuthToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type DiffStat struct {
	Values []struct {
		Status string `json:"status"`
		Old    struct {
			Path string `json:"path"`
		} `json:"old,omitempty"`
		New struct {
			Path string `json:"path"`
		} `json:"new,omitempty"`
	} `json:"values"`
	Next string `json:"next,omitempty"`
}

func (b BitbucketAPI) GetChangedFiles(prNumber int) ([]string, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/diffstat", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	resp, err := b.sendRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get changed files. Status code: %d", resp.StatusCode)
	}
	diffStat := &DiffStat{}
	err = json.NewDecoder(resp.Body).Decode(diffStat)
	if err != nil {
		return nil, err
	}
	var files []string

	for _, v := range diffStat.Values {
		files = append(files, v.Old.Path)
	}
	return files, nil
}

type BitbucketCommentResponse struct {
	ID      int `json:"id"`
	Content struct {
		Raw string `json:"raw"`
	} `json:"content"`
	Links struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
	CreatedOn string `json:"created_on"`
	User      struct {
		DisplayName string `json:"display_name"`
		UUID        string `json:"uuid"`
	} `json:"user"`
}

func (b BitbucketAPI) PublishComment(prNumber int, comment string) (*ci.Comment, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/comments", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	commentBody := map[string]interface{}{
		"content": map[string]string{
			"raw": comment,
		},
	}

	commentJSON, err := json.Marshal(commentBody)
	if err != nil {
		return nil, err
	}

	resp, err := b.sendRequest("POST", url, commentJSON)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to publish comment. Status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	var commentResponse BitbucketCommentResponse
	if err := json.Unmarshal(body, &commentResponse); err != nil {
		fmt.Println("Error parsing response:", err)
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	res := ci.Comment{
		Id:           strconv.Itoa(commentResponse.ID),
		DiscussionId: "",
		Body:         &comment,
		Url:          commentResponse.Links.HTML.Href,
	}
	return &res, nil
}

func (svc BitbucketAPI) ListIssues() ([]*ci.Issue, error) {
	return nil, fmt.Errorf("implement me")
}

func (svc BitbucketAPI) PublishIssue(title string, body string, labels *[]string) (int64, error) {
	return 0, fmt.Errorf("implement me")
}

func (svc BitbucketAPI) UpdateIssue(ID int64, title string, body string) (int64, error) {
	return 0, fmt.Errorf("implement me")
}

func (b BitbucketAPI) EditComment(prNumber int, id string, comment string) error {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/comments/%s", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber, id)

	commentBody := map[string]interface{}{
		"content": map[string]string{
			"raw": comment,
		},
	}

	commentJSON, err := json.Marshal(commentBody)
	if err != nil {
		return err
	}

	resp, err := b.sendRequest("PUT", url, commentJSON)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to edit comment. Status code: %d", resp.StatusCode)
	}

	return nil
}

func (a BitbucketAPI) CreateCommentReaction(id string, reaction string) error {
	// TODO implement me
	return nil
}

type Comment struct {
	Size     int    `json:"size"`
	Page     int    `json:"page"`
	Pagelen  int    `json:"pagelen"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Values   []struct {
		Id      int `json:"id"`
		Content struct {
			Raw string `json:"raw"`
		}
	} `json:"values"`
}

func (b BitbucketAPI) GetComments(prNumber int) ([]ci.Comment, error) {

	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/comments", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	resp, err := b.sendRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get comments. Status code: %d", resp.StatusCode)
	}

	var commentResponse Comment
	err = json.NewDecoder(resp.Body).Decode(&commentResponse)
	if err != nil {
		return nil, err
	}

	var comments []ci.Comment

	for _, v := range commentResponse.Values {
		comments = append(comments, ci.Comment{
			Id:   strconv.Itoa(v.Id),
			Body: &v.Content.Raw,
		})
	}

	return comments, nil

}

func (svc BitbucketAPI) GetApprovals(prNumber int) ([]string, error) {
	approvals := make([]string, 0)
	// TODO: implement me
	return approvals, nil
}

type PullRequest struct {
	Id     int `json:"id"`
	Source struct {
		Commit struct {
			Hash string `json:"hash"`
		}
	}
}

func (b BitbucketAPI) SetStatus(prNumber int, status string, statusContext string) error {
	prUrl := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	resp, err := b.sendRequest("GET", prUrl, nil)
	if err != nil {
		return fmt.Errorf("failed to get pull request. Status code: %d", resp.StatusCode)
	}

	var prResponse PullRequest
	err = json.NewDecoder(resp.Body).Decode(&prResponse)

	url := fmt.Sprintf("%s/repositories/%s/%s/commit/%s/statuses/build", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prResponse.Source.Commit.Hash)

	if status == "failure" {
		status = "FAILED"
	} else if status == "success" {
		status = "SUCCESSFUL"
	} else if status == "pending" {
		status = "INPROGRESS"
	}

	statusBody := map[string]interface{}{
		"state": status,
		"key":   statusContext,
		"url":   prUrl,
	}

	statusJSON, err := json.Marshal(statusBody)
	if err != nil {
		return err
	}

	resp, err = b.sendRequest("POST", url, statusJSON)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body := &bytes.Buffer{}
		_, err := body.ReadFrom(resp.Body)
		if err != nil {
			fmt.Printf("failed to read response body: %v", err)
		}
		fmt.Printf("failed to set status. Status code: %d. Response: %s", resp.StatusCode, body.String())
		return fmt.Errorf("failed to set status. Status code: %d", resp.StatusCode)
	}

	return nil
}

type CommitStatuses struct {
	Size     int    `json:"size"`
	Page     int    `json:"page"`
	Pagelen  int    `json:"pagelen"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Values   []struct {
		State     string    `json:"state"`
		Key       string    `json:"key"`
		UpdatedOn time.Time `json:"updated_on"`
	} `json:"values"`
}

func (b BitbucketAPI) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/commit/%d/statuses", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	resp, err := b.sendRequest("GET", url, nil)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get combined status. Status code: %d", resp.StatusCode)
	}

	var statuses CommitStatuses

	err = json.NewDecoder(resp.Body).Decode(&statuses)

	if err != nil {
		return "", err
	}
	// group by key and get latest per key

	type status struct {
		State     string
		UpdatedOn time.Time
	}
	latestStatusByKey := make(map[string]status)

	for _, v := range statuses.Values {
		currentlyKnownStatus, ok := latestStatusByKey[v.Key]
		if !ok {
			latestStatusByKey[v.Key] = status{
				State:     v.State,
				UpdatedOn: v.UpdatedOn,
			}
			continue
		}
		if currentlyKnownStatus.UpdatedOn.Before(v.UpdatedOn) {
			latestStatusByKey[v.Key] = status{
				State:     v.State,
				UpdatedOn: v.UpdatedOn,
			}
		}
	}
	for _, status := range latestStatusByKey {
		if status.State == "FAILED" {
			return "failure", nil
		}
	}

	var allSuccess = true
	for _, status := range latestStatusByKey {
		if status.State != "SUCCESSFUL" {
			allSuccess = false
			break
		}
	}
	if allSuccess {
		return "success", nil
	}

	return "pending", nil

}

func (b BitbucketAPI) MergePullRequest(prNumber int, mergeStrategy string) error {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d/merge", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	resp, err := b.sendRequest("POST", url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to merge pull request. Status code: %d", resp.StatusCode)
	}

	return nil
}

func (b BitbucketAPI) IsMergeable(prNumber int) (bool, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	resp, err := b.sendRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to get pull request. Status code: %d", resp.StatusCode)
	}

	var pullRequest struct {
		State string `json:"state"`
	}

	err = json.NewDecoder(resp.Body).Decode(&pullRequest)
	if err != nil {
		return false, err
	}

	return pullRequest.State == "OPEN", nil
}

func (b BitbucketAPI) IsMerged(prNumber int) (bool, error) {

	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	resp, err := b.sendRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to get pull request. Status code: %d", resp.StatusCode)
	}

	var pullRequest struct {
		State string `json:"state"`
	}

	err = json.NewDecoder(resp.Body).Decode(&pullRequest)
	if err != nil {
		return false, err
	}

	return pullRequest.State == "MERGED", nil
}

func (b BitbucketAPI) IsClosed(prNumber int) (bool, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	resp, err := b.sendRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to get pull request. Status code: %d", resp.StatusCode)
	}

	var pullRequest struct {
		State string `json:"state"`
	}

	err = json.NewDecoder(resp.Body).Decode(&pullRequest)
	if err != nil {
		return false, err
	}

	return pullRequest.State != "OPEN", nil
}

func (b BitbucketAPI) GetBranchName(prNumber int) (string, string, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%d", bitbucketBaseURL, b.RepoWorkspace, b.RepoName, prNumber)

	resp, err := b.sendRequest("GET", url, nil)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to get pull request. Status code: %d", resp.StatusCode)
	}

	var pullRequest struct {
		Source struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
		} `json:"source"`
	}

	err = json.NewDecoder(resp.Body).Decode(&pullRequest)
	if err != nil {
		return "", "", err
	}

	return pullRequest.Source.Branch.Name, "", nil
}

func (svc BitbucketAPI) SetOutput(prNumber int, key string, value string) error {
	//TODO implement me
	return nil
}

// Implement the OrgService interface.

func (b BitbucketAPI) GetUserTeams(organisation string, user string) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

type PipelineResponse struct {
	UUID        string `json:"uuid"`
	BuildNumber int    `json:"build_number"`
	CreatedOn   string `json:"created_on"`
	Creator     struct {
		DisplayName string `json:"display_name"`
		UUID        string `json:"uuid"`
		AccountID   string `json:"account_id"`
		Nickname    string `json:"nickname"`
		Type        string `json:"type"`
		Links       struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"links"`
	} `json:"creator"`
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		UUID     string `json:"uuid"`
		Type     string `json:"type"`
		Links    struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"links"`
	} `json:"repository"`
	Target struct {
		Type     string `json:"type"`
		RefName  string `json:"ref_name"`
		RefType  string `json:"ref_type"`
		Selector struct {
			Type string `json:"type"`
		} `json:"selector"`
		Commit struct {
			Type string `json:"type"`
			Hash string `json:"hash"`
		} `json:"commit"`
	} `json:"target"`
	Trigger struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"trigger"`
	State struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Stage struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"stage"`
	} `json:"state"`
	Variables []struct {
		Key     string `json:"key"`
		Value   string `json:"value"`
		Secured bool   `json:"secured"`
		UUID    string `json:"uuid"`
	} `json:"variables"`
	Links struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}

// trigger pipeline from a specific branch
func (b BitbucketAPI) TriggerPipeline(branch string, variables []interface{}) (string, error) {
	url := fmt.Sprintf("%s/repositories/%s/%s/pipelines", bitbucketBaseURL, b.RepoWorkspace, b.RepoName)

	triggerOptions := map[string]interface{}{
		"target": map[string]interface{}{
			"ref_type": "branch",
			"type":     "pipeline_ref_target",
			"ref_name": branch,
			"selector": map[string]interface{}{
				"type":    "custom",
				"pattern": "digger",
			},
		},
		"variables": variables,
	}

	triggerJSON, err := json.Marshal(triggerOptions)
	if err != nil {
		return "", err
	}

	resp, err := b.sendRequest("POST", url, triggerJSON)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to trigger pipeline: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return "", fmt.Errorf("error reading response: %v", err)
	}

	var triggerPipelineResponse PipelineResponse
	if err := json.Unmarshal(body, &triggerPipelineResponse); err != nil {
		fmt.Println("Error parsing response:", err)
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	return "", nil

}

func FindImpactedProjectsInBitbucket(diggerConfig *configuration.DiggerConfig, prNumber int, prService ci.PullRequestService) ([]configuration.Project, error) {
	changedFiles, err := prService.GetChangedFiles(prNumber)

	if err != nil {
		fmt.Printf("Error getting changed files: %v", err)
		return nil, err
	}

	impactedProjects, _ := diggerConfig.GetModifiedProjects(changedFiles)
	return impactedProjects, nil
}
