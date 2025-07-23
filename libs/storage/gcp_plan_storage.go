package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
)

type PlanStorageGcp struct {
	Client  *storage.Client
	Bucket  *storage.BucketHandle
	Context context.Context
}

func (psg *PlanStorageGcp) PlanExists(artifactName, storedPlanFilePath string) (bool, error) {
	slog.Debug("Checking if plan exists in GCP storage",
		"bucket", psg.Bucket.BucketName(),
		"path", storedPlanFilePath,
		"artifactName", artifactName)

	obj := psg.Bucket.Object(storedPlanFilePath)
	attrs, err := obj.Attrs(psg.Context)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			slog.Debug("Plan does not exist in GCP storage",
				"bucket", psg.Bucket.BucketName(),
				"path", storedPlanFilePath)
			return false, nil
		}
		slog.Error("Failed to check if plan exists in GCP storage",
			"bucket", psg.Bucket.BucketName(),
			"path", storedPlanFilePath,
			"error", err)
		return false, fmt.Errorf("unable to get object attributes: %v", err)
	}

	slog.Debug("Plan exists in GCP storage",
		"bucket", psg.Bucket.BucketName(),
		"path", storedPlanFilePath,
		"size", attrs.Size,
		"updated", attrs.Updated)
	return true, nil
}

func (psg *PlanStorageGcp) StorePlanFile(fileContents []byte, artifactName, fileName string) error {
	slog.Debug("Storing plan file in GCP storage",
		"bucket", psg.Bucket.BucketName(),
		"path", fileName,
		"artifactName", artifactName,
		"size", len(fileContents))

	fullPath := fileName
	obj := psg.Bucket.Object(fullPath)
	writer := obj.NewWriter(context.Background())
	defer writer.Close()

	if _, err := writer.Write(fileContents); err != nil {
		slog.Error("Failed to write file to GCP bucket",
			"bucket", psg.Bucket.BucketName(),
			"path", fileName,
			"error", err)
		return err
	}

	slog.Info("Successfully stored plan file in GCP storage",
		"bucket", psg.Bucket.BucketName(),
		"path", fileName)
	return nil
}

func (psg *PlanStorageGcp) RetrievePlan(localPlanFilePath, artifactName, storedPlanFilePath string) (*string, error) {
	slog.Debug("Retrieving plan from GCP storage",
		"bucket", psg.Bucket.BucketName(),
		"path", storedPlanFilePath,
		"artifactName", artifactName,
		"localPath", localPlanFilePath)

	obj := psg.Bucket.Object(storedPlanFilePath)
	rc, err := obj.NewReader(psg.Context)
	if err != nil {
		slog.Error("Unable to read data from GCP bucket",
			"bucket", psg.Bucket.BucketName(),
			"path", storedPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("unable to read data from bucket: %v", err)
	}
	defer rc.Close()

	file, err := os.Create(localPlanFilePath)
	if err != nil {
		slog.Error("Unable to create local file",
			"path", localPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("unable to create file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, rc); err != nil {
		slog.Error("Unable to write data to file",
			"path", localPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("unable to write data to file: %v", err)
	}

	fileName, err := filepath.Abs(file.Name())
	if err != nil {
		slog.Error("Unable to get absolute path for file",
			"path", file.Name(),
			"error", err)
		return nil, fmt.Errorf("unable to get absolute path for file: %v", err)
	}

	slog.Info("Successfully retrieved plan from GCP storage",
		"bucket", psg.Bucket.BucketName(),
		"path", storedPlanFilePath,
		"localPath", fileName)
	return &fileName, nil
}

func (psg *PlanStorageGcp) DeleteStoredPlan(artifactName, storedPlanFilePath string) error {
	slog.Debug("Deleting stored plan from GCP storage",
		"bucket", psg.Bucket.BucketName(),
		"path", storedPlanFilePath,
		"artifactName", artifactName)

	obj := psg.Bucket.Object(storedPlanFilePath)
	err := obj.Delete(psg.Context)
	if err != nil {
		slog.Error("Unable to delete file from GCP bucket",
			"bucket", psg.Bucket.BucketName(),
			"path", storedPlanFilePath,
			"error", err)
		return fmt.Errorf("unable to delete file '%v' from bucket: %v", storedPlanFilePath, err)
	}

	slog.Info("Successfully deleted plan from GCP storage",
		"bucket", psg.Bucket.BucketName(),
		"path", storedPlanFilePath)
	return nil
}
