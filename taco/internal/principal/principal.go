package principal

// Principal represents the identity of an authenticated user or service.
type Principal struct {
	Subject string
	Email   string
	Roles   []string
	Groups  []string
}
