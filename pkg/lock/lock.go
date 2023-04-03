package lock

import (
	"digger/pkg/domain"
	"digger/pkg/lock/aws"
	"digger/pkg/lock/azure"
	"digger/pkg/lock/gcp"
	"errors"
	"os"
)

var ErrLockProviderNotValid = errors.New("lock provider is not valid")

// TODO: pass in the right paramters to the structs when ready
func GetProvider() (domain.LockProvider, error) {
	provider := os.Getenv("DIGGER_LOCK_PROVIDER")
	if provider == "aws" {
		return &aws.DynamoDB{}, nil
	}

	if provider == "gcp" {
		return &gcp.Storage{}, nil
	}

	if provider == "azure" {
		return &azure.StorageAccount{}, nil
	}

	return nil, ErrLockProviderNotValid
}
