package execution

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

type OpenTofu struct {
	WorkingDir string
	Workspace  string
}

func (tf OpenTofu) Init(params []string, envs map[string]string) (string, string, error) {
	params = append(params, "-input=false")
	params = append(params, "-no-color")
	stdout, stderr, _, err := tf.runOpentofuCommand("init", true, envs, nil, params...)
	return stdout, stderr, err
}

func (tf OpenTofu) Apply(params []string, plan *string, envs map[string]string) (string, string, error) {
	if tf.Workspace != "default" {
		err := tf.switchToWorkspace(envs)
		if err != nil {
			slog.Error("Error switching to workspace",
				"workspace", tf.Workspace,
				"error", err)
			return "", "", err
		}
	}
	params = append(params, []string{"-lock-timeout=3m"}...)
	params = append(append(append(params, "-input=false"), "-no-color"), "-auto-approve")
	if plan != nil {
		params = append(params, *plan)
	}
	stdout, stderr, _, err := tf.runOpentofuCommand("apply", true, envs, nil, params...)
	return stdout, stderr, err
}

func (tf OpenTofu) Plan(params []string, envs map[string]string, planArtefactFilePath string, filterRegex *string) (bool, string, string, error) {
	if tf.Workspace != "default" {
		err := tf.switchToWorkspace(envs)
		if err != nil {
			slog.Error("Error switching to workspace",
				"workspace", tf.Workspace,
				"error", err)
			return false, "", "", err
		}
	}
	if planArtefactFilePath != "" {
		params = append(params, []string{"-out", planArtefactFilePath}...)
	}
	params = append(params, "-lock-timeout=3m")
	params = append(append(append(params, "-input=false"), "-no-color"), "-detailed-exitcode")
	stdout, stderr, statusCode, err := tf.runOpentofuCommand("plan", true, envs, filterRegex, params...)
	if err != nil && statusCode != 2 {
		return false, "", "", err
	}
	return statusCode == 2, stdout, stderr, nil
}

func (tf OpenTofu) Show(params []string, envs map[string]string, planArtefactFilePath string, b bool) (string, string, error) {
	params = append(params, []string{"-no-color", "-json", planArtefactFilePath}...)
	stdout, stderr, _, err := tf.runOpentofuCommand("show", false, envs, nil, params...)
	if err != nil {
		return "", "", err
	}
	return stdout, stderr, nil
}

func (tf OpenTofu) Destroy(params []string, envs map[string]string) (string, string, error) {
	if tf.Workspace != "default" {
		err := tf.switchToWorkspace(envs)
		if err != nil {
			slog.Error("Error switching to workspace",
				"workspace", tf.Workspace,
				"error", err)
			return "", "", err
		}
	}
	params = append(append(append(params, "-input=false"), "-no-color"), "-auto-approve")
	stdout, stderr, _, err := tf.runOpentofuCommand("destroy", true, envs, nil, params...)
	return stdout, stderr, err
}

func (tf OpenTofu) switchToWorkspace(envs map[string]string) error {
	workspaces, _, _, err := tf.runOpentofuCommand("workspace", false, envs, nil, "list")
	if err != nil {
		return err
	}
	workspaces = tf.formatOpentofuWorkspaces(workspaces)
	if strings.Contains(workspaces, tf.Workspace) {
		slog.Debug("Selecting existing workspace", "workspace", tf.Workspace)
		_, _, _, err := tf.runOpentofuCommand("workspace", true, envs, nil, "select", tf.Workspace)
		if err != nil {
			return err
		}
	} else {
		slog.Debug("Creating new workspace", "workspace", tf.Workspace)
		_, _, _, err := tf.runOpentofuCommand("workspace", true, envs, nil, "new", tf.Workspace)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tf OpenTofu) runOpentofuCommand(command string, printOutputToStdout bool, envs map[string]string, filterRegex *string, arg ...string) (string, string, int, error) {
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

	regEx, err := stringToRegex(filterRegex)
	if err != nil {
		return "", "", 0, err
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

	cmd := exec.Command("tofu", expandedArgs...)
	slog.Info("Running OpenTofu command",
		slog.Group("command",
			"binary", "tofu",
			"args", expandedArgs,
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

	err = cmd.Run()

	// terraform plan can return 2 if there are changes to be applied, so we don't want to fail in that case
	if err != nil && cmd.ProcessState.ExitCode() != 2 {
		slog.Error("Command execution failed",
			"command", "tofu",
			"args", expandedArgs,
			"exitCode", cmd.ProcessState.ExitCode(),
			"error", err,
		)
	}

	return stdout.String(), stderr.String(), cmd.ProcessState.ExitCode(), err
}

func (tf OpenTofu) formatOpentofuWorkspaces(list string) string {
	list = strings.TrimSpace(list)
	char_replace := strings.NewReplacer("*", "", "\n", ",", " ", "")
	list = char_replace.Replace(list)
	return list
}
