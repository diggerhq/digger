package locking

type Lock interface {
	Lock(transactionId int, resource string) (bool, error)
	Unlock(resource string) (bool, error)
	GetLock(resource string) (*int, error)
}

type ProjectLock interface {
	Lock() (bool, error)
	Unlock() (bool, error)
	ForceUnlock() error
	LockId() string
}
