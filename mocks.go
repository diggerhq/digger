package main

type MockTerraform struct {
	commands []string
}

func (tf *MockTerraform) Apply() error {
	tf.commands = append(tf.commands, "apply")
	return nil
}

func (tf *MockTerraform) Plan() error {
	tf.commands = append(tf.commands, "plan")
	return nil
}
