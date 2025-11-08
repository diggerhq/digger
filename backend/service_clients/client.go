package service_clients

type BackgroundJobTriggerResponse struct {
	ID string `json:"id"`
}
type BackgroundJobsClient interface {
	TriggerProjectsRefreshService(cloneUrl string, branch string, githubToken string, repoFullName string, orgId string) (*BackgroundJobTriggerResponse, error)
}
