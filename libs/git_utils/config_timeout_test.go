package git_utils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestReadGitTimeout(t *testing.T) {
	for _, test := range []struct {
		name    string
		files   map[string]string
		want    time.Duration
		paths   []string
		wantErr bool
	}{
		{"configured", map[string]string{"digger.yml": "git_timeout: 120\nprojects: []"}, 120 * time.Second, []string{"digger.yml"}, false},
		{"yaml fallback", map[string]string{"digger.yaml": "git_timeout: 90"}, 90 * time.Second, []string{"digger.yml", "digger.yaml"}, false},
		{"yml precedence", map[string]string{"digger.yml": "git_timeout: 60", "digger.yaml": "git_timeout: 90"}, time.Minute, []string{"digger.yml"}, false},
		{"missing files", nil, 30 * time.Second, []string{"digger.yml", "digger.yaml"}, false},
		{"omitted", map[string]string{"digger.yml": "projects: []"}, 30 * time.Second, []string{"digger.yml"}, false},
		{"zero", map[string]string{"digger.yml": "git_timeout: 0"}, 30 * time.Second, []string{"digger.yml"}, false},
		{"negative", map[string]string{"digger.yml": "git_timeout: -1"}, 30 * time.Second, []string{"digger.yml"}, false},
		{"invalid type", map[string]string{"digger.yml": "git_timeout: slow"}, 0, []string{"digger.yml"}, true},
		{"overflow", map[string]string{"digger.yml": "git_timeout: 9223372037"}, 0, []string{"digger.yml"}, true},
		{"malformed", map[string]string{"digger.yml": "git_timeout: ["}, 0, []string{"digger.yml"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var paths []string
			got, err := readGitTimeout(func(ctx context.Context, path, ref string) ([]byte, error) {
				paths = append(paths, path)
				if ref != "feature/timeout" {
					t.Fatalf("unexpected ref: %s", ref)
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("API lookup has no deadline")
				}
				if data, ok := test.files[path]; ok {
					return []byte(data), nil
				}
				return nil, fmt.Errorf("missing %s: %w", path, os.ErrNotExist)
			}, "feature/timeout")
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("got %s, %v; want %s, error=%v", got, err, test.want, test.wantErr)
			}
			if !reflect.DeepEqual(paths, test.paths) {
				t.Fatalf("read %v; want %v", paths, test.paths)
			}
		})
	}
}

func TestConfigLookupFailurePreventsClone(t *testing.T) {
	commandsPath := t.TempDir() + "/commands"
	t.Setenv("GIT_TEST_COMMANDS", commandsPath)
	installFakeGit(t, "echo clone > \"$GIT_TEST_COMMANDS\"\n")
	apiErr := errors.New("API unavailable")
	err := CloneGitRepoAndDoActionWithConfig("https://example.com/repo", "main", "", "", "", func(ctx context.Context, path, ref string) ([]byte, error) {
		if ref != "main" {
			t.Fatalf("unexpected ref: %s", ref)
		}
		return nil, apiErr
	}, func(string) error {
		t.Fatal("action called after configuration lookup failed")
		return nil
	})
	if !errors.Is(err, apiErr) {
		t.Fatalf("expected API error, got %v", err)
	}
	if _, err := os.Stat(commandsPath); !os.IsNotExist(err) {
		t.Fatal("clone started after configuration lookup failed")
	}
}
