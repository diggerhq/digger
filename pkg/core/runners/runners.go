package runners

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

type CommandRun interface {
	Run(workingDir string, shell string, commands []string) (string, string, error)
}

type CommandRunner struct {
}

func (c CommandRunner) Run(workingDir string, shell string, commands []string) (string, string, error) {
	var args []string
	if shell == "" {
		shell = "bash"
		args = []string{"-eo", "pipefail"}
	}

	scriptFile, err := ioutil.TempFile("", "run-script")
	if err != nil {
		return "", "", fmt.Errorf("error creating script file: %v", err)
	}
	defer os.Remove(scriptFile.Name())

	for _, command := range commands {
		_, err := scriptFile.WriteString(command + "\n")
		if err != nil {
			return "", "", fmt.Errorf("error writing to script file: %v", err)
		}
	}
	args = append(args, scriptFile.Name())

	cmd := exec.Command(shell, args...)
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	mwout := io.MultiWriter(os.Stdout, &stdout)
	mwerr := io.MultiWriter(os.Stderr, &stderr)
	cmd.Stdout = mwout
	cmd.Stderr = mwerr
	err = cmd.Run()

	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("error: %v", err)
	}

	return stdout.String(), stderr.String(), err
}
