package utils

type MockTerraform struct {
	commands []string
}

func (tf *MockTerraform) Apply() (string, string, error) {
	tf.commands = append(tf.commands, "apply")
	return "", "", nil
}

func (tf *MockTerraform) Plan() (string, string, error) {
	tf.commands = append(tf.commands, "plan")
	return "", "", nil
}
