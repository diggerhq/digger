package service_clients

import (
	"fmt"
	"os"
)

func GetBackgroundJobsClient() (BackgroundJobsClient, error) {
	clientType := os.Getenv("BACKGROUND_JOBS_CLIENT_TYPE")
	switch clientType {
		case "k8s":
			clientSet, err := newInClusterClient()
			if err != nil {
				return nil, fmt.Errorf("error creating k8s client: %v", err)
			}
			return K8sJobClient{
				clientset:         clientSet,
				namespace:          "opentaco",
			}, nil
		case "flyio":
			return FlyIOMachineJobClient{}, nil
		case "local-exec":
			return LocalExecJobClient{}, nil
	}
	return FlyIOMachineJobClient{}, nil
}
