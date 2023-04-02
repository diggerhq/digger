package domain

type LockProvider interface {
	// TODO: add the right signature
	Lock() error
	Unlock() error
	Get() (bool, string, error)
}
