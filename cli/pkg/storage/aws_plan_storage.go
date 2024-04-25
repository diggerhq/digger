package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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

type PlanStorageAWS struct {
	Client  S3Client
	Bucket  string
	Context context.Context
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
				log.Printf("Plan %v is not available.\n", psa.Bucket)
				return false, nil
			default:
				log.Printf("Either you don't have access to bucket %v or another error occurred: %v\n", psa.Bucket, err)
			}
		}
		return false, fmt.Errorf("unable to get object attributes: %v", err)
	}
	return true, nil
}

func (psa *PlanStorageAWS) StorePlanFile(fileContents []byte, artifactName, fileName string) error {
	input := &s3.PutObjectInput{
		Body:   bytes.NewReader(fileContents),
		Bucket: aws.String(psa.Bucket),
		Key:    aws.String(fileName),
	}
	_, err := psa.Client.PutObject(psa.Context, input)
	if err != nil {
		log.Printf("Failed to write file to bucket: %v", err)
		return err
	}
	return nil
}

func (psa *PlanStorageAWS) RetrievePlan(localPlanFilePath, artifactName, storedPlanFilePath string) (*string, error) {
	output, err := psa.Client.GetObject(psa.Context, &s3.GetObjectInput{
		Bucket: aws.String(psa.Bucket),
		Key:    aws.String(storedPlanFilePath),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to read data from bucket: %v", err)
	}
	defer output.Body.Close()

	file, err := os.Create(localPlanFilePath)
	if err != nil {
		return nil, fmt.Errorf("unable to create file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, output.Body); err != nil {
		return nil, fmt.Errorf("unable to write data to file: %v", err)
	}
	fileName, err := filepath.Abs(file.Name())
	if err != nil {
		return nil, fmt.Errorf("unable to get absolute path for file: %v", err)
	}
	return &fileName, nil
}

func (psa *PlanStorageAWS) DeleteStoredPlan(artifactName, storedPlanFilePath string) error {
	_, err := psa.Client.DeleteObject(psa.Context, &s3.DeleteObjectInput{
		Bucket: aws.String(psa.Bucket),
		Key:    aws.String(storedPlanFilePath),
	})
	if err != nil {
		return fmt.Errorf("unable to delete file '%v' from bucket: %v", storedPlanFilePath, err)
	}
	return nil
}

func GetAWSStorageClient() (context.Context, *s3.Client, error) {
	ctx := context.Background()
	sdkConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, s3.NewFromConfig(sdkConfig), nil
}
