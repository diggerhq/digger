package domain

type CIRunner interface {
	CurrentEvent() (*Event, error)
}
