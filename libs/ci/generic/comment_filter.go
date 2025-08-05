package generic

import (
	"fmt"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/samber/lo"
)

func FilterOutProjectsFromComment(impactedProjects []digger_config.Project, comment string) ([]digger_config.Project, error) {
	var filteredProjects []digger_config.Project
	commentParts, valid, err := ParseDiggerCommentFlags(comment)
	if !valid {
		return nil, fmt.Errorf("invalid comment: %v", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse comment %v", err)
	}

	// filtering by layer
	if commentParts.Layer != -1 {
		filteredProjects = lo.Filter(impactedProjects, func(project digger_config.Project, _ int) bool {
			return int(project.Layer) == commentParts.Layer
		})
		return filteredProjects, nil
	}

	// filtering by projects and directories
	if commentParts.Projects != nil || commentParts.Directories != nil {
		if commentParts.Projects != nil {
			// check that projects are in the list
			for _, project := range commentParts.Projects {
				if !lo.ContainsBy(impactedProjects, func(p digger_config.Project) bool {
					return p.Name == project
				}) {
					return nil, fmt.Errorf("project %v not found in the list of impacted projects", project)
				}
			}
			filteredProjects = lo.Filter(impactedProjects, func(project digger_config.Project, _ int) bool {
				return lo.Contains(commentParts.Projects, project.Name)
			})
		}
		if commentParts.Directories != nil {
			filteredDirectoriesProjects := lo.Filter(impactedProjects, func(project digger_config.Project, _ int) bool {
				return lo.Contains(commentParts.Directories, project.Dir)
			})
			filteredProjects = append(filteredProjects, filteredDirectoriesProjects...)
		}

		return filteredProjects, nil
	}

	// if nothing specified in flags, we will return the original list
	return impactedProjects, nil

}
