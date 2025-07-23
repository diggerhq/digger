package storage

type PlanStorage interface {
	StorePlanFile(fileContents []byte, artifactName, storedPlanFilePath string) error
	RetrievePlan(localPlanFilePath, artifactName, storedPlanFilePath string) (*string, error)
	DeleteStoredPlan(artifactName, storedPlanFilePath string) error
	PlanExists(artifactName, storedPlanFilePath string) (bool, error)
}
