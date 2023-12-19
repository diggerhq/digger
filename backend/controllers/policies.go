package controllers

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/dominikbraun/graph"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io"
	"log"
	"net/http"
)

type CreatePolicyInput struct {
	Policy string
}

func FindAccessPolicy(c *gin.Context) {
	findPolicy(c, models.POLICY_TYPE_ACCESS)
}

func FindPlanPolicy(c *gin.Context) {
	findPolicy(c, models.POLICY_TYPE_PLAN)
}

func FindDriftPolicy(c *gin.Context) {
	findPolicy(c, models.POLICY_TYPE_DRIFT)
}

func findPolicy(c *gin.Context, policyType string) {
	repo := c.Param("repo")
	projectName := c.Param("projectName")
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		log.Printf("Organisation ID not found in context")
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	var policy models.Policy
	query := JoinedOrganisationRepoProjectQuery()

	if repo != "" && projectName != "" {
		err := query.
			Where("repos.name = ? AND projects.name = ? AND policies.organisation_id = ? AND policies.type = ?", repo, projectName, orgId, policyType).
			First(&policy).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.String(http.StatusNotFound, fmt.Sprintf("Could not find policy for repo %v and project name %v", repo, projectName))
			} else {
				c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
			}
			return
		}
	} else {
		c.String(http.StatusBadRequest, "Should pass repo and project name")
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, policy.Policy)
}

func FindAccessPolicyForOrg(c *gin.Context) {
	findPolicyForOrg(c, models.POLICY_TYPE_ACCESS)
}

func FindPlanPolicyForOrg(c *gin.Context) {
	findPolicyForOrg(c, models.POLICY_TYPE_PLAN)
}

func FindDriftPolicyForOrg(c *gin.Context) {
	findPolicyForOrg(c, models.POLICY_TYPE_DRIFT)
}

func findPolicyForOrg(c *gin.Context, policyType string) {
	organisation := c.Param("organisation")
	var policy models.Policy
	query := JoinedOrganisationRepoProjectQuery()

	err := query.
		Where("organisations.name = ? AND (repos.id IS NULL AND projects.id IS NULL) AND policies.type = ? ", organisation, policyType).
		First(&policy).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.String(http.StatusNotFound, "Could not find policy for organisation: "+organisation)
		} else {
			c.String(http.StatusInternalServerError, "Unknown error occurred while fetching database")
		}
		return
	}

	loggedInOrganisation := c.GetUint(middleware.ORGANISATION_ID_KEY)

	if policy.OrganisationID != loggedInOrganisation {
		log.Printf("Organisation ID %v does not match logged in organisation ID %v", policy.OrganisationID, loggedInOrganisation)
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, policy.Policy)
}

func JoinedOrganisationRepoProjectQuery() *gorm.DB {
	return models.DB.GormDB.Preload("Organisation").Preload("Repo").Preload("Project").
		Joins("LEFT JOIN repos ON policies.repo_id = repos.id").
		Joins("LEFT JOIN projects ON policies.project_id = projects.id").
		Joins("LEFT JOIN organisations ON policies.organisation_id = organisations.id")
}

func UpsertAccessPolicyForOrg(c *gin.Context) {
	upsertPolicyForOrg(c, models.POLICY_TYPE_ACCESS)
}

func UpsertPlanPolicyForOrg(c *gin.Context) {
	upsertPolicyForOrg(c, models.POLICY_TYPE_PLAN)
}

func UpsertDriftPolicyForOrg(c *gin.Context) {
	upsertPolicyForOrg(c, models.POLICY_TYPE_DRIFT)
}

func upsertPolicyForOrg(c *gin.Context, policyType string) {
	// Validate input
	policyData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		// Handle the error
		c.String(http.StatusInternalServerError, "Error reading request body")
		return
	}
	organisation := c.Param("organisation")

	org := models.Organisation{}
	orgResult := models.DB.GormDB.Where("name = ?", organisation).Take(&org)
	if orgResult.RowsAffected == 0 {
		c.String(http.StatusNotFound, "Could not find organisation: "+organisation)
		return
	}

	loggedInOrganisation := c.GetUint(middleware.ORGANISATION_ID_KEY)

	if org.ID != loggedInOrganisation {
		log.Printf("Organisation ID %v does not match logged in organisation ID %v", org.ID, loggedInOrganisation)
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

	policy := models.Policy{}

	policyResult := models.DB.GormDB.Where("organisation_id = ? AND (repo_id IS NULL AND project_id IS NULL) AND type = ?", org.ID, policyType).Take(&policy)

	if policyResult.RowsAffected == 0 {
		err := models.DB.GormDB.Create(&models.Policy{
			OrganisationID: org.ID,
			Type:           policyType,
			Policy:         string(policyData),
		}).Error

		if err != nil {
			log.Printf("Error creating policy: %v", err)
			c.String(http.StatusInternalServerError, "Error creating policy")
			return
		}
	} else {
		err := policyResult.Update("policy", string(policyData)).Error
		if err != nil {
			log.Printf("Error updating policy: %v", err)
			c.String(http.StatusInternalServerError, "Error updating policy")
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UpsertAccessPolicyForRepoAndProject(c *gin.Context) {
	upsertPolicyForRepoAndProject(c, models.POLICY_TYPE_ACCESS)
}

func UpsertPlanPolicyForRepoAndProject(c *gin.Context) {
	upsertPolicyForRepoAndProject(c, models.POLICY_TYPE_PLAN)
}

func UpsertDriftPolicyForRepoAndProject(c *gin.Context) {
	upsertPolicyForRepoAndProject(c, models.POLICY_TYPE_DRIFT)
}

func upsertPolicyForRepoAndProject(c *gin.Context, policyType string) {
	orgID, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		c.String(http.StatusUnauthorized, "Not authorized")
		return
	}

	orgID = orgID.(uint)

	// Validate input
	policyData, err := io.ReadAll(c.Request.Body)
	if err != nil {
		// Handle the error
		c.String(http.StatusInternalServerError, "Error reading request body")
		return
	}
	repo := c.Param("repo")
	projectName := c.Param("projectName")
	repoModel := models.Repo{}
	repoResult := models.DB.GormDB.Where("name = ?", repo).Take(&repoModel)
	if repoResult.RowsAffected == 0 {
		repoModel = models.Repo{
			OrganisationID: orgID.(uint),
			Name:           repo,
		}
		result := models.DB.GormDB.Create(&repoModel)
		if result.Error != nil {
			log.Printf("Error creating repo: %v", err)
			c.String(http.StatusInternalServerError, "Error creating missing repo")
			return
		}
	}

	projectModel := models.Project{}
	projectResult := models.DB.GormDB.Where("name = ?", projectName).Take(&projectModel)
	if projectResult.RowsAffected == 0 {
		projectModel = models.Project{
			OrganisationID: orgID.(uint),
			RepoID:         repoModel.ID,
			Name:           projectName,
		}
		err := models.DB.GormDB.Create(&projectModel).Error
		if err != nil {
			log.Printf("Error creating project: %v", err)
			c.String(http.StatusInternalServerError, "Error creating missing project")
			return
		}
	}

	var policy models.Policy

	policyResult := models.DB.GormDB.Where("organisation_id = ? AND repo_id = ? AND project_id = ? AND type = ?", orgID, repoModel.ID, projectModel.ID, policyType).Take(&policy)

	if policyResult.RowsAffected == 0 {
		err := models.DB.GormDB.Create(&models.Policy{
			OrganisationID: orgID.(uint),
			RepoID:         &repoModel.ID,
			ProjectID:      &projectModel.ID,
			Type:           policyType,
			Policy:         string(policyData),
		}).Error
		if err != nil {
			log.Printf("Error creating policy: %v", err)
			c.String(http.StatusInternalServerError, "Error creating policy")
			return
		}
	} else {
		err := policyResult.Update("policy", string(policyData)).Error
		if err != nil {
			log.Printf("Error updating policy: %v", err)
			c.String(http.StatusInternalServerError, "Error updating policy")
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func IssueAccessTokenForOrg(c *gin.Context) {
	organisation_ID, exists := c.Get(middleware.ORGANISATION_ID_KEY)

	if !exists {
		c.String(http.StatusUnauthorized, "Not authorized")
		return
	}

	org := models.Organisation{}
	orgResult := models.DB.GormDB.Where("id = ?", organisation_ID).Take(&org)
	if orgResult.RowsAffected == 0 {
		log.Printf("Could not find organisation: %v", organisation_ID)
		c.String(http.StatusInternalServerError, "Unexpected error")
		return
	}

	// prefixing token to make easier to retire this type of tokens later
	token := "t:" + uuid.New().String()

	err := models.DB.GormDB.Create(&models.Token{
		Value:          token,
		OrganisationID: org.ID,
		Type:           models.AccessPolicyType,
	}).Error

	if err != nil {
		log.Printf("Error creating token: %v", err)
		c.String(http.StatusInternalServerError, "Unexpected error")
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func loadDiggerConfig(configYaml *dg_configuration.DiggerConfigYaml) (*dg_configuration.DiggerConfig, graph.Graph[string, dg_configuration.Project], error) {

	err := dg_configuration.ValidateDiggerConfigYaml(configYaml, "loaded config")
	if err != nil {
		return nil, nil, fmt.Errorf("error validating config: %v", err)
	}

	config, depGraph, err := dg_configuration.ConvertDiggerYamlToConfig(configYaml)
	if err != nil {
		return nil, nil, fmt.Errorf("error converting config: %v", err)
	}

	err = dg_configuration.ValidateDiggerConfig(config)

	if err != nil {
		return nil, nil, fmt.Errorf("error validating config: %v", err)
	}
	return config, depGraph, nil
}

func GetIndependentProjects(depGraph graph.Graph[string, string], projectsToFilter []dg_configuration.Project) ([]dg_configuration.Project, error) {
	adjacencyMap, _ := depGraph.AdjacencyMap()
	res := make([]dg_configuration.Project, 0)
	for _, project := range projectsToFilter {
		if len(adjacencyMap[project.Name]) == 0 {
			res = append(res, project)
		}
	}
	return res, nil
}
