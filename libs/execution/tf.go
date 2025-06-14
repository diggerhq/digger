package execution

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type TerraformExecutor interface {
	Init([]string, map[string]string) (string, string, error)
	Apply([]string, *string, map[string]string) (string, string, error)
	Destroy([]string, map[string]string) (string, string, error)
	Plan([]string, map[string]string, string, *string) (bool, string, string, error)
	Show([]string, map[string]string, string) (string, string, error)
}

type Terraform struct {
	WorkingDir string
	Workspace  string
}

func (tf Terraform) Init(params []string, envs map[string]string) (string, string, error) {
	params = append(params, "-input=false")
	params = append(params, "-no-color")
	stdout, stderr, _, err := tf.runTerraformCommand("init", true, envs, nil, params...)

	// switch to workspace for next step
	// TODO: make this an individual and isolated step
	if tf.Workspace != "default" {
		werr := tf.switchToWorkspace(envs)
		if werr != nil {
			slog.Error("Failed to switch workspace",
				"workspace", tf.Workspace,
				"error", werr)
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
	stdout, stderr, _, err := tf.runTerraformCommand("apply", true, envs, nil, params...)
	return stdout, stderr, err
}

func (tf Terraform) Destroy(params []string, envs map[string]string) (string, string, error) {
	params = append(append(append(params, "-input=false"), "-no-color"), "-auto-approve")
	stdout, stderr, _, err := tf.runTerraformCommand("destroy", true, envs, nil, params...)
	return stdout, stderr, err
}

func (tf Terraform) switchToWorkspace(envs map[string]string) error {
	workspaces, _, _, err := tf.runTerraformCommand("workspace", false, envs, nil, "list")
	if err != nil {
		return err
	}
	workspaces = tf.formatTerraformWorkspaces(workspaces)
	if strings.Contains(workspaces, tf.Workspace) {
		slog.Debug("Selecting existing workspace", "workspace", tf.Workspace)
		_, _, _, err := tf.runTerraformCommand("workspace", true, envs, nil, "select", tf.Workspace)
		if err != nil {
			return err
		}
	} else {
		slog.Debug("Creating new workspace", "workspace", tf.Workspace)
		_, _, _, err := tf.runTerraformCommand("workspace", true, envs, nil, "new", tf.Workspace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tf Terraform) runTerraformCommand(command string, printOutputToStdout bool, envs map[string]string, filterRegex *string, arg ...string) (string, string, int, error) {
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

	var regEx *regexp.Regexp
	if filterRegex != nil {
		slog.Debug("using regex for filter", "regex", *filterRegex)
		var err error
		regEx, err = regexp.Compile(*filterRegex)
		if err != nil {
			slog.Error("invalid regex for filter",
				"regex", *filterRegex,
				"error", err)
			return "", "", 0, fmt.Errorf("regex for filter is invalid: %v", err)
		}
	} else {
		slog.Debug("no regex for filter")
		regEx = nil
	}

	var mwout, mwerr io.Writer
	var stdout, stderr bytes.Buffer
	if printOutputToStdout {
		mwout = NewFilteringWriter(os.Stdout, &stdout, regEx)
		mwerr = NewFilteringWriter(os.Stderr, &stderr, regEx)
	} else {
		mwout = NewFilteringWriter(nil, &stdout, regEx)
		mwerr = NewFilteringWriter(nil, &stderr, regEx)
	}

	cmd := exec.Command("terraform", expandedArgs...)
	slog.Info("Running Terraform command",
		slog.Group("command",
			"binary", "terraform",
			"args", RedactSecrets(expandedArgs),
			"workingDir", tf.WorkingDir,
		),
	)
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
		slog.Error("Command execution failed",
			"command", "terraform",
			"exitCode", cmd.ProcessState.ExitCode(),
			"error", err,
		)
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

func (tf Terraform) Plan(params []string, envs map[string]string, planArtefactFilePath string, filterRegex *string) (bool, string, string, error) {
	params = append(append(append(params, "-input=false"), "-no-color"), "-detailed-exitcode")
	if planArtefactFilePath != "" {
		params = append(params, []string{"-out", planArtefactFilePath}...)
	}
	params = append(params, "-lock-timeout=3m")
	stdout, stderr, statusCode, err := tf.runTerraformCommand("plan", true, envs, filterRegex, params...)
	if err != nil && statusCode != 2 {
		return false, "", "", err
	}
	return statusCode == 2, stdout, stderr, nil
}

func (tf Terraform) Show(params []string, envs map[string]string, planArtefactFilePath string) (string, string, error) {
	params = append(params, []string{"-no-color", "-json", planArtefactFilePath}...)
	stdout, stderr, _, err := tf.runTerraformCommand("show", false, envs, nil, params...)
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
