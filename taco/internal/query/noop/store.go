package noop

// noop store
// basically allows graceful fallback if someone configures for no sqlite 

type NoOpQueryStore struct{}

func NewNoOpQueryStore() *NoOpQueryStore {
    return &NoOpQueryStore{}
}

func (n *NoOpQueryStore) Close() error {
    return nil
}

func (n *NoOpQueryStore) IsEnabled() bool {
	// Not NOOP ? 
    return false
}
