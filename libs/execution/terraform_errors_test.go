package execution

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Stub the CLI so exit codes and both output streams can be tested without providers.
func installPlanErrorCLI(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test CLI uses a POSIX shell")
	}
	dir := t.TempDir()
	script := []byte("#!/bin/sh\nprintf 'plan output\\n'\nprintf 'plan diagnostic\\n' >&2\nexit \"$DIGGER_TEST_EXIT_CODE\"\n")
	for _, binary := range []string{"terraform", "tofu"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, binary), script, 0o700))
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestPlanPreservesOutput(t *testing.T) {
	dir := installPlanErrorCLI(t)
	for _, executor := range []struct {
		name string
		tf   TerraformExecutor
	}{
		{"terraform", Terraform{WorkingDir: dir, Workspace: "default"}},
		{"tofu", OpenTofu{WorkingDir: dir, Workspace: "default"}},
	} {
		for _, code := range []int{0, 1, 2} {
			t.Run(fmt.Sprintf("%s/exit-%d", executor.name, code), func(t *testing.T) {
				t.Setenv("DIGGER_TEST_EXIT_CODE", fmt.Sprint(code))
				changes, stdout, stderr, err := executor.tf.Plan(nil, nil, "", nil)
				assert.Equal(t, code == 2, changes)
				if code == 1 {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				assert.Equal(t, "plan output\n", stdout)
				assert.Equal(t, "plan diagnostic\n", stderr)
			})
		}
	}
}

type terraformErrorReporter struct {
	reporting.NoopReporter
	markdown bool
	comments []string
}

func (r *terraformErrorReporter) SupportsMarkdown() bool { return r.markdown }

func (r *terraformErrorReporter) Report(output string, format func(string) string) (string, string, error) {
	r.comments = append(r.comments, format(output))
	return "", "", nil
}

func TestDiggerExecutorReportsFailedStep(t *testing.T) {
	dir := installPlanErrorCLI(t)
	t.Setenv("DIGGER_TEST_EXIT_CODE", "1")
	for _, markdown := range []bool{false, true} {
		for _, step := range []string{"plan", "show", "init"} {
			t.Run(fmt.Sprintf("%s/markdown-%t", step, markdown), func(t *testing.T) {
				reporter := &terraformErrorReporter{markdown: markdown}
				executor := DiggerExecutor{
					TerraformExecutor: Terraform{WorkingDir: dir, Workspace: "default"},
					Reporter:          reporter,
					PlanPathProvider:  ProjectPathProvider{ProjectPath: dir},
					PlanStage:         &scheduler.Stage{},
					ApplyStage:        &scheduler.Stage{Steps: []scheduler.Step{{Action: "init"}}},
				}
				var err error
				if step == "init" {
					_, _, _, err = executor.Apply()
				} else {
					if step == "plan" {
						executor.PlanStage.Steps = []scheduler.Step{{Action: "plan"}}
					}
					_, _, _, _, _, err = executor.Plan()
				}
				require.Error(t, err)
				require.Len(t, reporter.comments, 1)
				assert.Contains(t, reporter.comments[0], "Error during "+step+".")
				if step == "show" {
					assert.Contains(t, reporter.comments[0], "exit status 1")
				} else {
					assert.Contains(t, reporter.comments[0], "plan diagnostic")
				}
				if step == "plan" {
					assert.Contains(t, err.Error(), "stdout: plan output")
					assert.Contains(t, err.Error(), "stderr: plan diagnostic")
				}
			})
		}
	}
}
