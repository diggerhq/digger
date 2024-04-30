package github

import "fmt"

type EventToIgnoreError struct {
	Message string
}

func (e *EventToIgnoreError) Error() string {
	return fmt.Sprintf("Unsupported event: %s", e.Message)
}
