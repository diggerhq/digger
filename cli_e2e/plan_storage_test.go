package cli_e2e

import (
	"fmt"
	"github.com/diggerhq/digger/libs/locking/gcp"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/diggerhq/digger/cli/pkg/storage"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func TestGCPPlanStorageStorageAndRetrieval(t *testing.T) {
	file, err := os.CreateTemp("/tmp", "prefix")
	fmt.Printf("%v", err)
	assert.Nil(t, err)
	defer os.Remove(file.Name())

	fmt.Printf("getting cp client")
	ctx, client := gcp.GetGoogleStorageClient()
	fmt.Printf("getting bucket")
	bucketName := strings.ToLower(os.Getenv("GOOGLE_STORAGE_PLAN_ARTEFACT_BUCKET"))
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
	fmt.Printf("error StorePlanFile: %v", err)
	assert.Nil(t, err)
	exists, err := planStorage.PlanExists(artefactName, fileName)
	fmt.Printf("error PlanExists: %v", err)
	assert.Nil(t, err)
	assert.True(t, exists)

	planStorage.RetrievePlan(file.Name(), artefactName, fileName)
	readContents, err := os.ReadFile(file.Name())
	fmt.Printf("error RetrievePlan: %v", err)
	assert.Nil(t, err)
	assert.Equal(t, readContents, contents)
}

func TestAWSPlanStorageStorageAndRetrieval(t *testing.T) {
	file, err := os.CreateTemp("/tmp", "prefix2")
	if err != nil {
		log.Printf("error creating temp dir: %v", err)
	}
	assert.Nil(t, err)
	defer os.Remove(file.Name())

	log.Printf("getting AWS S3 client")
	ctx, client, err := storage.GetAWSStorageClient()
	if err != nil {
		t.Errorf("failed to get AWS storage client: %v", err)
	}
	bucketName := strings.ToLower(os.Getenv("AWS_S3_BUCKET"))
	log.Printf("using AWS S3 bucket found by env var 'AWS_S3_BUCKET': %s", bucketName)
	planStorage := &storage.PlanStorageAWS{
		Client:  client,
		Bucket:  bucketName,
		Context: ctx,
	}
	contents := []byte{'a'}
	artefactName := "myartefact"
	fileName := "myplan.tfplan"
	err = planStorage.StorePlanFile(contents, artefactName, fileName)
	if err != nil {
		log.Printf("error StorePlanFile: %v", err)
	}
	assert.Nil(t, err)
	exists, err := planStorage.PlanExists(artefactName, fileName)
	if err != nil {
		log.Printf("error PlanExists: %v", err)
	}
	assert.Nil(t, err)
	assert.True(t, exists)

	planStorage.RetrievePlan(file.Name(), artefactName, fileName)
	readContents, err := os.ReadFile(file.Name())
	if err != nil {
		log.Printf("error RetrievePlan: %v", err)
	}
	assert.Nil(t, err)
	assert.Equal(t, readContents, contents)
}
