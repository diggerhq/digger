package github

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"strconv"
)

type MockCiService struct {
	CommentsPerPr map[int][]*ci.Comment
}

func (t MockCiService) GetUserTeams(organisation string, user string) ([]string, error) {
	return nil, nil
}

func (t MockCiService) GetApprovals(prNumber int) ([]string, error) {
	return []string{}, nil
}

func (t MockCiService) GetChangedFiles(prNumber int) ([]string, error) {
	return nil, nil
}
func (t MockCiService) PublishComment(prNumber int, comment string) (*ci.Comment, error) {

	latestId := 0

	for _, comments := range t.CommentsPerPr {
		for _, c := range comments {
			id, _ := c.GetIdAsInt()
			if id > latestId {
				latestId = id
			}
		}
	}

	t.CommentsPerPr[prNumber] = append(t.CommentsPerPr[prNumber], &ci.Comment{Id: strconv.Itoa(latestId + 1), Body: &comment})

	return &ci.Comment{Id: strconv.Itoa(latestId)}, nil
}

func (t MockCiService) ListIssues() ([]*ci.Issue, error) {
	return nil, fmt.Errorf("implement me")
}

func (t MockCiService) PublishIssue(title string, body string) (int64, error) {
	return 0, fmt.Errorf("implement me")
}

func (t MockCiService) SetStatus(prNumber int, status string, statusContext string) error {
	return nil
}

func (t MockCiService) GetCombinedPullRequestStatus(prNumber int) (string, error) {
	return "", nil
}

func (t MockCiService) MergePullRequest(prNumber int) error {
	return nil
}

func (t MockCiService) IsMergeable(prNumber int) (bool, error) {
	return true, nil
}

func (t MockCiService) IsMerged(prNumber int) (bool, error) {
	return false, nil
}

func (t MockCiService) DownloadLatestPlans(prNumber int) (string, error) {
	return "", nil
}

func (t MockCiService) IsClosed(prNumber int) (bool, error) {
	return false, nil
}

func (t MockCiService) GetComments(prNumber int) ([]ci.Comment, error) {
	comments := []ci.Comment{}
	for _, c := range t.CommentsPerPr[prNumber] {
		comments = append(comments, *c)
	}
	return comments, nil
}

func (t MockCiService) EditComment(prNumber int, id string, comment string) error {
	for _, comments := range t.CommentsPerPr {
		for _, c := range comments {
			if c.Id == id {
				c.Body = &comment
				return nil
			}
		}
	}
	return nil
}

func (t MockCiService) CreateCommentReaction(id string, reaction string) error {
	// TODO implement me
	return nil
}

func (t MockCiService) GetBranchName(prNumber int) (string, string, error) {
	return "", "", nil
}

func (svc MockCiService) SetOutput(prNumber int, key string, value string) error {
	//TODO implement me
	return nil
}
