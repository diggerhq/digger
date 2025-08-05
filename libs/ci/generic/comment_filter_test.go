package generic

import (
	"github.com/diggerhq/digger/libs/digger_config"
	"reflect"
	"strings"
	"testing"
)

func TestFilterOutProjectsFromComment(t *testing.T) {
	projects := []digger_config.Project{
		{Name: "proj1", Dir: "/app", Layer: 1},
		{Name: "proj2", Dir: "/db", Layer: 2},
	}

	tests := []struct {
		name      string
		comment   string
		want      []digger_config.Project
		wantErr   bool
		errorText string
	}{
		{
			name:    "No flags returns all projects",
			comment: "digger plan",
			want:    projects,
		},
		{
			name:    "Filter by layer",
			comment: "digger plan --layer 1",
			want:    []digger_config.Project{{Name: "proj1", Dir: "/app", Layer: 1}},
		},
		{
			name:    "Filter by project",
			comment: "digger plan --project proj1",
			want:    []digger_config.Project{{Name: "proj1", Dir: "/app", Layer: 1}},
		},
		{
			name:    "Filter by directory",
			comment: "digger plan --directory /app",
			want:    []digger_config.Project{{Name: "proj1", Dir: "/app", Layer: 1}},
		},
		{
			name:      "Invalid project name error",
			comment:   "digger plan --project unknown",
			wantErr:   true,
			errorText: "project unknown not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterOutProjectsFromComment(projects, tt.comment)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorText) {
					t.Errorf("expected error containing %q, got %v", tt.errorText, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
