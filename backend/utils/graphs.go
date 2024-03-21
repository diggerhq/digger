package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/orchestrator"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/dominikbraun/graph"
	"github.com/google/uuid"
	"log"
)

// ConvertJobsToDiggerJobs jobs is map with project name as a key and a Job as a value
func ConvertJobsToDiggerJobs(jobsMap map[string]orchestrator.Job, projectMap map[string]configuration.Project, projectsGraph graph.Graph[string, configuration.Project], githubInstallationId int64, branch string, prNumber int, repoOwner string, repoName string, repoFullName string, commentId int64, diggerConfigStr string, batchType orchestrator_scheduler.DiggerBatchType) (*uuid.UUID, map[string]*models.DiggerJob, error) {
	result := make(map[string]*models.DiggerJob)

	log.Printf("Number of Jobs: %v\n", len(jobsMap))
	marshalledJobsMap := map[string][]byte{}
	for projectName, job := range jobsMap {
		marshalled, err := json.Marshal(orchestrator.JobToJson(job, projectMap[projectName]))
		if err != nil {
			return nil, nil, err
		}
		marshalledJobsMap[job.ProjectName] = marshalled
	}

	log.Printf("marshalledJobsMap: %v\n", marshalledJobsMap)

	batch, err := models.DB.CreateDiggerBatch(githubInstallationId, repoOwner, repoName, repoFullName, prNumber, diggerConfigStr, branch, batchType, &commentId)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create batch: %v", err)
	}
	graphWithImpactedProjectsOnly, err := ImpactedProjectsOnlyGraph(projectsGraph, projectMap)

	if err != nil {
		return nil, nil, err
	}

	predecessorMap, err := graphWithImpactedProjectsOnly.PredecessorMap()

	if err != nil {
		return nil, nil, err
	}
	visit := func(value string) bool {
		if predecessorMap[value] == nil || len(predecessorMap[value]) == 0 {
			fmt.Printf("no parent for %v\n", value)

			parentJob, err := models.DB.CreateDiggerJob(batch.ID, marshalledJobsMap[value], projectMap[value].WorkflowFile)
			if err != nil {
				log.Printf("failed to create a job, error: %v", err)
				return false
			}
			_, err = models.DB.CreateDiggerJobLink(parentJob.DiggerJobID, repoFullName)
			if err != nil {
				log.Printf("failed to create a digger job link")
				return false
			}
			result[value] = parentJob
			return false
		} else {
			parents := predecessorMap[value]
			for _, edge := range parents {
				parent := edge.Source
				fmt.Printf("parent: %v\n", parent)
				parentDiggerJob := result[parent]
				childJob, err := models.DB.CreateDiggerJob(batch.ID, marshalledJobsMap[value], projectMap[value].WorkflowFile)
				if err != nil {
					log.Printf("failed to create a job")
					return false
				}
				_, err = models.DB.CreateDiggerJobLink(childJob.DiggerJobID, repoFullName)
				if err != nil {
					log.Printf("failed to create a digger job link")
					return false
				}
				err = models.DB.CreateDiggerJobParentLink(parentDiggerJob.DiggerJobID, childJob.DiggerJobID)
				if err != nil {
					log.Printf("failed to create a digger job parent link")
					return false
				}
				result[value] = childJob
			}
			return false
		}
	}
	err = TraverseGraphVisitAllParentsFirst(graphWithImpactedProjectsOnly, visit)

	if err != nil {
		return nil, nil, err
	}

	return &batch.ID, result, nil
}

func TraverseGraphVisitAllParentsFirst(g graph.Graph[string, configuration.Project], visit func(value string) bool) error {
	// We need a dummy parent node that is ignored during traversal to ensure that all root nodes are visited first,
	// otherwise when looking back at all parents of a node a parent might not be visited yet and we would miss it.
	dummyParent := configuration.Project{Name: "DUMMY_PARENT_PROJECT_FOR_PROCESSING"}
	predecessorMap, err := g.PredecessorMap()
	if err != nil {
		return err
	}

	visitIgnoringDummyParent := func(value string) bool {
		if value == dummyParent.Name {
			return false
		}
		return visit(value)
	}

	err = g.AddVertex(dummyParent)
	if err != nil {
		return err
	}
	for node := range predecessorMap {
		if predecessorMap[node] == nil || len(predecessorMap[node]) == 0 {
			err := g.AddEdge(dummyParent.Name, node)
			if err != nil {
				return err
			}
		}
	}
	return graph.BFS(g, dummyParent.Name, visitIgnoringDummyParent)
}

func ImpactedProjectsOnlyGraph(projectsGraph graph.Graph[string, configuration.Project], impactedProjectMap map[string]configuration.Project) (graph.Graph[string, configuration.Project], error) {
	adjacencyMap, err := projectsGraph.AdjacencyMap()
	if err != nil {
		return nil, err
	}
	predecessorMap, err := projectsGraph.PredecessorMap()
	if err != nil {
		return nil, err
	}

	graphWithImpactedProjectsOnly := graph.NewLike(projectsGraph)

	for node := range predecessorMap {
		if predecessorMap[node] == nil || len(predecessorMap[node]) == 0 {
			err := CollapsedGraph(nil, node, adjacencyMap, graphWithImpactedProjectsOnly, impactedProjectMap)
			if err != nil {
				return nil, err
			}
		}
	}
	return graphWithImpactedProjectsOnly, nil
}

func CollapsedGraph(impactedParent *string, currentNode string, adjMap map[string]map[string]graph.Edge[string], g graph.Graph[string, configuration.Project], impactedProjects map[string]configuration.Project) error {
	// add to the resulting graph only if the project has been impacted by changes
	if _, ok := impactedProjects[currentNode]; ok {
		currentProject, ok := impactedProjects[currentNode]
		if !ok {
			return fmt.Errorf("project %s not found", currentNode)
		}
		err := g.AddVertex(currentProject)
		if err != nil {
			if errors.Is(err, graph.ErrVertexAlreadyExists) {
				return nil
			}
			return err
		}
		// process all children nodes
		for child, _ := range adjMap[currentNode] {
			err := CollapsedGraph(&currentNode, child, adjMap, g, impactedProjects)
			if err != nil {
				return err
			}
		}
		// if there is an impacted parent add an edge
		if impactedParent != nil {
			err := g.AddEdge(*impactedParent, currentNode)
			if err != nil {
				return err
			}
		}
	} else {
		// if current wasn't impacted, see children of current node and set currently known parent
		for child, _ := range adjMap[currentNode] {
			err := CollapsedGraph(impactedParent, child, adjMap, g, impactedProjects)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
