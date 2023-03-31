package digger

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseWorkspace(t *testing.T) {
	var commentTests = []struct {
		in  string
		out string
		err bool
	}{
		{"test", "", false},
		{"test -w workspace", "workspace", false},
		{"test -w workspace -w workspace2", "", true},
		{"test -w", "", true},
	}

	for _, tt := range commentTests {
		out, err := parseWorkspace(tt.in)
		if tt.err {
			if err == nil {
				t.Errorf("parseWorkspace(%q) = %q, want error", tt.in, out)
			}
		} else {
			if err != nil {
				t.Errorf("parseWorkspace(%q) = %q, want %q", tt.in, err, tt.out)
			}
			if out != tt.out {
				t.Errorf("parseWorkspace(%q) = %q, want %q", tt.in, out, tt.out)
			}
		}
	}
}

func TestDetectCIGitHub(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "github")
	ci := DetectCI()
	assert.Equal(t, GitHub, ci)
}

func TestDetectCINone(t *testing.T) {
	ci := DetectCI()
	assert.Equal(t, None, ci)
}

func TestDetectCIBitBucket(t *testing.T) {
	t.Setenv("BITBUCKET_BUILD_NUMBER", "212")
	ci := DetectCI()
	assert.Equal(t, BitBucket, ci)
}

func TestDetectCIGitLab(t *testing.T) {
	t.Setenv("GITLAB_CI", "gitlab")
	ci := DetectCI()
	assert.Equal(t, GitLab, ci)
}
