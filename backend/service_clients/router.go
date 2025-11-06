package service_clients

import (
	"fmt"
	"os"
)

func GetBackgroundJobsClient() (BackgroundJobsClient, error) {
	clientType := os.Getenv("BACKGROUND_JOBS_CLIENT_TYPE")
	if clientType == "k8s" {
		batchClient, err := newInClusterBatchClient()
		if err != nil {
			return nil, fmt.Errorf("error creating k8s client: %v", err)
		}
		return K8sJobClient{
			batch:     batchClient,
			namespace: "opentaco",
		}, nil
	} else {
		return FlyIOMachineJobClient{}, nil
	}
}
