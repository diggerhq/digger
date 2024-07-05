package storage

type MockPlanStorage struct {
}

func (t *MockPlanStorage) StorePlanFile(fileContents []byte, artifactName string, fileName string) error {
	return nil
}

func (t MockPlanStorage) RetrievePlan(localPlanFilePath string, artifactName string, storedPlanFilePath string) (*string, error) {
	return nil, nil
}

func (t MockPlanStorage) DeleteStoredPlan(artifactName string, storedPlanFilePath string) error {
	return nil
}

func (t MockPlanStorage) PlanExists(artifactName string, storedPlanFilePath string) (bool, error) {
	return false, nil
}
