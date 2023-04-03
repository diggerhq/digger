package azure

// TODO: to implement
type StorageAccount struct{}

func (sa *StorageAccount) Lock() error
func (sa *StorageAccount) Unlock() error
func (sa *StorageAccount) Get() (bool, string, error)
