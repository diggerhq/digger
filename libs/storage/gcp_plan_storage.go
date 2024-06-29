package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
)

type PlanStorageGcp struct {
	Client  *storage.Client
	Bucket  *storage.BucketHandle
	Context context.Context
}

func (psg *PlanStorageGcp) PlanExists(artifactName string, storedPlanFilePath string) (bool, error) {
	obj := psg.Bucket.Object(storedPlanFilePath)
	_, err := obj.Attrs(psg.Context)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, fmt.Errorf("unable to get object attributes: %v", err)
	}
	return true, nil
}

func (psg *PlanStorageGcp) StorePlanFile(fileContents []byte, artifactName string, fileName string) error {
	fullPath := fileName
	obj := psg.Bucket.Object(fullPath)
	writer := obj.NewWriter(context.Background())
	defer writer.Close()

	if _, err := writer.Write(fileContents); err != nil {
		log.Printf("Failed to write file to bucket: %v", err)
		return err
	}
	return nil
}

func (psg *PlanStorageGcp) RetrievePlan(localPlanFilePath string, artifactName string, storedPlanFilePath string) (*string, error) {
	obj := psg.Bucket.Object(storedPlanFilePath)
	rc, err := obj.NewReader(psg.Context)
	if err != nil {
		return nil, fmt.Errorf("unable to read data from bucket: %v", err)
	}
	defer rc.Close()

	file, err := os.Create(localPlanFilePath)
	if err != nil {
		return nil, fmt.Errorf("unable to create file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, rc); err != nil {
		return nil, fmt.Errorf("unable to write data to file: %v", err)
	}
	fileName, err := filepath.Abs(file.Name())
	if err != nil {
		return nil, fmt.Errorf("unable to get absolute path for file: %v", err)
	}
	return &fileName, nil
}

func (psg *PlanStorageGcp) DeleteStoredPlan(artifactName string, storedPlanFilePath string) error {
	obj := psg.Bucket.Object(storedPlanFilePath)
	err := obj.Delete(psg.Context)

	if err != nil {
		return fmt.Errorf("unable to delete file '%v' from bucket: %v", storedPlanFilePath, err)
	}
	return nil
}
