package drift

// LastChange describes the most recent commit that touched a project's
// directory, so drift notifications can point at who changed the code last.
type LastChange struct {
	Author string
	Commit string
	When   string
}

type Notification interface {
	SendNotificationForProject(projectName string, repoFullName string, plan string, lastChange *LastChange) error
	SendErrorNotificationForProject(projectName string, repoFullName string, err error) error
	Flush() error
}
