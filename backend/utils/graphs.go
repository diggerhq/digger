package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/diggerhq/digger/backend/models"
	configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/dominikbraun/graph"
	"github.com/google/uuid"
)

// ConvertJobsToDiggerJobs jobs is map with project name as a key and a Job as a value
func ConvertJobsToDiggerJobs(jobType scheduler.DiggerCommand, vcsType models.DiggerVCSType, organisationId uint, jobsMap map[string]scheduler.Job, projectMap map[string]configuration.Project, projectsGraph graph.Graph[string, configuration.Project], githubInstallationId int64, branch string, prNumber int, repoOwner string, repoName string, repoFullName string, commitSha string, commentId int64, diggerConfigStr string, gitlabProjectId int, aiSummaryCommentId string, reportTerraformOutput bool, coverAllImpactedProjects bool, VCSConnectionId *uint, batchCheckRunData *CheckRunData, jobsCheckRunIdsMap map[string]CheckRunData) (*uuid.UUID, map[string]*models.DiggerJob, error) {
	slog.Info("Converting jobs to Digger jobs",
		"jobType", jobType,
		"vcsType", vcsType,
		"organisationId", organisationId,
		"jobCount", len(jobsMap),
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"branch", branch,
		"prNumber", prNumber,
	)

	result := make(map[string]*models.DiggerJob)
	organisation, err := models.DB.GetOrganisationById(organisationId)
	if err != nil {
		slog.Error("Failed to get organisation",
			"organisationId", organisationId,
			"error", err,
		)
		return nil, nil, fmt.Errorf("error retrieving organisation")
	}
	organisationName := organisation.Name

	backendHostName := os.Getenv("HOSTNAME")

	slog.Debug("Processing jobs", "count", len(jobsMap))
	marshalledJobsMap := map[string][]byte{}
	for projectName, job := range jobsMap {
		jobToken, err := models.DB.CreateDiggerJobToken(organisationId)
		if err != nil {
			slog.Error("Failed to create job token",
				"projectName", projectName,
				"organisationId", organisationId,
				"error", err,
			)
			return nil, nil, fmt.Errorf("error creating job token")
		}

		marshalled, err := json.Marshal(scheduler.JobToJson(job, jobType, organisationName, branch, commitSha, jobToken.Value, backendHostName, projectMap[projectName]))
		if err != nil {
			slog.Error("Failed to marshal job",
				"projectName", projectName,
				"error", err,
			)
			return nil, nil, err
		}
		marshalledJobsMap[job.ProjectName] = marshalled

		slog.Debug("Marshalled job",
			"projectName", job.ProjectName,
			"dataLength", len(marshalled),
		)
	}

	var batchCheckRunId *string = nil
	var batchCheckRunUrl *string = nil
	if batchCheckRunData != nil {
		batchCheckRunId = &batchCheckRunData.Id
		batchCheckRunUrl = &batchCheckRunData.Url
	}
	batch, err := models.DB.CreateDiggerBatch(vcsType, githubInstallationId, repoOwner, repoName, repoFullName, prNumber, diggerConfigStr, branch, jobType, &commentId, gitlabProjectId, aiSummaryCommentId, reportTerraformOutput, coverAllImpactedProjects, VCSConnectionId, commitSha, batchCheckRunId, batchCheckRunUrl)
	if err != nil {
		slog.Error("Failed to create batch", "error", err)
		return nil, nil, fmt.Errorf("failed to create batch: %v", err)
	}

	slog.Debug("Created batch", "batchId", batch.ID)

	graphWithImpactedProjectsOnly, err := ImpactedProjectsOnlyGraph(projectsGraph, projectMap)
	if err != nil {
		slog.Error("Failed to create impacted projects graph", "error", err)
		return nil, nil, err
	}

	predecessorMap, err := graphWithImpactedProjectsOnly.PredecessorMap()
	if err != nil {
		slog.Error("Failed to get predecessor map", "error", err)
		return nil, nil, err
	}

	visit := func(value string) bool {
		var jobCheckRunId *string = nil
		var jobCheckRunUrl *string = nil
		if jobsCheckRunIdsMap != nil {
			if v, ok := jobsCheckRunIdsMap[value]; ok {
				jobCheckRunId = &v.Id
				jobCheckRunUrl = &v.Url
			}
		}
		if predecessorMap[value] == nil || len(predecessorMap[value]) == 0 {
			slog.Debug("Processing node with no parents", "projectName", value)
			parentJob, err := models.DB.CreateDiggerJob(batch.ID, marshalledJobsMap[value], projectMap[value].WorkflowFile, jobCheckRunId, jobCheckRunUrl)
			if err != nil {
				slog.Error("Failed to create job",
					"projectName", value,
					"batchId", batch.ID,
					"error", err,
				)
				return false
			}

			_, err = models.DB.CreateDiggerJobLink(parentJob.DiggerJobID, repoFullName)
			if err != nil {
				slog.Error("Failed to create job link",
					"jobId", parentJob.DiggerJobID,
					"repoFullName", repoFullName,
					"error", err,
				)
				return false
			}

			result[value] = parentJob
			slog.Debug("Created job with no parents",
				"projectName", value,
				"jobId", parentJob.DiggerJobID,
			)
			return false
		} else {
			parents := predecessorMap[value]
			slog.Debug("Processing node with parents",
				"projectName", value,
				"parentCount", len(parents),
			)

			for _, edge := range parents {
				parent := edge.Source
				parentDiggerJob := result[parent]

				childJob, err := models.DB.CreateDiggerJob(batch.ID, marshalledJobsMap[value], projectMap[value].WorkflowFile, jobCheckRunId, jobCheckRunUrl)
				if err != nil {
					slog.Error("Failed to create child job",
						"projectName", value,
						"parentProject", parent,
						"batchId", batch.ID,
						"error", err,
					)
					return false
				}

				_, err = models.DB.CreateDiggerJobLink(childJob.DiggerJobID, repoFullName)
				if err != nil {
					slog.Error("Failed to create job link",
						"jobId", childJob.DiggerJobID,
						"repoFullName", repoFullName,
						"error", err,
					)
					return false
				}

				err = models.DB.CreateDiggerJobParentLink(parentDiggerJob.DiggerJobID, childJob.DiggerJobID)
				if err != nil {
					slog.Error("Failed to create job parent link",
						"parentJobId", parentDiggerJob.DiggerJobID,
						"childJobId", childJob.DiggerJobID,
						"error", err,
					)
					return false
				}

				result[value] = childJob
				slog.Debug("Created job with parent",
					"projectName", value,
					"jobId", childJob.DiggerJobID,
					"parentProject", parent,
					"parentJobId", parentDiggerJob.DiggerJobID,
				)
			}
			return false
		}
	}

	err = TraverseGraphVisitAllParentsFirst(graphWithImpactedProjectsOnly, visit)
	if err != nil {
		slog.Error("Failed to traverse graph", "error", err)
		return nil, nil, err
	}

	slog.Info("Successfully converted jobs to Digger jobs",
		"batchId", batch.ID,
		"diggerJobCount", len(result),
	)

	return &batch.ID, result, nil
}

func TraverseGraphVisitAllParentsFirst(g graph.Graph[string, configuration.Project], visit func(value string) bool) error {
	slog.Debug("Traversing graph, visiting all parents first")

	// We need a dummy parent node that is ignored during traversal to ensure that all root nodes are visited first,
	// otherwise when looking back at all parents of a node a parent might not be visited yet and we would miss it.
	dummyParent := configuration.Project{Name: "DUMMY_PARENT_PROJECT_FOR_PROCESSING"}
	predecessorMap, err := g.PredecessorMap()
	if err != nil {
		slog.Error("Failed to get predecessor map", "error", err)
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
		slog.Error("Failed to add dummy parent vertex", "error", err)
		return err
	}

	rootCount := 0
	for node := range predecessorMap {
		if predecessorMap[node] == nil || len(predecessorMap[node]) == 0 {
			err := g.AddEdge(dummyParent.Name, node)
			if err != nil {
				slog.Error("Failed to add edge from dummy parent",
					"node", node,
					"error", err,
				)
				return err
			}
			rootCount++
		}
	}

	slog.Debug("Added dummy parent to root nodes", "rootNodeCount", rootCount)
	return graph.BFS(g, dummyParent.Name, visitIgnoringDummyParent)
}

func ImpactedProjectsOnlyGraph(projectsGraph graph.Graph[string, configuration.Project], impactedProjectMap map[string]configuration.Project) (graph.Graph[string, configuration.Project], error) {
	slog.Debug("Creating graph with only impacted projects",
		"totalProjects", len(impactedProjectMap),
	)

	adjacencyMap, err := projectsGraph.AdjacencyMap()
	if err != nil {
		slog.Error("Failed to get adjacency map", "error", err)
		return nil, err
	}

	predecessorMap, err := projectsGraph.PredecessorMap()
	if err != nil {
		slog.Error("Failed to get predecessor map", "error", err)
		return nil, err
	}

	graphWithImpactedProjectsOnly := graph.NewLike(projectsGraph)

	rootCount := 0
	for node := range predecessorMap {
		if predecessorMap[node] == nil || len(predecessorMap[node]) == 0 {
			err := CollapsedGraph(nil, node, adjacencyMap, graphWithImpactedProjectsOnly, impactedProjectMap)
			if err != nil {
				slog.Error("Failed to collapse graph",
					"node", node,
					"error", err,
				)
				return nil, err
			}
			rootCount++
		}
	}

	slog.Debug("Created impacted projects graph",
		"rootNodeCount", rootCount,
	)
	return graphWithImpactedProjectsOnly, nil
}

func CollapsedGraph(impactedParent *string, currentNode string, adjMap map[string]map[string]graph.Edge[string], g graph.Graph[string, configuration.Project], impactedProjects map[string]configuration.Project) error {
	// add to the resulting graph only if the project has been impacted by changes
	if _, ok := impactedProjects[currentNode]; ok {
		currentProject, ok := impactedProjects[currentNode]
		if !ok {
			slog.Error("Project not found", "projectName", currentNode)
			return fmt.Errorf("project %s not found", currentNode)
		}

		err := g.AddVertex(currentProject)
		if err != nil {
			if errors.Is(err, graph.ErrVertexAlreadyExists) {
				return nil
			}
			slog.Error("Failed to add vertex",
				"projectName", currentNode,
				"error", err,
			)
			return err
		}

		slog.Debug("Added impacted project to graph", "projectName", currentNode)

		// process all children nodes
		for child := range adjMap[currentNode] {
			err := CollapsedGraph(&currentNode, child, adjMap, g, impactedProjects)
			if err != nil {
				return err
			}
		}

		// if there is an impacted parent add an edge
		if impactedParent != nil {
			err := g.AddEdge(*impactedParent, currentNode)
			if err != nil {
				slog.Error("Failed to add edge",
					"parent", *impactedParent,
					"child", currentNode,
					"error", err,
				)
				return err
			}

			slog.Debug("Added edge between impacted projects",
				"parent", *impactedParent,
				"child", currentNode,
			)
		}
	} else {
		// if current wasn't impacted, see children of current node and set currently known parent
		for child := range adjMap[currentNode] {
			err := CollapsedGraph(impactedParent, child, adjMap, g, impactedProjects)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
