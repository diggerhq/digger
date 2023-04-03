package azure

// TODO: to implement
type StorageAccount struct{}

func (sa *StorageAccount) Lock() error                { return nil }
func (sa *StorageAccount) Unlock() error              { return nil }
func (sa *StorageAccount) Get() (bool, string, error) { return false, "", nil }
