package tfe

// ApplyRecord represents a Terraform apply operation
type ApplyRecord struct {
	ID          string `jsonapi:"primary,applies" json:"id"`
	Status      string `jsonapi:"attr,status" json:"status"`
	LogReadURL  string `jsonapi:"attr,log-read-url" json:"log-read-url"`
	
	// Relationship to run (RunRef is declared in plan.go)
	Run *RunRef `jsonapi:"relation,run,omitempty" json:"run,omitempty"`
}

