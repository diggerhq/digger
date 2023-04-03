package gcp

// TODO: to reimplement
type Storage struct{}

func (s *Storage) Lock() error                { return nil }
func (s *Storage) Unlock() error              { return nil }
func (s *Storage) Get() (bool, string, error) { return false, "", nil }
