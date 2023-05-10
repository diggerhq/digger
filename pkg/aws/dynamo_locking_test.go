package aws

import (
	"github.com/stretchr/testify/mock"
	"testing"
)

func TestLockingTwiceThrowsError(t *testing.T) {
	m := mock.Mock{}
	createTableIfNotExists(m)
}
