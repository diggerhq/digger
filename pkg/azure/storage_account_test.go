package azure

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

const (
	AZURITE_CONN_STRING = "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;TableEndpoint=http://127.0.0.1:10002/devstoreaccount1;"
	AZURITE_SHARED_KEY  = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
)

var (
	envToClean = []string{
		"DIGGER_AZURE_CONNECTION_STRING",
		"DIGGER_AZURE_SHARED_KEY",
		"DIGGER_AZURE_SA_NAME",
	}
)

type SALockTestSuite struct {
	suite.Suite
}

// Run once before the test suite starts
func (suite *SALockTestSuite) SetupSuite() {
	// We set 'testingMode' so we can use the right service url format
	testingMode = true
}

// Runs after every test
func (suite *SALockTestSuite) TearDownTest() {
	// Clean environment variables
	for _, env := range envToClean {
		os.Setenv(env, "")
	}
}

func (suite *SALockTestSuite) TestNewStorageAccountLock() {
	loadConnStringEnv()

	sal, err := NewStorageAccountLock()
	suite.NotNil(sal)
	suite.NoError(err)
}

func (suite *SALockTestSuite) TestNewStorageAccountLock_NoAuthMethods() {
	sal, err := NewStorageAccountLock()

	suite.Nil(sal)
	suite.Error(err, "expected an error, since no authentication mecanism was provided")
}

func (suite *SALockTestSuite) TestNewStorageAccountLock_WithSharedKey() {
	loadSharedKeyEnv()
	os.Setenv("DIGGER_AZURE_SA_NAME", "devstoreaccount1")
	sal, err := NewStorageAccountLock()

	suite.NotNil(sal)
	suite.NoError(err)
}

func (suite *SALockTestSuite) TestNewStorageAccountLock_WithSharedKey_MissingAccountName() {
	loadSharedKeyEnv()
	sal, err := NewStorageAccountLock()
	suite.Nil(sal)
	suite.Error(err, "should have got an error")
}

func (suite *SALockTestSuite) TestLock_WhenNotLockedYet() {
	loadConnStringEnv()
	sal, _ := NewStorageAccountLock()
	ok, err := sal.Lock(18, generateResourceName())

	suite.True(ok, "lock acquisition should be true")
	suite.NoError(err, "error while acquiring lock")
}

func (suite *SALockTestSuite) TestLock_WhenNotLockedYet_WithSharedKey() {
	loadSharedKeyEnv()
	os.Setenv("DIGGER_AZURE_SA_NAME", "devstoreaccount1")
	sal, _ := NewStorageAccountLock()
	ok, err := sal.Lock(18, generateResourceName())

	suite.True(ok, "lock acquisition should be true")
	suite.NoError(err, "error while acquiring lock")
}

func (suite *SALockTestSuite) TestLock_WhenAlreadyLocked() {
	loadConnStringEnv()
	sal, _ := NewStorageAccountLock()
	resourceName := generateResourceName()

	// Locking the first time
	ok, err := sal.Lock(18, resourceName)
	suite.True(ok, "lock acquisition should be true")
	suite.NoError(err, "should not have got an error")

	// Lock the second time on the same resource name
	ok, err = sal.Lock(18, resourceName)
	suite.False(ok, "lock acquisition should be false")
	suite.NoError(err, "should not have got an error")
}

func (suite *SALockTestSuite) TestUnlock() {
	loadConnStringEnv()
	sal, _ := NewStorageAccountLock()
	resourceName := generateResourceName()

	// Locking the first time
	ok, err := sal.Lock(18, resourceName)
	suite.True(ok, "lock acquisition should be true")
	suite.NoError(err, "should not have got an error")

	// Lock the second time on the same resource name
	ok, err = sal.Unlock(resourceName)
	suite.True(ok)
	suite.NoError(err, "should not have got an error")
}

func (suite *SALockTestSuite) TestGetLock() {
	loadConnStringEnv()
	sal, _ := NewStorageAccountLock()
	resourceName := generateResourceName()

	// Locking
	ok, err := sal.Lock(21, resourceName)
	suite.True(ok, "lock acquisition should be true")
	suite.NoError(err, "should not have got an error")

	// Get the lock
	transactionId, err := sal.GetLock(resourceName)
	suite.Equal(21, *transactionId, "transaction id mismatch")
	suite.NoError(err, "should not have got an error")
}

func (suite *SALockTestSuite) TestGetLock_LockDoesNotExist() {
	loadConnStringEnv()
	sal, _ := NewStorageAccountLock()
	resourceName := generateResourceName()

	// Get a lock that doesn't exist
	transactionId, err := sal.GetLock(resourceName)
	suite.Nil(transactionId, "transaction id should be nil")
	suite.NoError(err, "should not have got an error")
}

func TestStorageAccountLockTestSuite(t *testing.T) {
	SkipCI(t)
	suite.Run(t, new(SALockTestSuite))
}

func SkipCI(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping testing in CI environment")
	}
}

// Generate unique string to be used as resource name
// so we don't have to delete the entity
func generateResourceName() string {
	return uuid.New().String()
}

func loadSharedKeyEnv() {
	os.Setenv("DIGGER_AZURE_SHARED_KEY", AZURITE_SHARED_KEY)

}

func loadConnStringEnv() {
	os.Setenv("DIGGER_AZURE_CONNECTION_STRING", AZURITE_CONN_STRING)
}
