package terraform

import (
	"log"
	"os"
)

func CreateTestTerraformProject() string {
	file, err := os.MkdirTemp("", "digger-test-*")
	if err != nil {
		log.Fatal(err)
	}
	return file
}

func CreateInvalidTerraformTestFile(dir string) {
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

func CreateValidTerraformTestFile(dir string) {
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

func CreateMultiEnvDiggerYmlFile(dir string) {
	f, err := os.Create(dir + "/digger.yml")
	if err != nil {
		log.Fatal(err)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	digger_yml := `
projects:
- name: dev
  branch: /main/
  dir: dev
  workspace: default
- name: prod
  branch: /main/
  dir: prod
  workspace: default
`

	_, err2 := f.WriteString(digger_yml)
	if err2 != nil {
		log.Fatal(err2)
	}
}

func CreateSingleEnvDiggerYmlFile(dir string) {
	f, err := os.Create(dir + "/digger.yml")
	if err != nil {
		log.Fatal(err)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	digger_yml := `
projects:
- name: dev
  branch: /main/
  dir: .
  workspace: default
  terragrunt: false
  workflow: dev
workflows:
  dev:
    env_vars:
      state:
      - name: TF_VAR_my_var
        value: my_value
      commands:
      - name: TF_VAR_my_var_commands
        value: my_value_commands
`
	_, err2 := f.WriteString(digger_yml)
	if err2 != nil {
		log.Fatal(err2)
	}
}

func CreateCustomDiggerYmlFile(dir string, diggerYaml string) {
	f, err := os.Create(dir + "/digger.yml")
	if err != nil {
		log.Fatal(err)
	}

	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)

	_, err2 := f.WriteString(diggerYaml)
	if err2 != nil {
		log.Fatal(err2)
	}
}
