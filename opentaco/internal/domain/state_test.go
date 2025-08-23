package domain

import (
	"testing"
)

func TestValidateStateID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid simple ID",
			id:      "my-state",
			wantErr: false,
		},
		{
			name:    "valid nested ID",
			id:      "my-project/prod/vpc",
			wantErr: false,
		},
		{
			name:    "empty ID",
			id:      "",
			wantErr: true,
		},
		{
			name:    "ID with ..",
			id:      "my-project/../evil",
			wantErr: true,
		},
		{
			name:    "just slashes",
			id:      "///",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStateID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStateID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeStateID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "simple ID",
			id:   "my-state",
			want: "my-state",
		},
		{
			name: "leading/trailing slashes",
			id:   "/my-state/",
			want: "my-state",
		},
		{
			name: "multiple slashes",
			id:   "my//project///prod",
			want: "my/project/prod",
		},
		{
			name: "complex path",
			id:   "///my/project//prod/vpc///",
			want: "my/project/prod/vpc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeStateID(tt.id)
			if got != tt.want {
				t.Errorf("NormalizeStateID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterStatesByPrefix(t *testing.T) {
	states := []*State{
		{ID: "project-a/dev/vpc"},
		{ID: "project-a/prod/vpc"},
		{ID: "project-b/dev/vpc"},
		{ID: "project-b/prod/vpc"},
		{ID: "global/dns"},
	}

	tests := []struct {
		name   string
		prefix string
		want   int
	}{
		{
			name:   "empty prefix",
			prefix: "",
			want:   5,
		},
		{
			name:   "project-a prefix",
			prefix: "project-a",
			want:   2,
		},
		{
			name:   "project-b/prod prefix",
			prefix: "project-b/prod",
			want:   1,
		},
		{
			name:   "non-existent prefix",
			prefix: "project-c",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterStatesByPrefix(states, tt.prefix)
			if len(filtered) != tt.want {
				t.Errorf("FilterStatesByPrefix() returned %d states, want %d", len(filtered), tt.want)
			}
		})
	}
}