package domain

type SCMProvider interface {
	PublishComment(string, PRDetails) error
}
