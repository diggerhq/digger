package domain

import "fmt"

// DomainError represents a domain-specific error
type DomainError struct {
	Code    string
	Message string
	Details map[string]interface{}
}

func (e *DomainError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Common error codes
const (
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeAlreadyExists  = "ALREADY_EXISTS"
	ErrCodeLockConflict   = "LOCK_CONFLICT"
	ErrCodeInvalidInput   = "INVALID_INPUT"
	ErrCodeInternalError  = "INTERNAL_ERROR"
)

// NewNotFoundError creates a not found error
func NewNotFoundError(resource string, id string) *DomainError {
	return &DomainError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("%s not found: %s", resource, id),
		Details: map[string]interface{}{
			"resource": resource,
			"id":       id,
		},
	}
}

// NewAlreadyExistsError creates an already exists error
func NewAlreadyExistsError(resource string, id string) *DomainError {
	return &DomainError{
		Code:    ErrCodeAlreadyExists,
		Message: fmt.Sprintf("%s already exists: %s", resource, id),
		Details: map[string]interface{}{
			"resource": resource,
			"id":       id,
		},
	}
}

// NewLockConflictError creates a lock conflict error
func NewLockConflictError(unitID string, lockID string) *DomainError {
    return &DomainError{
        Code:    ErrCodeLockConflict,
        Message: fmt.Sprintf("lock conflict on unit %s", unitID),
        Details: map[string]interface{}{
            "unit_id": unitID,
            "lock_id":  lockID,
        },
    }
}

// NewInvalidInputError creates an invalid input error
func NewInvalidInputError(field string, reason string) *DomainError {
	return &DomainError{
		Code:    ErrCodeInvalidInput,
		Message: fmt.Sprintf("invalid %s: %s", field, reason),
		Details: map[string]interface{}{
			"field":  field,
			"reason": reason,
		},
	}
}
