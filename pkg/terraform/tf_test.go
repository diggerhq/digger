package terraform

import (
	"log"
	"os"
	"testing"
)

func createTestTerraformProject() string {
	file, err := os.MkdirTemp("", "digger-test")
	if err != nil {
		log.Fatal(err)
	}
	return file
}

func createInvalidTerraformTestFile(dir string) {
	f, err := os.Create(dir + "/main.tf")
	if err != nil {
		log.Fatal(err)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	_, err2 := f.WriteString("resource \"null_resource\" \"test\" {\n}\n")
	if err2 != nil {
		log.Fatal(err2)
	}
}

func createValidTerraformTestFile(dir string) {
	f, err := os.Create(dir + "/main.tf")
	if err != nil {
		log.Fatal(err)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	_, err2 := f.WriteString("resource \"null_resource\" \"test\" {\n}\n")
	if err2 != nil {
		log.Fatal(err2)
	}
}

func TestExecuteTerraformPlan(t *testing.T) {
	dir := createTestTerraformProject()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

	createValidTerraformTestFile(dir)

	tf := Terraform{WorkingDir: dir}
	_, _, _, err := tf.Plan()
	if err != nil {
		print(err.Error())
	}
}

func TestExecuteTerraformApply(t *testing.T) {
	dir := createTestTerraformProject()
	defer func(name string) {
		err := os.RemoveAll(name)
		if err != nil {
			log.Fatal(err)
		}
	}(dir)

	createValidTerraformTestFile(dir)

	tf := Terraform{WorkingDir: dir}
	_, _, err := tf.Apply()
	if err != nil {
		print(err.Error())
	}
}
