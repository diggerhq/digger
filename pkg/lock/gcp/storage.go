package gcp

// TODO: to reimplement
type Storage struct{}

func (s *Storage) Lock() error
func (s *Storage) Unlock() error
func (s *Storage) Get() (bool, string, error)
