package tfe

// StateVersionOutput represents a single output value attached to a state
// version in the Terraform Cloud / Enterprise JSON API.
type StateVersionOutput struct {
	// Primary identifier for this output resource.
	ID string `jsonapi:"primary,state-version-outputs" json:"id"`

	// Logical name of the output, e.g. "db_endpoint".
	Name string `jsonapi:"attr,name" json:"name"`

	// If true, the value should be treated as secret/redacted.
	Sensitive bool `jsonapi:"attr,sensitive" json:"sensitive"`

	// Terraform type string for the value, such as "string", "number", "map", etc.
	Type string `jsonapi:"attr,type" json:"type"`

	// Actual value of the output as returned by the API.
	Value interface{} `jsonapi:"attr,value" json:"value"`
}



