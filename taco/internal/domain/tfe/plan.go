package tfe

type PlanRecord struct {
	ID string `jsonapi:"primary,plans" json:"id"`

	// ----- attributes -----
	Status               string `jsonapi:"attr,status" json:"status"`
	ResourceAdditions    int    `jsonapi:"attr,resource-additions" json:"resource-additions"`
	ResourceChanges      int    `jsonapi:"attr,resource-changes" json:"resource-changes"`
	ResourceDestructions int    `jsonapi:"attr,resource-destructions" json:"resource-destructions"`
	LogReadURL           string `jsonapi:"attr,log-read-url" json:"log-read-url"`
	HasChanges           bool   `jsonapi:"attr,has-changes" json:"has-changes"`

	// ----- relationships -----
	Run *RunRef `jsonapi:"relation,run" json:"run"`
}

// Minimal run ref for the relationship
type RunRef struct {
	ID string `jsonapi:"primary,runs" json:"id"`
}
