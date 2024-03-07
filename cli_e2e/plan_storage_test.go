package cli_e2e

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/gcp"
	"github.com/diggerhq/digger/cli/pkg/storage"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
)

func TestGCPPlanStorageStorageAndRetrieval(t *testing.T) {
	fmt.Printf("in function")
	file, err := os.CreateTemp("/tmp", "prefix")
	fmt.Printf("%v", err)
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
	err = planStorage.StorePlanFile(contents, artefactName, fileName)
	fmt.Printf("%v", err)
	assert.Nil(t, err)
	exists, err := planStorage.PlanExists(artefactName, fileName)
	fmt.Printf("%v", err)
	assert.Nil(t, err)
	assert.True(t, exists)

	planStorage.RetrievePlan(file.Name(), artefactName, fileName)
	readContents, err := os.ReadFile(file.Name())
	fmt.Printf("%v", err)
	assert.Nil(t, err)
	assert.Equal(t, readContents, contents)
}
