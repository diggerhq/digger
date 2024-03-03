package drift

type Notification interface {
	Send(projectName string, plan string) error
}
