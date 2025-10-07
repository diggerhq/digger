package types

import (
	"errors"
)



var (
	ErrNotSupported = errors.New("Query operation not supported by this backend")
	ErrNotFound = errors.New("Not found")
)
