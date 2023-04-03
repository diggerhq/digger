package aws

// TODO: to reimplement
type DynamoDB struct{}

func (db *DynamoDB) Lock() error                { return nil }
func (db *DynamoDB) Unlock() error              { return nil }
func (db *DynamoDB) Get() (bool, string, error) { return false, "", nil }
