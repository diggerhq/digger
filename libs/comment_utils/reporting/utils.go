package reporting

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/diggerhq/digger/libs/ci"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
)

type SourceDetails struct {
	SourceLocation string   `json:"source_location"`
	CommentId      string   `json:"comment_id"`
	Projects       []string `json:"projects"`
}

func PostInitialSourceComments(ghService ci.PullRequestService, prNumber int, impactedProjectsSourceMapping map[string]dg_configuration.ProjectToSourceMapping) ([]SourceDetails, error) {
	locations := make(map[string][]string)
	sourceDetails := make([]SourceDetails, 0)

	slog.Info("posting initial source comments",
		"prNumber", prNumber,
		"projectCount", len(impactedProjectsSourceMapping))

	for projectName, sourceMapping := range impactedProjectsSourceMapping {
		for _, location := range sourceMapping.ImpactingLocations {
			locations[location] = append(locations[location], projectName)
		}
	}

	for location, projects := range locations {
		slog.Debug("posting comment for location",
			"location", location,
			"projectCount", len(projects))

		reporter := CiReporter{
			PrNumber:       prNumber,
			CiService:      ghService,
			ReportStrategy: CommentPerRunStrategy{fmt.Sprintf("Report for location: %v", location), time.Now()},
		}

		commentId, _, err := reporter.Report("Comment Reporter", func(report string) string { return "" })
		if err != nil {
			slog.Error("error reporting source module comment",
				"error", err,
				"location", location,
				"prNumber", prNumber)
			return nil, fmt.Errorf("error reporting source module comment: %v", err)
		}

		slog.Info("posted comment for location",
			"location", location,
			"commentId", commentId,
			"projectCount", len(projects))

		sourceDetails = append(sourceDetails, SourceDetails{location, commentId, projects})
	}

	slog.Info("completed posting source comments",
		"commentCount", len(sourceDetails),
		"prNumber", prNumber)

	return sourceDetails, nil
}
