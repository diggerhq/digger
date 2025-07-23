package drift

type Notification interface {
	Send(projectName, plan string) error
}
