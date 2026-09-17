package cli_e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/diggerhq/digger/libs/locking/alicloud"
	storage2 "github.com/diggerhq/digger/libs/storage"
)

func TestAlicloudOSSLock(t *testing.T) {
	if os.Getenv(alicloud.LockBucketEnv) == "" {
		t.Skipf("%s not set, skipping Alibaba Cloud lock e2e test", alicloud.LockBucketEnv)
	}

	lock, err := alicloud.NewOSSLock()
	require.NoError(t, err)

	resource := "digger-e2e/repo#project-" + time.Now().UTC().Format("20060102T150405")
	t.Cleanup(func() { _, _ = lock.Unlock(resource) })

	holder, err := lock.GetLock(resource)
	require.NoError(t, err)
	require.Nil(t, holder)

	acquired, err := lock.Lock(101, resource)
	require.NoError(t, err)
	assert.True(t, acquired)

	holder, err = lock.GetLock(resource)
	require.NoError(t, err)
	require.NotNil(t, holder)
	assert.Equal(t, 101, *holder)

	acquired, err = lock.Lock(102, resource)
	require.NoError(t, err)
	assert.False(t, acquired, "second PR must not steal the lock")

	holder, err = lock.GetLock(resource)
	require.NoError(t, err)
	require.NotNil(t, holder)
	assert.Equal(t, 101, *holder)

	released, err := lock.Unlock(resource)
	require.NoError(t, err)
	assert.True(t, released)

	holder, err = lock.GetLock(resource)
	require.NoError(t, err)
	assert.Nil(t, holder)
}

func TestAlicloudPlanStorageStorageAndRetrieval(t *testing.T) {
	bucket := os.Getenv("ALICLOUD_OSS_PLAN_ARTEFACT_BUCKET")
	if bucket == "" {
		t.Skip("ALICLOUD_OSS_PLAN_ARTEFACT_BUCKET not set, skipping Alibaba Cloud plan storage e2e test")
	}

	client, err := alicloud.NewOSSClient()
	require.NoError(t, err)
	planStorage := &storage2.PlanStorageAlicloud{
		Client:  client,
		Bucket:  bucket,
		Context: context.Background(),
	}

	contents := []byte("digger e2e plan " + time.Now().UTC().Format(time.RFC3339))
	artefactName := "myartefact"
	fileName := "digger-e2e/myplan-" + time.Now().UTC().Format("20060102T150405") + ".tfplan"
	t.Cleanup(func() { _ = planStorage.DeleteStoredPlan(artefactName, fileName) })

	exists, err := planStorage.PlanExists(artefactName, fileName)
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, planStorage.StorePlanFile(contents, artefactName, fileName))

	exists, err = planStorage.PlanExists(artefactName, fileName)
	require.NoError(t, err)
	assert.True(t, exists)

	localPath := filepath.Join(t.TempDir(), "plan.tfplan")
	retrieved, err := planStorage.RetrievePlan(localPath, artefactName, fileName)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	readContents, err := os.ReadFile(*retrieved)
	require.NoError(t, err)
	assert.Equal(t, contents, readContents)

	require.NoError(t, planStorage.DeleteStoredPlan(artefactName, fileName))
	exists, err = planStorage.PlanExists(artefactName, fileName)
	require.NoError(t, err)
	assert.False(t, exists)
}
