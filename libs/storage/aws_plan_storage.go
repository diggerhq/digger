package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type S3Client interface {
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type AwsS3EncryptionType string

const (
	ServerSideEncryptionAes256 AwsS3EncryptionType = "AES256"
	ServerSideEncryptionAwsKms AwsS3EncryptionType = "aws:kms"
)

type PlanStorageAWS struct {
	Client            S3Client
	Bucket            string
	Context           context.Context
	EncryptionEnabled bool
	EncryptionType    AwsS3EncryptionType
	KMSEncryptionId   string
}

func NewAWSPlanStorage(bucketName string, encryptionEnabled bool, encryptionType, KMSEncryptionId string) (*PlanStorageAWS, error) {
	if bucketName == "" {
		slog.Error("AWS S3 bucket name not provided")
		return nil, fmt.Errorf("AWS_S3_BUCKET is not defined")
	}

	slog.Debug("Creating AWS plan storage",
		"bucketName", bucketName,
		"encryptionEnabled", encryptionEnabled,
		"encryptionType", encryptionType)

	ctx, client, err := GetAWSStorageClient()
	if err != nil {
		slog.Error("Could not retrieve AWS storage client", "error", err)
		return nil, fmt.Errorf("could not retrieve aws storage client")
	}

	planStorage := &PlanStorageAWS{
		Context: ctx,
		Client:  client,
		Bucket:  bucketName,
	}

	if encryptionEnabled {
		planStorage.EncryptionEnabled = true
		switch encryptionType {
		case "AES256":
			slog.Debug("Using AES256 encryption for S3 storage")
			planStorage.EncryptionType = ServerSideEncryptionAes256
		case "KMS":
			if KMSEncryptionId == "" {
				slog.Error("KMS encryption requested but no KMS key specified")
				return nil, fmt.Errorf("KMS encryption requested but no KMS key specified")
			}
			slog.Debug("Using KMS encryption for S3 storage", "kmsKeyId", KMSEncryptionId)
			planStorage.EncryptionType = ServerSideEncryptionAwsKms
			planStorage.KMSEncryptionId = KMSEncryptionId
		default:
			slog.Error("Unknown encryption type specified", "encryptionType", encryptionType)
			return nil, fmt.Errorf("unknown encryption type specified for aws plan bucket: %v", encryptionType)
		}
	}

	slog.Info("AWS plan storage initialized successfully", "bucket", bucketName)
	return planStorage, nil
}

func (psa *PlanStorageAWS) PlanExists(artifactName, storedPlanFilePath string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(psa.Bucket),
		Key:    aws.String(storedPlanFilePath),
	}

	_, err := psa.Client.HeadObject(psa.Context, input)
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *types.NotFound:
				slog.Debug("Plan is not available",
					"bucket", psa.Bucket,
					"key", storedPlanFilePath)
				return false, nil
			default:
				slog.Error("Error accessing S3 bucket",
					"bucket", psa.Bucket,
					"error", err)
			}
		}
		return false, fmt.Errorf("unable to get object attributes: %v", err)
	}

	slog.Debug("Plan exists",
		"bucket", psa.Bucket,
		"key", storedPlanFilePath)
	return true, nil
}

func (psa *PlanStorageAWS) StorePlanFile(fileContents []byte, artifactName, fileName string) error {
	input := &s3.PutObjectInput{
		Body:   bytes.NewReader(fileContents),
		Bucket: aws.String(psa.Bucket),
		Key:    aws.String(fileName),
	}

	// support for encryption
	if psa.EncryptionEnabled {
		input.ServerSideEncryption = types.ServerSideEncryption(psa.EncryptionType)
		if psa.EncryptionType == ServerSideEncryptionAwsKms {
			input.SSEKMSKeyId = aws.String(psa.KMSEncryptionId)
		}
	}

	_, err := psa.Client.PutObject(psa.Context, input)
	if err != nil {
		slog.Error("Failed to write file to bucket",
			"bucket", psa.Bucket,
			"key", fileName,
			"error", err)
		return err
	}

	slog.Info("Successfully stored plan file",
		"bucket", psa.Bucket,
		"key", fileName)
	return nil
}

func (psa *PlanStorageAWS) RetrievePlan(localPlanFilePath, artifactName, storedPlanFilePath string) (*string, error) {
	output, err := psa.Client.GetObject(psa.Context, &s3.GetObjectInput{
		Bucket: aws.String(psa.Bucket),
		Key:    aws.String(storedPlanFilePath),
	})
	if err != nil {
		slog.Error("Unable to read data from bucket",
			"bucket", psa.Bucket,
			"key", storedPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("unable to read data from bucket: %v", err)
	}
	defer output.Body.Close()

	file, err := os.Create(localPlanFilePath)
	if err != nil {
		slog.Error("Unable to create local file",
			"path", localPlanFilePath,
			"error", err)
		return nil, fmt.Errorf("unable to create file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, output.Body); err != nil {
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

	slog.Info("Successfully retrieved plan",
		"bucket", psa.Bucket,
		"key", storedPlanFilePath,
		"localPath", fileName)
	return &fileName, nil
}

func (psa *PlanStorageAWS) DeleteStoredPlan(artifactName, storedPlanFilePath string) error {
	_, err := psa.Client.DeleteObject(psa.Context, &s3.DeleteObjectInput{
		Bucket: aws.String(psa.Bucket),
		Key:    aws.String(storedPlanFilePath),
	})
	if err != nil {
		slog.Error("Unable to delete file from bucket",
			"bucket", psa.Bucket,
			"key", storedPlanFilePath,
			"error", err)
		return fmt.Errorf("unable to delete file '%v' from bucket: %v", storedPlanFilePath, err)
	}

	slog.Info("Successfully deleted plan",
		"bucket", psa.Bucket,
		"key", storedPlanFilePath)
	return nil
}

func GetAWSStorageClient() (context.Context, *s3.Client, error) {
	ctx := context.Background()

	slog.Debug("Loading AWS configuration for S3 client")
	sdkConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		slog.Error("Failed to load AWS configuration", "error", err)
		return ctx, nil, err
	}

	slog.Debug("AWS S3 client created successfully")
	return ctx, s3.NewFromConfig(sdkConfig), nil
}
