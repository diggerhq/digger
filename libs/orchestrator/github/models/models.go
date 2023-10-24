package models

type EventPackage struct {
	Event      interface{}
	EventName  string
	Actor      string
	Repository string
}
