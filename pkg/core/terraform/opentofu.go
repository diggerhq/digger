package terraform

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

type OpenTofu struct {
	WorkingDir string
	Workspace  string
}

func (tf OpenTofu) Init(params []string, envs map[string]string) (string, string, error) {
	params = append(params, "-upgrade=true")
	params = append(params, "-input=false")
	params = append(params, "-no-color")
	stdout, stderr, _, err := tf.runOpentofuCommand(true, "init", envs, params...)
	return stdout, stderr, err
}

func (tf OpenTofu) Apply(params []string, plan *string, envs map[string]string) (string, string, error) {
	err := tf.switchToWorkspace(envs)
	if err != nil {
		log.Printf("Fatal: Error terraform to workspace %v", err)
		return "", "", err
	}
	params = append(append(append(params, "-input=false"), "-no-color"), "-auto-approve")
	if plan != nil {
		params = append(params, *plan)
	}
	stdout, stderr, _, err := tf.runOpentofuCommand(true, "apply", envs, params...)
	return stdout, stderr, err
}

func (tf OpenTofu) Plan(params []string, envs map[string]string) (bool, string, string, error) {

	workspaces, _, _, err := tf.runOpentofuCommand(false, "workspace", envs, "list")
	if err != nil {
		return false, "", "", err
	}
	workspaces = tf.formatOpentofuWorkspaces(workspaces)
	if strings.Contains(workspaces, tf.Workspace) {
		_, _, _, err := tf.runOpentofuCommand(true, "workspace", envs, "select", tf.Workspace)
		if err != nil {
			return false, "", "", err
		}
	} else {
		_, _, _, err := tf.runOpentofuCommand(true, "workspace", envs, "new", tf.Workspace)
		if err != nil {
			return false, "", "", err
		}
	}
	params = append(append(append(params, "-input=false"), "-no-color"), "-detailed-exitcode")
	stdout, stderr, statusCode, err := tf.runOpentofuCommand(true, "plan", envs, params...)
	if err != nil && statusCode != 2 {
		return false, "", "", err
	}
	return statusCode == 2, stdout, stderr, nil
}

func (tf OpenTofu) Show(params []string, envs map[string]string) (string, string, error) {
	stdout, stderr, _, err := tf.runOpentofuCommand(false, "show", envs, params...)
	if err != nil {
		return "", "", err
	}
	return stdout, stderr, nil
}

func (tf OpenTofu) Destroy(params []string, envs map[string]string) (string, string, error) {
	err := tf.switchToWorkspace(envs)
	if err != nil {
		log.Printf("Fatal: Error terraform to workspace %v", err)
		return "", "", err
	}
	params = append(append(append(params, "-input=false"), "-no-color"), "-auto-approve")
	stdout, stderr, _, err := tf.runOpentofuCommand(true, "destroy", envs, params...)
	return stdout, stderr, err
}

func (tf OpenTofu) switchToWorkspace(envs map[string]string) error {
	workspaces, _, _, err := tf.runOpentofuCommand(false, "workspace", envs, "list")
	if err != nil {
		return err
	}
	workspaces = tf.formatOpentofuWorkspaces(workspaces)
	if strings.Contains(workspaces, tf.Workspace) {
		_, _, _, err := tf.runOpentofuCommand(true, "workspace", envs, "select", tf.Workspace)
		if err != nil {
			return err
		}
	} else {
		_, _, _, err := tf.runOpentofuCommand(true, "workspace", envs, "new", tf.Workspace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tf OpenTofu) runOpentofuCommand(printOutputToStdout bool, command string, envs map[string]string, arg ...string) (string, string, int, error) {
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

	cmd := exec.Command("opentofu", expandedArgs...)
	log.Printf("Running command: opentofu %v", expandedArgs)
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

func (tf OpenTofu) formatOpentofuWorkspaces(list string) string {
	list = strings.TrimSpace(list)
	char_replace := strings.NewReplacer("*", "", "\n", ",", " ", "")
	list = char_replace.Replace(list)
	return list
}
