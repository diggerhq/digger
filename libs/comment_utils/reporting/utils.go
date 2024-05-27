package reporting

import (
	"fmt"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"log"
	"time"
)

type SourceDetails struct {
	SourceLocation string   `json:"source_location"`
	CommentId      string   `json:"comment_id"`
	Projects       []string `json:"projects"`
}

func PostInitialSourceComments(ghService *dg_github.GithubService, prNumber int, impactedProjectsSourceMapping map[string]dg_configuration.ProjectToSourceMapping) ([]SourceDetails, error) {

	locations := make(map[string][]string)
	sourceDetails := make([]SourceDetails, 0)

	for projectName, sourceMapping := range impactedProjectsSourceMapping {
		for _, location := range sourceMapping.ImpactingLocations {
			locations[location] = append(locations[location], projectName)
		}
	}
	for location, projects := range locations {
		reporter := CiReporter{
			PrNumber:       prNumber,
			CiService:      ghService,
			ReportStrategy: CommentPerRunStrategy{fmt.Sprintf("Report for location: %v", location), time.Now()},
		}
		commentId, _, err := reporter.Report("Comment Reporter", func(report string) string { return "" })
		if err != nil {
			log.Printf("Error reporting source module comment: %v", err)
			return nil, fmt.Errorf("error reporting source module comment: %v", err)
		}

		sourceDetails = append(sourceDetails, SourceDetails{location, commentId, projects})
	}

	return sourceDetails, nil
}
