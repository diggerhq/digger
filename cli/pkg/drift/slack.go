package drift

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

type SlackNotification struct {
	Url string
}

func SplitCodeBlocks(message string) []string {
	var res []string

	if strings.Count(message, "```") < 2 {
		res = append(res, message)
		return res
	}

	regex := regexp.MustCompile("\n")
	split := regex.Split(message, -1)
	part := ""
	for _, line := range split {
		if len(part+line) > 4000 {
			res = append(res, part+"\n"+line+"\n```")
			part = "```\n" + line
		} else {
			part = part + "\n" + line
		}
	}
	if len(part) > 0 {
		res = append(res, part)
	}
	return res
}

func (slack *SlackNotification) SendNotificationForProject(projectName string, repoFullName string, plan string) error {
	message := fmt.Sprintf(
		":warning: *Infrastructure Drift Detected* :warning:\n\n"+
			":file_folder: *Project:* `%s`\n"+
			":books: *Repository:* `%s`\n\n"+
			":memo: *Terraform Plan:*\n```\n%v\n```\n\n",
		projectName, repoFullName, plan,
	)
	parts := SplitCodeBlocks(message)
	for _, part := range parts {
		err := SendSlackMessage(slack.Url, part)
		if err != nil {
			slog.Error("failed to send slack drift request", "error", err)
			return err
		}
	}

	return nil
}

func (slack *SlackNotification) SendErrorNotificationForProject(projectName string, repoFullName string, err error) error {
	message := fmt.Sprintf(
		":rotating_light: *Error While Drift Processing* :rotating_light:\n\n"+
			":file_folder: *Project:* `%s`\n"+
			":books: *Repository:* `%s`\n\n"+
			":warning: *Error Details:*\n```\n%v\n```\n\n"+
			"_Please check the workflow logs for more information._",
		projectName, repoFullName, err,
	)

	return SendSlackMessage(slack.Url, message)
}

func (slack *SlackNotification) Flush() error {
	return nil
}
