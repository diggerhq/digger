package storage

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

type PlanStorageAzure struct {
	ServiceClient *azblob.Client
	ContainerName string
	Context       context.Context
}

func (psa *PlanStorageAzure) PlanExists(artifactName string, storedPlanFilePath string) (bool, error) {
	slog.Debug("Checking if plan exists in Azure Blob Storage",
		"container", psa.ContainerName,
		"path", storedPlanFilePath,
		"artifactName", artifactName)

	blobClient := psa.ServiceClient.ServiceClient().NewContainerClient(psa.ContainerName).NewBlobClient(storedPlanFilePath)

	// Get the blob properties
	resp, err := blobClient.GetProperties(context.TODO(), nil)
	if err != nil {
		slog.Error("Failed to get blob properties",
			"container", psa.ContainerName,
			"path", storedPlanFilePath,
			"error", err)
		return false, err
	}
	slog.Debug("Blob found",
		"container", psa.ContainerName,
		"path", storedPlanFilePath,
		"size", resp.ContentLength,
		"lastModified", resp.LastModified,
	)

	return true, nil
}

func (psa *PlanStorageAzure) StorePlanFile(fileContents []byte, artifactName string, fileName string) error {
	slog.Debug("Storing plan file in Azure Blob Storage",
		"container", psa.ContainerName,
		"path", fileName,
		"artifactName", artifactName,
		"size", len(fileContents))

	_, err := psa.ServiceClient.UploadBuffer(
		psa.Context,
		psa.ContainerName,
		fileName,
		fileContents,
		&azblob.UploadBufferOptions{},
	)
	
	if err != nil {
		slog.Error("Failed to write file to Azure Blob Storage",
			"container", psa.ContainerName,
			"path", fileName,
			"error", err)
		return err
	}

	slog.Info("Successfully stored plan file in Azure Blob Storage",
		"container", psa.ContainerName,
		"path", fileName)
	return nil
}

func (psa *PlanStorageAzure) RetrievePlan(localPlanFilePath string, artifactName string, storedPlanFilePath string) (*string, error) {
	slog.Debug("Retrieving plan from Azure Blob Storage",
		"container", psa.ContainerName,
		"path", storedPlanFilePath,
		"artifactName", artifactName,
		"localPath", localPlanFilePath)

	localFile, err := os.Create(localPlanFilePath)
	if err != nil {
		slog.Error("Unable to create local file",
			"path", localPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("unable to create file: %v", err)
	}
	defer localFile.Close()

	_, err = psa.ServiceClient.DownloadFile(
		psa.Context,
		psa.ContainerName,
		storedPlanFilePath,
		localFile,
		&azblob.DownloadFileOptions{},
	)
	if err != nil {
		slog.Error("Unable to read data from Azure Blob Storage",
			"container", psa.ContainerName,
			"path", storedPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("unable to read data from blob: %v", err)
	}

	fileName, err := filepath.Abs(localFile.Name())
	if err != nil {
		slog.Error("Unable to get absolute path for file",
			"path", localFile.Name(),
			"error", err)
		return nil, fmt.Errorf("unable to get absolute path for file: %v", err)
	}

	slog.Info("Successfully retrieved plan from Azure Blob Storage",
		"container", psa.ContainerName,
		"path", storedPlanFilePath,
		"localPath", fileName)
	return &fileName, nil
}

func (psa *PlanStorageAzure) DeleteStoredPlan(artifactName string, storedPlanFilePath string) error {
	slog.Debug("Deleting stored plan from Azure Blob Storage",
		"container", psa.ContainerName,
		"path", storedPlanFilePath,
		"artifactName", artifactName)

	_, err := psa.ServiceClient.DeleteBlob(
		psa.Context,
		psa.ContainerName,
		storedPlanFilePath,
		&azblob.DeleteBlobOptions{},
	)

	if err != nil {
		slog.Error("Unable to delete file from Azure Blob Storage",
			"container", psa.ContainerName,
			"path", storedPlanFilePath,
			"error", err)
		return fmt.Errorf("unable to delete file '%v' from container: %v", storedPlanFilePath, err)
	}

	slog.Info("Successfully deleted plan from Azure Blob Storage",
		"container", psa.ContainerName,
		"path", storedPlanFilePath)
	return nil
}
