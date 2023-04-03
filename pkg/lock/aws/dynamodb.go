package aws

// TODO: to reimplement
type DynamoDB struct{}

func (db *DynamoDB) Lock() error
func (db *DynamoDB) Unlock() error
func (db *DynamoDB) Get() (bool, string, error)
