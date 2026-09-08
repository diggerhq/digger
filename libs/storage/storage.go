package storage

import "time"

type PlanStorage interface {
	StorePlanFile(fileContents []byte, artifactName string, storedPlanFilePath string) error
	RetrievePlan(localPlanFilePath string, artifactName string, storedPlanFilePath string) (*string, error)
	DeleteStoredPlan(artifactName string, storedPlanFilePath string) error
	PlanExists(artifactName string, storedPlanFilePath string) (bool, error)
}

type PlanUrlProvider interface {
	StoredPlanUrl(storedPlanFilePath string, validFor time.Duration) (string, error)
}
