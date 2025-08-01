package drift

type Notification interface {
	SendNotificationForProject(projectName string, repoFullName string, plan string) error
	SendErrorNotificationForProject(projectName string, repoFullName string, err error) error
	Flush() error
}
