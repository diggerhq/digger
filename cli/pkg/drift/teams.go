package drift

import (
	"fmt"
	"log/slog"
)

type TeamsNotification struct {
	Url string
}

func (teams *TeamsNotification) SendNotificationForProject(projectName string, repoFullName string, plan string) error {
	message := fmt.Sprintf(
		"⚠️ **Infrastructure Drift Detected** ⚠️\n\n"+
			"📁 **Project:** `%s`\n"+
			"📚 **Repository:** `%s`\n\n"+
			"📝 **Terraform Plan:**\n```\n%v\n```\n\n",
		projectName, repoFullName, plan,
	)
	parts := SplitCodeBlocks(message)
	for _, part := range parts {
		err := SendSlackMessage(teams.Url, part)
		if err != nil {
			slog.Error("failed to send teams drift request", "error", err)
			return err
		}
	}

	return nil
}

func (teams *TeamsNotification) SendErrorNotificationForProject(projectName string, repoFullName string, err error) error {
	message := fmt.Sprintf(
		"🚨 **Error While Drift Processing** 🚨\n\n"+
			"📁 **Project:** `%s`\n"+
			"📚 **Repository:** `%s`\n\n"+
			"⚠️ **Error Details:**\n```\n%v\n```\n\n"+
			"_Please check the workflow logs for more information._",
		projectName, repoFullName, err,
	)

	return SendSlackMessage(teams.Url, message)
}

func (teams *TeamsNotification) Flush() error {
	return nil
}
