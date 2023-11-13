package reporting

import (
	"testing"
	"time"

	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/pkg/core/utils"

	"github.com/stretchr/testify/assert"
)

func TestCommentPerRunStrategyReport(t *testing.T) {
	timeOfRun := time.Now()
	strategy := CommentPerRunStrategy{
		TimeOfRun: timeOfRun,
	}
	existingCommentForOtherRun := utils.AsCollapsibleComment("Digger run report at some other time")("")

	prNumber := 1
	ciService := &MockCiService{
		CommentsPerPr: map[int][]*orchestrator.Comment{
			prNumber: {
				{
					Id:   1,
					Body: &existingCommentForOtherRun,
				},
			},
		},
	}

	report := "resource \"null_resource\" \"test\" {}"
	reportFormatter := utils.GetTerraformOutputAsCollapsibleComment("run1")
	err := strategy.Report(ciService, prNumber, report, reportFormatter, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	report2 := "resource \"null_resource\" \"test\" {}"
	reportFormatter2 := utils.GetTerraformOutputAsCollapsibleComment("run2")
	err = strategy.Report(ciService, prNumber, report2, reportFormatter2, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	report3 := "resource \"null_resource\" \"test\" {}"
	reportFormatter3 := utils.GetTerraformOutputAsCollapsibleComment("run3")
	err = strategy.Report(ciService, prNumber, report3, reportFormatter3, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	report4 := "resource \"null_resource\" \"test\" {}"
	reportFormatter4 := utils.GetTerraformOutputAsCollapsibleComment("run4")
	err = strategy.Report(ciService, prNumber, report4, reportFormatter4, true)

	if err != nil {
		t.Errorf("Error: %v", err)
	}

	assert.Equal(t, 2, len(ciService.CommentsPerPr[prNumber]))
	assert.Equal(t, "<details><summary>Digger run report at "+timeOfRun.Format("2006-01-02 15:04:05 (MST)")+"</summary>\n        <details><summary>run1</summary>\n  \n```terraform\nresource \"null_resource\" \"test\" {}\n  ```\n</details>\n\n<details><summary>run2</summary>\n  \n```terraform\nresource \"null_resource\" \"test\" {}\n  ```\n</details>\n\n\n<details><summary>run3</summary>\n  \n```terraform\nresource \"null_resource\" \"test\" {}\n  ```\n</details>\n\n\n<details><summary>run4</summary>\n  \n```terraform\nresource \"null_resource\" \"test\" {}\n  ```\n</details>\n\n</details>", *ciService.CommentsPerPr[prNumber][1].Body)
}

func TestLatestCommentStrategyReport(t *testing.T) {
	timeOfRun := time.Now()
	strategy := LatestRunCommentStrategy{
		TimeOfRun: timeOfRun,
	}
	existingCommentForOtherRun := utils.AsCollapsibleComment("Digger run report at some other time")("")

	prNumber := 1
	ciService := &MockCiService{
		CommentsPerPr: map[int][]*orchestrator.Comment{
			prNumber: {
				{
					Id:   1,
					Body: &existingCommentForOtherRun,
				},
			},
		},
	}

	report := "resource \"null_resource\" \"test\" {}"
	reportFormatter := utils.GetTerraformOutputAsCollapsibleComment("run1")
	err := strategy.Report(ciService, prNumber, report, reportFormatter, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	report2 := "resource \"null_resource\" \"test\" {}"
	reportFormatter2 := utils.GetTerraformOutputAsCollapsibleComment("run2")
	err = strategy.Report(ciService, prNumber, report2, reportFormatter2, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	report3 := "resource \"null_resource\" \"test\" {}"
	reportFormatter3 := utils.GetTerraformOutputAsCollapsibleComment("run3")
	err = strategy.Report(ciService, prNumber, report3, reportFormatter3, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	report4 := "resource \"null_resource\" \"test\" {}"
	reportFormatter4 := utils.GetTerraformOutputAsCollapsibleComment("run4")
	err = strategy.Report(ciService, prNumber, report4, reportFormatter4, true)

	if err != nil {
		t.Errorf("Error: %v", err)
	}

	assert.Equal(t, 2, len(ciService.CommentsPerPr[prNumber]))
	assert.Equal(t, "<details><summary>Digger latest run report</summary>\n        <details><summary>run1</summary>\n  \n```terraform\nresource \"null_resource\" \"test\" {}\n  ```\n</details>\n\n<details><summary>run2</summary>\n  \n```terraform\nresource \"null_resource\" \"test\" {}\n  ```\n</details>\n\n\n<details><summary>run3</summary>\n  \n```terraform\nresource \"null_resource\" \"test\" {}\n  ```\n</details>\n\n\n<details><summary>run4</summary>\n  \n```terraform\nresource \"null_resource\" \"test\" {}\n  ```\n</details>\n\n</details>", *ciService.CommentsPerPr[prNumber][1].Body)
}

func TestMultipleCommentStrategyReport(t *testing.T) {
	strategy := MultipleCommentsStrategy{}
	existingCommentForOtherRun := utils.AsCollapsibleComment("Digger run report at some other time")("")

	prNumber := 1
	ciService := &MockCiService{
		CommentsPerPr: map[int][]*orchestrator.Comment{
			prNumber: {
				{
					Id:   1,
					Body: &existingCommentForOtherRun,
				},
			},
		},
	}

	report := "resource \"null_resource\" \"test\" {}"
	reportFormatter := utils.GetTerraformOutputAsCollapsibleComment("run1")
	err := strategy.Report(ciService, prNumber, report, reportFormatter, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	report2 := "resource \"null_resource\" \"test\" {}"
	reportFormatter2 := utils.GetTerraformOutputAsCollapsibleComment("run2")
	err = strategy.Report(ciService, prNumber, report2, reportFormatter2, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	report3 := "resource \"null_resource\" \"test\" {}"
	reportFormatter3 := utils.GetTerraformOutputAsCollapsibleComment("run3")
	err = strategy.Report(ciService, prNumber, report3, reportFormatter3, true)
	if err != nil {
		t.Errorf("Error: %v", err)
	}
	report4 := "resource \"null_resource\" \"test\" {}"
	reportFormatter4 := utils.GetTerraformOutputAsCollapsibleComment("run4")
	err = strategy.Report(ciService, prNumber, report4, reportFormatter4, true)

	if err != nil {
		t.Errorf("Error: %v", err)
	}

	assert.Equal(t, 5, len(ciService.CommentsPerPr[prNumber]))
	assert.Equal(t, "<details><summary>run4</summary>\n  \n```terraform\nresource \"null_resource\" \"test\" {}\n  ```\n</details>", *ciService.CommentsPerPr[prNumber][4].Body)
}

type MockCiService struct {
	CommentsPerPr map[int][]*orchestrator.Comment
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
func (t MockCiService) PublishComment(prNumber int, comment string) error {

	latestId := 0

	for _, comments := range t.CommentsPerPr {
		for _, c := range comments {
			if c.Id.(int) > latestId {
				latestId = c.Id.(int)
			}
		}
	}

	t.CommentsPerPr[prNumber] = append(t.CommentsPerPr[prNumber], &orchestrator.Comment{Id: latestId + 1, Body: &comment})

	return nil
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

func (t MockCiService) GetComments(prNumber int) ([]orchestrator.Comment, error) {
	comments := []orchestrator.Comment{}
	for _, c := range t.CommentsPerPr[prNumber] {
		comments = append(comments, *c)
	}
	return comments, nil
}

func (t MockCiService) EditComment(prNumber int, commentId interface{}, comment string) error {
	for _, comments := range t.CommentsPerPr {
		for _, c := range comments {
			if c.Id == commentId {
				c.Body = &comment
				return nil
			}
		}
	}
	return nil
}

func (t MockCiService) GetBranchName(prNumber int) (string, error) {
	return "", nil
}
