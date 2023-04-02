package domain

type EventParser interface {
	Parse(Event, DiggerConfig) (ParsedEvent, error)
}
