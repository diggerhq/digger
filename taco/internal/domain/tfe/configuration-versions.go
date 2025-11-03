package tfe

// ConfigurationVersionRecord is the "data" object Terraform CLI expects back
// from POST /workspaces/:workspace_id/configuration-versions
//
// It will be wrapped by jsonapi.MarshalPayload(...) into:
// { "data": { "id": "...", "type": "configuration-versions", "attributes": {...}, "relationships": {...} } }
type ConfigurationVersionRecord struct {
	ID string `jsonapi:"primary,configuration-versions" json:"id"`

	// ---------- attributes ----------
	AutoQueueRuns    bool              `jsonapi:"attr,auto-queue-runs" json:"auto-queue-runs"`
	Error            *string           `jsonapi:"attr,error" json:"error"`
	ErrorMessage     *string           `jsonapi:"attr,error-message" json:"error-message"`
	Source           string            `jsonapi:"attr,source" json:"source"`
	Speculative      bool              `jsonapi:"attr,speculative" json:"speculative"`
	Status           string            `jsonapi:"attr,status" json:"status"`
	StatusTimestamps map[string]string `jsonapi:"attr,status-timestamps" json:"status-timestamps"`
	UploadURL        *string            `jsonapi:"attr,upload-url" json:"upload-url"`
	Provisional      bool              `jsonapi:"attr,provisional" json:"provisional"`
	IngressAttributes *IngressAttributesStub `jsonapi:"relation,ingress-attributes" json:"ingress-attributes"`
}


type IngressAttributesStub struct {
}
