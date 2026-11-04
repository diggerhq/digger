package utils

import (
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"testing"
)

func TestImpactedProjectsOnlyGraph(t *testing.T) {
	p1 := configuration.Project{Name: "p1"}
	p2 := configuration.Project{Name: "p2", DependencyProjects: []string{"p1"}}
	p3 := configuration.Project{Name: "p3", DependencyProjects: []string{"p1", "p2"}}
	p4 := configuration.Project{Name: "p4"}
	p5 := configuration.Project{Name: "p5", DependencyProjects: []string{"p4"}}
	p6 := configuration.Project{Name: "p6", DependencyProjects: []string{"p1", "p2", "p3"}}
	projects := []configuration.Project{p1, p2, p3, p4, p5, p6}

	impactedProjects := map[string]configuration.Project{"p1": p1, "p2": p2, "p3": p3}

	dg, err := configuration.CreateProjectDependencyGraph(projects)

	if err != nil {
		t.Errorf("Error creating dependency graph: %v", err)
	}

	newGraph, err := ImpactedProjectsOnlyGraph(dg, impactedProjects)

	if err != nil {
		t.Errorf("Error creating impacted projects only graph: %v", err)
	}

	adjMap, err := newGraph.AdjacencyMap()

	if err != nil {
		t.Errorf("Error getting adjacency map: %v", err)
	}

	if len(adjMap) != 3 {
		t.Errorf("Expected 3 projects, got %d", len(adjMap))
	}

	if _, ok := adjMap["p1"]; !ok {
		t.Errorf("Expected p1 to be in the graph")
	}

	if _, ok := adjMap["p2"]; !ok {
		t.Errorf("Expected p2 to be in the graph")
	}

	if _, ok := adjMap["p3"]; !ok {
		t.Errorf("Expected p3 to be in the graph")
	}
}

func TestImpactedProjectsOnlyGraph2(t *testing.T) {
	p1 := configuration.Project{Name: "p1"}
	p2 := configuration.Project{Name: "p2", DependencyProjects: []string{"p1"}}
	p3 := configuration.Project{Name: "p3", DependencyProjects: []string{"p1", "p2"}}
	p4 := configuration.Project{Name: "p4"}
	p5 := configuration.Project{Name: "p5", DependencyProjects: []string{"p4"}}
	p6 := configuration.Project{Name: "p6", DependencyProjects: []string{"p1", "p2", "p3"}}
	projects := []configuration.Project{p1, p2, p3, p4, p5, p6}

	impactedProjects := map[string]configuration.Project{"p1": p1, "p2": p2, "p3": p3, "p5": p5}

	dg, err := configuration.CreateProjectDependencyGraph(projects)

	if err != nil {
		t.Errorf("Error creating dependency graph: %v", err)
	}

	newGraph, err := ImpactedProjectsOnlyGraph(dg, impactedProjects)

	if err != nil {
		t.Errorf("Error creating impacted projects only graph: %v", err)
	}

	adjMap, err := newGraph.AdjacencyMap()

	if err != nil {
		t.Errorf("Error getting adjacency map: %v", err)
	}

	if len(adjMap) != 4 {
		t.Errorf("Expected 4 projects, got %d", len(adjMap))
	}

	if _, ok := adjMap["p1"]; !ok {
		t.Errorf("Expected p1 to be in the graph")
	}

	if _, ok := adjMap["p2"]; !ok {
		t.Errorf("Expected p2 to be in the graph")
	}

	if _, ok := adjMap["p3"]; !ok {
		t.Errorf("Expected p3 to be in the graph")
	}

	if _, ok := adjMap["p5"]; !ok {
		t.Errorf("Expected p5 to be in the graph")
	}
}

func TestImpactedProjectsOnlyGraph3(t *testing.T) {
	p1 := configuration.Project{Name: "p1"}
	p2 := configuration.Project{Name: "p2", DependencyProjects: []string{"p1"}}
	p3 := configuration.Project{Name: "p3", DependencyProjects: []string{"p1", "p2"}}
	p4 := configuration.Project{Name: "p4"}
	p5 := configuration.Project{Name: "p5", DependencyProjects: []string{"p4"}}
	p6 := configuration.Project{Name: "p6", DependencyProjects: []string{"p1", "p2", "p3"}}
	projects := []configuration.Project{p1, p2, p3, p4, p5, p6}

	impactedProjects := map[string]configuration.Project{"p1": p1, "p3": p3, "p6": p6}

	dg, err := configuration.CreateProjectDependencyGraph(projects)

	if err != nil {
		t.Errorf("Error creating dependency graph: %v", err)
	}

	newGraph, err := ImpactedProjectsOnlyGraph(dg, impactedProjects)

	if err != nil {
		t.Errorf("Error creating impacted projects only graph: %v", err)
	}

	adjMap, err := newGraph.AdjacencyMap()

	if err != nil {
		t.Errorf("Error getting adjacency map: %v", err)
	}

	if len(adjMap) != 3 {
		t.Errorf("Expected 3 projects, got %d", len(adjMap))
	}

	if _, ok := adjMap["p1"]; !ok {
		t.Errorf("Expected p1 to be in the graph")
	}

	if _, ok := adjMap["p3"]; !ok {
		t.Errorf("Expected p3 to be in the graph")
	}

	if _, ok := adjMap["p6"]; !ok {
		t.Errorf("Expected p5 to be in the graph")
	}
}

func TestTraverseGraphVisitAllParentsFirst(t *testing.T) {
	p1 := configuration.Project{Name: "root1"}
	p2 := configuration.Project{Name: "root2"}
	p3 := configuration.Project{Name: "root3"}
	p4 := configuration.Project{Name: "root4"}
	p5 := configuration.Project{Name: "child1", DependencyProjects: []string{"root1", "root2"}}
	p6 := configuration.Project{Name: "child2", DependencyProjects: []string{"root1", "root2"}}
	p7 := configuration.Project{Name: "child3", DependencyProjects: []string{"child1"}}
	p8 := configuration.Project{Name: "child4", DependencyProjects: []string{"root4"}}
	projects := []configuration.Project{p1, p2, p3, p4, p5, p6, p7, p8}

	dg, err := configuration.CreateProjectDependencyGraph(projects)

	if err != nil {
		t.Errorf("Error creating dependency graph: %v", err)
	}

	visitedOrder := make(map[string]int, 0)
	order := 0
	visit := func(value string) bool {
		visitedOrder[value] = order
		order++
		return true
	}

	err = TraverseGraphVisitAllParentsFirst(dg, visit)

	if err != nil {
		t.Errorf("Error traversing graph: %v", err)
	}

	if visitedOrder["root1"] > visitedOrder["child1"] {
		t.Errorf("Expected root1 to be visited before child1")
	}

	if visitedOrder["root2"] > visitedOrder["child1"] {
		t.Errorf("Expected root2 to be visited before child1")
	}

	if visitedOrder["root1"] > visitedOrder["child2"] {
		t.Errorf("Expected root1 to be visited before child2")
	}

	if visitedOrder["root2"] > visitedOrder["child2"] {
		t.Errorf("Expected root2 to be visited before child2")
	}

	if visitedOrder["child1"] > visitedOrder["child3"] {
		t.Errorf("Expected child1 to be visited before child3")
	}

	if visitedOrder["child2"] > visitedOrder["child3"] {
		t.Errorf("Expected child2 to be visited before child3")
	}

	if visitedOrder["root4"] > visitedOrder["child1"] {
		t.Errorf("Expected root4 to be visited before child1")
	}
	if visitedOrder["root4"] > visitedOrder["child2"] {
		t.Errorf("Expected root4 to be visited before child2")
	}
	if visitedOrder["root4"] > visitedOrder["child3"] {
		t.Errorf("Expected root4 to be visited before child3")
	}
	if visitedOrder["root4"] > visitedOrder["child4"] {
		t.Errorf("Expected root4 to be visited before child4")
	}
}

// Regression test: a child with multiple impacted predecessors must keep an
// incoming edge from EVERY one of them in the collapsed graph. Previously
// CollapsedGraph returned early when the child vertex already existed, so
// only one (traversal-order dependent) parent edge survived.
func TestImpactedProjectsOnlyGraphMultiParentChildKeepsAllParentEdges(t *testing.T) {
	p1 := configuration.Project{Name: "p1"}
	p2 := configuration.Project{Name: "p2"}
	p3 := configuration.Project{Name: "p3"}
	child := configuration.Project{Name: "child", DependencyProjects: []string{"p1", "p2", "p3"}}
	projects := []configuration.Project{p1, p2, p3, child}

	impactedProjects := map[string]configuration.Project{"p1": p1, "p2": p2, "p3": p3, "child": child}

	dg, err := configuration.CreateProjectDependencyGraph(projects)
	if err != nil {
		t.Errorf("Error creating dependency graph: %v", err)
	}

	newGraph, err := ImpactedProjectsOnlyGraph(dg, impactedProjects)
	if err != nil {
		t.Errorf("Error creating impacted projects only graph: %v", err)
	}

	predMap, err := newGraph.PredecessorMap()
	if err != nil {
		t.Errorf("Error getting predecessor map: %v", err)
	}

	childPredecessors := predMap["child"]
	if len(childPredecessors) != 3 {
		t.Errorf("Expected child to have 3 predecessors, got %d", len(childPredecessors))
	}
	for _, parent := range []string{"p1", "p2", "p3"} {
		if _, ok := childPredecessors[parent]; !ok {
			t.Errorf("Expected edge %s -> child to be present in the collapsed graph", parent)
		}
	}
}
