package domain

type CIRunner interface {
	CurrentEvent(*DiggerConfig) (*ParsedEvent, error)
}
