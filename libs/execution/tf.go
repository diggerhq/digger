package execution

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type TerraformExecutor interface {
	Init([]string, map[string]string) (string, string, error)
	Apply([]string, *string, map[string]string) (string, string, error)
	Destroy([]string, map[string]string) (string, string, error)
	Plan([]string, map[string]string, string) (bool, string, string, error)
	Show([]string, map[string]string, string) (string, string, error)
}

type Terraform struct {
	WorkingDir string
	Workspace  string
}

func (tf Terraform) Init(params []string, envs map[string]string) (string, string, error) {
	params = append(params, "-input=false")
	params = append(params, "-no-color")
	stdout, stderr, _, err := tf.runTerraformCommand("init", true, envs, params...)

	// switch to workspace for next step
	// TODO: make this an individual and isolated step
	if tf.Workspace != "default" {
		werr := tf.switchToWorkspace(envs)
		if werr != nil {
			log.Printf("Fatal: Error terraform switch to workspace %v", err)
			return "", "", werr
		}
	}

	return stdout, stderr, err
}

func (tf Terraform) Apply(params []string, plan *string, envs map[string]string) (string, string, error) {
	params = append(params, []string{"-lock-timeout=3m"}...)
	params = append(append(append(params, "-input=false"), "-no-color"), "-auto-approve")
	if plan != nil {
		params = append(params, *plan)
	}
	stdout, stderr, _, err := tf.runTerraformCommand("apply", true, envs, params...)
	return stdout, stderr, err
}

func (tf Terraform) Destroy(params []string, envs map[string]string) (string, string, error) {
	params = append(append(append(params, "-input=false"), "-no-color"), "-auto-approve")
	stdout, stderr, _, err := tf.runTerraformCommand("destroy", true, envs, params...)
	return stdout, stderr, err
}

func (tf Terraform) switchToWorkspace(envs map[string]string) error {
	workspaces, _, _, err := tf.runTerraformCommand("workspace", false, envs, "list")
	if err != nil {
		return err
	}
	workspaces = tf.formatTerraformWorkspaces(workspaces)
	if strings.Contains(workspaces, tf.Workspace) {
		_, _, _, err := tf.runTerraformCommand("workspace", true, envs, "select", tf.Workspace)
		if err != nil {
			return err
		}
	} else {
		_, _, _, err := tf.runTerraformCommand("workspace", true, envs, "new", tf.Workspace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tf Terraform) runTerraformCommand(command string, printOutputToStdout bool, envs map[string]string, arg ...string) (string, string, int, error) {
	args := []string{command}
	args = append(args, arg...)

	expandedArgs := make([]string, 0)
	for _, p := range args {
		s := os.ExpandEnv(p)
		s = strings.TrimSpace(s)
		if s != "" {
			expandedArgs = append(expandedArgs, s)
		}
	}

	var mwout, mwerr io.Writer
	var stdout, stderr bytes.Buffer
	if printOutputToStdout {
		mwout = io.MultiWriter(os.Stdout, &stdout)
		mwerr = io.MultiWriter(os.Stderr, &stderr)
	} else {
		mwout = io.Writer(&stdout)
		mwerr = io.Writer(&stderr)
	}

	cmd := exec.Command("terraform", expandedArgs...)
	log.Printf("Running command: terraform %v", RedactSecrets(expandedArgs))
	cmd.Dir = tf.WorkingDir

	env := os.Environ()
	for k, v := range envs {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env
	cmd.Stdout = mwout
	cmd.Stderr = mwerr

	err := cmd.Run()

	// terraform plan can return 2 if there are changes to be applied, so we don't want to fail in that case
	if err != nil && cmd.ProcessState.ExitCode() != 2 {
		log.Println("Error:", err)
	}

	return stdout.String(), stderr.String(), cmd.ProcessState.ExitCode(), err
}

type StdWriter struct {
	data  []byte
	print bool
}

func (tf Terraform) formatTerraformWorkspaces(list string) string {

	list = strings.TrimSpace(list)
	char_replace := strings.NewReplacer("*", "", "\n", ",", " ", "")
	list = char_replace.Replace(list)
	return list
}

func (tf Terraform) Plan(params []string, envs map[string]string, planJsonFilePath string) (bool, string, string, error) {
	params = append(append(append(params, "-input=false"), "-no-color"), "-detailed-exitcode")
	if planJsonFilePath != "" {
		params = append(params, []string{"-out", planJsonFilePath}...)
	}
	params = append(params, "-lock-timeout=3m")
	stdout, stderr, statusCode, err := tf.runTerraformCommand("plan", true, envs, params...)
	if err != nil && statusCode != 2 {
		return false, "", "", err
	}
	return statusCode == 2, stdout, stderr, nil
}

func (tf Terraform) Show(params []string, envs map[string]string, planJsonFilePath string) (string, string, error) {
	params = append(params, []string{"-no-color", "-json", planJsonFilePath}...)
	stdout, stderr, _, err := tf.runTerraformCommand("show", false, envs, params...)
	if err != nil {
		return "", "", err
	}
	return stdout, stderr, nil
}

func RedactSecret(s string) string {
	exps := []*regexp.Regexp{
		regexp.MustCompile(`\-backend\-config\=access\_key\=(.*)`),
		regexp.MustCompile(`\-backend\-config\=secret\_key\=(.*)`),
		regexp.MustCompile(`\-backend\-config\=token\=(.*)`),
	}
	for _, e := range exps {
		x := e.FindStringSubmatch(s)
		if len(x) > 1 {
			s = strings.ReplaceAll(s, x[1], "<REDACTED>")
		}
	}
	return s
}

func RedactSecrets(secrets []string) []string {
	for i, s := range secrets {
		secrets[i] = RedactSecret(s)
	}
	return secrets
}
