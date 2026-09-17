package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"

	"github.com/diggerhq/digger/libs/locking/alicloud"
)

type PlanStorageAlicloud struct {
	Client  alicloud.OSSClient
	Bucket  string
	Context context.Context
}

func (ps *PlanStorageAlicloud) PlanExists(artifactName string, storedPlanFilePath string) (bool, error) {
	_, err := ps.Client.HeadObject(ps.Context, &oss.HeadObjectRequest{
		Bucket: oss.Ptr(ps.Bucket),
		Key:    oss.Ptr(storedPlanFilePath),
	})
	if err != nil {
		if alicloud.HasStatus(err, http.StatusNotFound) {
			slog.Debug("Plan does not exist in OSS",
				"bucket", ps.Bucket,
				"key", storedPlanFilePath)
			return false, nil
		}
		slog.Error("Failed to check if plan exists in OSS",
			"bucket", ps.Bucket,
			"key", storedPlanFilePath,
			"error", err)
		return false, fmt.Errorf("head object %q in bucket %q: %w", storedPlanFilePath, ps.Bucket, err)
	}

	slog.Debug("Plan exists in OSS",
		"bucket", ps.Bucket,
		"key", storedPlanFilePath)
	return true, nil
}

func (ps *PlanStorageAlicloud) StorePlanFile(fileContents []byte, artifactName string, fileName string) error {
	_, err := ps.Client.PutObject(ps.Context, &oss.PutObjectRequest{
		Bucket: oss.Ptr(ps.Bucket),
		Key:    oss.Ptr(fileName),
		Body:   bytes.NewReader(fileContents),
	})
	if err != nil {
		slog.Error("Failed to write plan file to OSS",
			"bucket", ps.Bucket,
			"key", fileName,
			"error", err)
		return fmt.Errorf("put object %q in bucket %q: %w", fileName, ps.Bucket, err)
	}

	slog.Info("Successfully stored plan file in OSS",
		"bucket", ps.Bucket,
		"key", fileName,
		"size", len(fileContents))
	return nil
}

func (ps *PlanStorageAlicloud) RetrievePlan(localPlanFilePath string, artifactName string, storedPlanFilePath string) (*string, error) {
	result, err := ps.Client.GetObject(ps.Context, &oss.GetObjectRequest{
		Bucket: oss.Ptr(ps.Bucket),
		Key:    oss.Ptr(storedPlanFilePath),
	})
	if err != nil {
		slog.Error("Unable to read plan from OSS",
			"bucket", ps.Bucket,
			"key", storedPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("get object %q from bucket %q: %w", storedPlanFilePath, ps.Bucket, err)
	}
	defer result.Body.Close()

	file, err := os.Create(localPlanFilePath)
	if err != nil {
		slog.Error("Unable to create local file",
			"path", localPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("create file %q: %w", localPlanFilePath, err)
	}
	defer file.Close()

	if _, err = io.Copy(file, result.Body); err != nil {
		slog.Error("Unable to write plan to local file",
			"path", localPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("write file %q: %w", localPlanFilePath, err)
	}

	fileName, err := filepath.Abs(file.Name())
	if err != nil {
		slog.Error("Unable to get absolute path for file",
			"path", file.Name(),
			"error", err)
		return nil, fmt.Errorf("absolute path for %q: %w", file.Name(), err)
	}

	slog.Info("Successfully retrieved plan from OSS",
		"bucket", ps.Bucket,
		"key", storedPlanFilePath,
		"localPath", fileName)
	return &fileName, nil
}

func (ps *PlanStorageAlicloud) DeleteStoredPlan(artifactName string, storedPlanFilePath string) error {
	_, err := ps.Client.DeleteObject(ps.Context, &oss.DeleteObjectRequest{
		Bucket: oss.Ptr(ps.Bucket),
		Key:    oss.Ptr(storedPlanFilePath),
	})
	if err != nil {
		slog.Error("Unable to delete plan from OSS",
			"bucket", ps.Bucket,
			"key", storedPlanFilePath,
			"error", err)
		return fmt.Errorf("delete object %q from bucket %q: %w", storedPlanFilePath, ps.Bucket, err)
	}

	slog.Info("Successfully deleted plan from OSS",
		"bucket", ps.Bucket,
		"key", storedPlanFilePath)
	return nil
}
