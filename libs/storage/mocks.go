package storage

type MockPlanStorage struct{}

func (t MockPlanStorage) StorePlanFile(fileContents []byte, artifactName, fileName string) error {
	return nil
}

func (t MockPlanStorage) RetrievePlan(localPlanFilePath, artifactName, storedPlanFilePath string) (*string, error) {
	return nil, nil
}

func (t MockPlanStorage) DeleteStoredPlan(artifactName, storedPlanFilePath string) error {
	return nil
}

func (t MockPlanStorage) PlanExists(artifactName, storedPlanFilePath string) (bool, error) {
	return false, nil
}
