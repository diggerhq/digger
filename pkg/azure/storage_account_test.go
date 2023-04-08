package azure

import (
	"net"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

// Default values to connect to Azurite
const (
	AZURITE_SA_NAME     = "devstoreaccount1"
	AZURITE_CONN_STRING = "DefaultEndpointsProtocol=http;AccountName=devstoreaccount1;AccountKey=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==;TableEndpoint=http://127.0.0.1:10002/devstoreaccount1;"
	AZURITE_SHARED_KEY  = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
)

var (
	usingRealSA = false
	envToClean  = []string{
		"DIGGER_AZURE_CONNECTION_STRING",
		"DIGGER_AZURE_SHARED_KEY",
		"DIGGER_AZURE_SA_NAME",
		"DIGGER_AZURE_TENANT_ID",
		"DIGGER_AZURE_CLIENT_ID",
		"DIGGER_AZURE_CLIENT_SECRET",
	}

	AZURE_CONN_STRING = ""
	AZURE_SHARED_KEY  = ""
	AZURE_SA_NAME     = ""

	AZURE_TENANT_ID     = ""
	AZURE_CLIENT_ID     = ""
	AZURE_CLIENT_SECRET = ""
)

type tt struct {
	loadEnv func(*SALockTestSuite)
	name    string
}

var (
	testCases = []tt{
		{
			name: "Connection string authentication mode",
			loadEnv: func(*SALockTestSuite) {
				loadConnStringEnv()
			},
		},
		{
			name: "Shared Key authentication mode",
			loadEnv: func(*SALockTestSuite) {
				loadSharedKeyEnv()
			},
		},
		{
			name: "Client secret authentication mode",
			loadEnv: func(s *SALockTestSuite) {
				// Skip this test case if we are not testing on a real storage account
				if !usingRealSA {
					s.T().Skip("Client secret method can only be tested when used against a real storage account.")
				}
				loadClientSecretEnv()
			},
		},
	}
)

type SALockTestSuite struct {
	suite.Suite
}

// Runs once before the test suite starts
func (suite *SALockTestSuite) SetupSuite() {
	// Prepare environment variables
	prepareEnv()

	// Make sure Azurite is started before the tests
	// if we are not using a real storage account
	if usingRealSA {
		return
	}

	conn, err := net.Dial("tcp", "127.0.0.1:10002")
	if err != nil {
		suite.T().Skip("Please make sure 'Azurite' table service is started before running Azure tests, or use a real storage account.")
	}
	conn.Close()
}

// Runs after every test
func (suite *SALockTestSuite) TearDownTest() {
	// Clean environment variables
	for _, env := range envToClean {
		os.Setenv(env, "")
	}
}

func (suite *SALockTestSuite) TestNewStorageAccountLock() {
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.loadEnv(suite)
			sal, err := NewStorageAccountLock()

			suite.NotNil(sal)
			suite.NoError(err)
		})
	}
}

func (suite *SALockTestSuite) TestNewStorageAccountLock_NoAuthMethods() {
	sal, err := NewStorageAccountLock()

	suite.Nil(sal)
	suite.Error(err, "expected an error, since no authentication mecanism was provided")
}

func (suite *SALockTestSuite) TestNewStorageAccountLock_WithSharedKey_MissingAccountName() {
	loadSharedKeyEnv()
	os.Setenv("DIGGER_AZURE_SA_NAME", "")

	sal, err := NewStorageAccountLock()
	suite.Nil(sal)
	suite.Error(err, "should have got an error")
}

func (suite *SALockTestSuite) TestNewStorageAccountLock_WithClientSecret_MissingEnv() {
	loadClientSecretEnv()

	sal, err := NewStorageAccountLock()
	suite.Nil(sal)
	suite.Error(err, "should have got an error")
}

func (suite *SALockTestSuite) TestLock_WhenNotLockedYet() {
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.loadEnv(suite)
			sal, _ := NewStorageAccountLock()
			ok, err := sal.Lock(18, generateResourceName())

			suite.True(ok, "lock acquisition should be true")
			suite.NoError(err, "error while acquiring lock")
		})
	}
}

func (suite *SALockTestSuite) TestLock_WhenAlreadyLocked() {
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.loadEnv(suite)
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
		})
	}
}

// TODO: Add Unlock test case for when lock doesn't exist
func (suite *SALockTestSuite) TestUnlock() {
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.loadEnv(suite)
			sal, _ := NewStorageAccountLock()
			resourceName := generateResourceName()

			// Locking
			ok, err := sal.Lock(18, resourceName)
			suite.True(ok, "lock acquisition should be true")
			suite.NoError(err, "should not have got an error")

			// Unlocking
			ok, err = sal.Unlock(resourceName)
			suite.True(ok)
			suite.NoError(err, "should not have got an error")
		})
	}
}

func (suite *SALockTestSuite) TestGetLock() {
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.loadEnv(suite)
			sal, err := NewStorageAccountLock()
			suite.NotNil(sal)
			suite.NoError(err)

			// Locking
			resourceName := generateResourceName()
			ok, err := sal.Lock(21, resourceName)
			suite.True(ok, "lock acquisition should be true")
			suite.NoError(err, "should not have got an error")

			// Get the lock
			transactionId, err := sal.GetLock(resourceName)
			suite.Equal(21, *transactionId, "transaction id mismatch")
			suite.NoError(err, "should not have got an error")
		})
	}
}

func (suite *SALockTestSuite) TestGetLock_WhenLockDoesNotExist() {
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.loadEnv(suite)
			sal, _ := NewStorageAccountLock()
			resourceName := generateResourceName()

			// Get a lock that doesn't exist
			transactionId, err := sal.GetLock(resourceName)
			suite.Nil(transactionId, "transaction id should be nil")
			suite.NoError(err, "should not have got an error")
		})
	}
}

// Entrypoint of the test suite
func TestStorageAccountLockTestSuite(t *testing.T) {
	// To test Azure Storage account we can either:
	// 1. use a real storage account if we have one
	// 2. or use a local storage account emulator (Azurite)

	useRealSA := os.Getenv("DIGGER_TEST_USE_REAL_SA")

	// Override the service url format
	// if we are not using a real storage account
	// We'll be relying on Azurite emulator which has
	// a different url format
	if useRealSA == "" {
		SERVICE_URL_FORMAT = "http://127.0.0.1:10002/%s"
		usingRealSA = false
		suite.Run(t, new(SALockTestSuite))
		return
	}

	usingRealSA = true
	suite.Run(t, new(SALockTestSuite))
}

// Generate unique string to be used as resource name
// so we don't have to delete the entity
func generateResourceName() string {
	return uuid.New().String()
}

// Initialize and save environment variables
// into local variables.
// This is useful so we can alter  environment variables
// without losing their initial values.

// This comes is handy when we want to test cases
// where an environment variable is not defined for example.
func prepareEnv() {
	if !usingRealSA {
		AZURE_SHARED_KEY = AZURITE_SHARED_KEY
		AZURE_SA_NAME = AZURITE_SA_NAME
		AZURE_CONN_STRING = AZURITE_CONN_STRING
		return
	}

	AZURE_SHARED_KEY = os.Getenv("DIGGER_AZURE_SHARED_KEY")
	AZURE_SA_NAME = os.Getenv("DIGGER_AZURE_SA_NAME")
	AZURE_CONN_STRING = os.Getenv("DIGGER_AZURE_CONNECTION_STRING")

	AZURE_TENANT_ID = os.Getenv("DIGGER_AZURE_TENANT_ID")
	AZURE_CLIENT_ID = os.Getenv("DIGGER_AZURE_CLIENT_ID")
	AZURE_CLIENT_SECRET = os.Getenv("DIGGER_AZURE_CLIENT_SECRET")
}

func loadSharedKeyEnv() {
	os.Setenv("DIGGER_AZURE_SHARED_KEY", AZURE_SHARED_KEY)
	os.Setenv("DIGGER_AZURE_SA_NAME", AZURE_SA_NAME)
}

func loadConnStringEnv() {
	os.Setenv("DIGGER_AZURE_CONNECTION_STRING", AZURE_CONN_STRING)
}

func loadClientSecretEnv() {
	os.Setenv("DIGGER_AZURE_TENANT_ID", AZURE_TENANT_ID)
	os.Setenv("DIGGER_AZURE_CLIENT_ID", AZURE_CLIENT_ID)
	os.Setenv("DIGGER_AZURE_CLIENT_SECRET", AZURE_CLIENT_SECRET)
	os.Setenv("DIGGER_AZURE_SA_NAME", AZURE_SA_NAME)
}
