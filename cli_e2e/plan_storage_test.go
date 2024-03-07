package cli_e2e

import (
	"github.com/diggerhq/digger/cli/pkg/gcp"
	"github.com/diggerhq/digger/cli/pkg/storage"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
)

func TestGCPPlanStorageStorageAndRetrieval(t *testing.T) {
	file, err := os.CreateTemp("/tmp", "prefix")
	assert.Nil(t, err)
	defer os.Remove(file.Name())

	ctx, client := gcp.GetGoogleStorageClient()
	bucketName := strings.ToLower(os.Getenv("GOOGLE_STORAGE_BUCKET"))
	bucket := client.Bucket(bucketName)
	planStorage := &storage.PlanStorageGcp{
		Client:  client,
		Bucket:  bucket,
		Context: ctx,
	}
	contents := []byte{'a'}
	artefactName := "myartefact"
	fileName := "myplan.tfplan"
	planStorage.StorePlanFile(contents, artefactName, fileName)
	exists, err := planStorage.PlanExists(artefactName, fileName)
	assert.Nil(t, err)
	assert.True(t, exists)

	planStorage.RetrievePlan(file.Name(), artefactName, fileName)
	readContents, err := os.ReadFile(file.Name())
	assert.Nil(t, err)
	assert.Equal(t, readContents, contents)
}
