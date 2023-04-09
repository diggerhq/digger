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
	usingRealSA = false // Whether we are running tests against a real storage account or not.
	envNames    = []string{
		"DIGGER_AZURE_CONNECTION_STRING",
		"DIGGER_AZURE_SHARED_KEY",
		"DIGGER_AZURE_SA_NAME",
		"DIGGER_AZURE_TENANT_ID",
		"DIGGER_AZURE_CLIENT_ID",
		"DIGGER_AZURE_CLIENT_SECRET",
	}

	// Holds our current environment
	envs = map[string]string{}
)

type tt struct {
	loadEnv func(*SALockTestSuite)
	name    string
}

// We use a test table so we can run our tests,
// using multiple authentication variations.
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
					return
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
	prepareEnv(suite)

	// Make sure Azurite is started before the tests
	// if we are not using a real storage account
	if usingRealSA {
		return
	}
	conn, err := net.Dial("tcp", "127.0.0.1:10002")
	if err != nil {
		suite.T().Fatalf("Please make sure 'Azurite' table service is started before running Azure tests, or use a real storage account.")
	}
	conn.Close()

	cleanEnv()
}

func cleanEnv() {
	// Clean environment variables
	for _, env := range envNames {
		os.Setenv(env, "")
	}
}

// Runs after every test
func (suite *SALockTestSuite) TearDownTest() {
	cleanEnv()
}

// Runs after every sub-test
func (suite *SALockTestSuite) TearDownSubTest() {
	cleanEnv()
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
	os.Setenv("DIGGER_AZURE_CLIENT_ID", "")

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
			sal, err := NewStorageAccountLock()
			suite.NotNil(sal)
			suite.NoError(err)
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

func (suite *SALockTestSuite) TestUnlock_WhenLockDoesNotExist() {
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.loadEnv(suite)
			sal, _ := NewStorageAccountLock()
			resourceName := generateResourceName()

			// Unlocking
			ok, err := sal.Unlock(resourceName)
			suite.False(ok)
			suite.Error(err, "should have got an error")
		})
	}
}

func (suite *SALockTestSuite) TestUnlock_Twice_WhenLockExist() {
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.loadEnv(suite)
			sal, _ := NewStorageAccountLock()
			resourceName := generateResourceName()

			// Locking
			ok, err := sal.Lock(18, resourceName)
			suite.True(ok, "lock acquisition should be true")
			suite.NoError(err, "should not have got an error")

			// Unlocking the first time
			ok, err = sal.Unlock(resourceName)
			suite.True(ok)
			suite.NoError(err, "should not have got an error")

			// Unlocking the second time
			ok, err = sal.Unlock(resourceName)
			suite.False(ok)
			suite.Error(err, "should have got an error")
		})
	}
}

func (suite *SALockTestSuite) TestGetLock() {
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.loadEnv(suite)
			sal, err := NewStorageAccountLock()
			suite.NotNil(sal)
			suite.Require().NoError(err)

			// Locking
			resourceName := generateResourceName()
			ok, err := sal.Lock(21, resourceName)
			suite.Require().True(ok, "lock acquisition should be true")
			suite.Require().NoError(err, "should not have got an error")

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
			sal, err := NewStorageAccountLock()
			suite.Require().NotNil(sal)
			suite.Require().NoError(err)

			// Get a lock that doesn't exist
			resourceName := generateResourceName()
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
	if useRealSA == "" || useRealSA == "0" {
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

// Initialize and save environment variables into a map.
// This is useful so we can alter  environment variables
// without losing their initial values.

// This comes is handy when we want to test cases
// where an environment variable is not defined.
func prepareEnv(s *SALockTestSuite) {
	// When using Azurite, environment are known
	if !usingRealSA {
		envs["AZURE_SHARED_KEY"] = AZURITE_SHARED_KEY
		envs["AZURE_SA_NAME"] = AZURITE_SA_NAME
		envs["AZURE_CONN_STRING"] = AZURITE_CONN_STRING
		return
	}

	// When using real storage account, environment
	// variables are not known, and must be injected by the user
	// before starting our tests.
	for _, env := range envNames {
		envValue, exists := os.LookupEnv(env)
		if !exists {
			s.T().Fatalf("Since 'DIGGER_TEST_USE_REAL_SA' has been set, '%s' environment variable must also be set before starting the tests.", env)
		}

		envs[env] = envValue
	}
}

func loadSharedKeyEnv() {
	os.Setenv("DIGGER_AZURE_SHARED_KEY", envs["DIGGER_AZURE_SHARED_KEY"])
	os.Setenv("DIGGER_AZURE_SA_NAME", envs["DIGGER_AZURE_SA_NAME"])
}

func loadConnStringEnv() {
	os.Setenv("DIGGER_AZURE_CONNECTION_STRING", envs["DIGGER_AZURE_CONNECTION_STRING"])
}

func loadClientSecretEnv() {
	os.Setenv("DIGGER_AZURE_TENANT_ID", envs["DIGGER_AZURE_TENANT_ID"])
	os.Setenv("DIGGER_AZURE_CLIENT_ID", envs["DIGGER_AZURE_CLIENT_ID"])
	os.Setenv("DIGGER_AZURE_CLIENT_SECRET", envs["DIGGER_AZURE_CLIENT_SECRET"])
	os.Setenv("DIGGER_AZURE_SA_NAME", envs["DIGGER_AZURE_SA_NAME"])
}
