package generic

import (
	"testing"

	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/dominikbraun/graph"
	"github.com/stretchr/testify/require"
)

func TestFindAllProjectsDependantOnImpactedProjectsSharedDependency(t *testing.T) {
	projects := make(map[string]digger_config.Project)
	dependencyGraph := graph.New(func(project digger_config.Project) string { return project.Name }, graph.Directed())
	for _, name := range []string{"root-a", "root-b", "shared", "a-middle", "a-tail", "b-middle", "b-tail", "unrelated", "unrelated-tail"} {
		project := digger_config.Project{Name: name, Dir: name}
		projects[name] = project
		require.NoError(t, dependencyGraph.AddVertex(project))
	}
	for _, edge := range [][2]string{
		{"root-a", "shared"}, {"root-b", "shared"},
		{"root-a", "a-middle"}, {"a-middle", "a-tail"},
		{"root-b", "b-middle"}, {"b-middle", "b-tail"},
		{"unrelated", "unrelated-tail"},
	} {
		require.NoError(t, dependencyGraph.AddEdge(edge[0], edge[1]))
	}

	allImpacted := []string{"root-a", "root-b", "shared", "a-middle", "a-tail", "b-middle", "b-tail"}
	tests := []struct {
		name     string
		impacted []string
		expected []string
	}{
		{"direct changes survive a shared dependency", allImpacted, allImpacted},
		{"both roots include every transitive dependent", []string{"root-a", "root-b"}, allImpacted},
		{"unchanged roots do not include their private branches", []string{"root-a"}, []string{"root-a", "shared", "a-middle", "a-tail"}},
		{"no changed projects", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impacted := make([]digger_config.Project, 0, len(tt.impacted))
			for _, name := range tt.impacted {
				impacted = append(impacted, projects[name])
			}
			expected := make([]digger_config.Project, 0, len(tt.expected))
			for _, name := range tt.expected {
				expected = append(expected, projects[name])
			}

			actual, err := FindAllProjectsDependantOnImpactedProjects(impacted, dependencyGraph)
			require.NoError(t, err)
			require.ElementsMatch(t, expected, actual)
		})
	}
}
