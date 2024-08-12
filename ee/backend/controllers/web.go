package controllers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/gin-gonic/gin"
	"github.com/robert-nix/ansihtml"
	"golang.org/x/exp/maps"
)

type WebController struct {
	Config *config.Config
}

func (web *WebController) validateRequestProjectId(c *gin.Context) (*models.Project, bool) {
	projectId64, err := strconv.ParseUint(c.Param("projectid"), 10, 32)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to parse project id")
		return nil, false
	}
	projectId := uint(projectId64)
	projects, done := models.DB.GetProjectsFromContext(c, middleware.ORGANISATION_ID_KEY)
	if !done {
		return nil, false
	}

	for _, p := range projects {
		if projectId == p.ID {
			return &p, true
		}
	}

	c.String(http.StatusForbidden, "Not allowed to access this resource")
	return nil, false
}

func (web *WebController) ProjectsPage(c *gin.Context) {
	projects, done := models.DB.GetProjectsFromContext(c, middleware.ORGANISATION_ID_KEY)
	if !done {
		return
	}

	pageContext := services.GetMessages(c)
	maps.Copy(pageContext, gin.H{
		"Projects": projects,
	})
	c.HTML(http.StatusOK, "projects.tmpl", pageContext)
}

func (web *WebController) ReposPage(c *gin.Context) {
	repos, done := models.DB.GetReposFromContext(c, middleware.ORGANISATION_ID_KEY)
	if !done {
		return
	}

	pageContext := services.GetMessages(c)

	maps.Copy(pageContext, gin.H{
		"Repos": repos,
		//"GithubApp": githubApp,
	})
	c.HTML(http.StatusOK, "repos.tmpl", pageContext)
}

func (web *WebController) RunsPage(c *gin.Context) {
	runs, done := models.DB.GetProjectRunsFromContext(c, middleware.ORGANISATION_ID_KEY)
	if !done {
		return
	}
	context := gin.H{
		"Runs": runs,
	}
	c.HTML(http.StatusOK, "runs.tmpl", context)
}

func (web *WebController) PoliciesPage(c *gin.Context) {
	policies, done := models.DB.GetPoliciesFromContext(c, middleware.ORGANISATION_ID_KEY)
	if !done {
		return
	}

	pageContext := services.GetMessages(c)
	maps.Copy(pageContext, gin.H{
		"Policies": policies,
	})
	c.HTML(http.StatusOK, "policies.tmpl", pageContext)
}

func (web *WebController) AddPolicyPage(c *gin.Context) {
	if c.Request.Method == "GET" {
		message := ""
		projects, done := models.DB.GetProjectsFromContext(c, middleware.ORGANISATION_ID_KEY)
		if !done {
			return
		}

		policyTypes := make([]string, 0)
		policyTypes = append(policyTypes, "drift")
		policyTypes = append(policyTypes, "plan")
		policyTypes = append(policyTypes, "access")

		log.Printf("projects: %v\n", projects)

		c.HTML(http.StatusOK, "policy_add.tmpl", gin.H{
			"Message": message, "Projects": projects, "PolicyTypes": policyTypes,
		})
	} else if c.Request.Method == "POST" {
		policyText := c.PostForm("policytext")
		if policyText == "" {
			message := "Policy can't be empty"
			services.AddWarning(c, message)
			pageContext := services.GetMessages(c)
			c.HTML(http.StatusOK, "policy_add.tmpl", pageContext)
		}

		policyType := c.PostForm("policytype")
		projectIdStr := c.PostForm("projectid")
		projectId64, err := strconv.ParseUint(projectIdStr, 10, 32)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to parse policy id")
			return
		}
		projectId := uint(projectId64)
		project, ok := models.DB.GetProjectByProjectId(c, projectId, middleware.ORGANISATION_ID_KEY)
		if !ok {
			log.Printf("Failed to fetch specified project by id: %v, %v\n", projectIdStr, err)
			message := "Failed to create a policy"
			services.AddError(c, message)
			pageContext := services.GetMessages(c)
			c.HTML(http.StatusOK, "policy_add.tmpl", pageContext)
		}

		log.Printf("repo: %v\n", project.Repo)

		policy := models.Policy{Project: project, Policy: policyText, Type: policyType, Organisation: project.Organisation, Repo: project.Repo}

		err = models.DB.GormDB.Create(&policy).Error
		if err != nil {
			log.Printf("Failed to create a new policy, %v\n", err)
			message := "Failed to create a policy"
			services.AddError(c, message)
			pageContext := services.GetMessages(c)
			c.HTML(http.StatusOK, "policy_add.tmpl", pageContext)
		}

		c.Redirect(http.StatusFound, "/policies")
	}
}

func (web *WebController) PolicyDetailsPage(c *gin.Context) {
	policyId64, err := strconv.ParseUint(c.Param("policyid"), 10, 32)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to parse policy id")
		return
	}
	policyId := uint(policyId64)
	policy, ok := models.DB.GetPolicyByPolicyId(c, policyId, middleware.ORGANISATION_ID_KEY)
	if !ok {
		return
	}

	pageContext := services.GetMessages(c)
	maps.Copy(pageContext, gin.H{"Policy": policy})
	c.HTML(http.StatusOK, "policy_details.tmpl", pageContext)
}

func (web *WebController) ProjectDetailsPage(c *gin.Context) {
	project, ok := web.validateRequestProjectId(c)
	if !ok {
		return
	}

	pageContext := services.GetMessages(c)
	maps.Copy(pageContext, gin.H{"Project": project})
	c.HTML(http.StatusOK, "project_details.tmpl", pageContext)
}

func (web *WebController) RunDetailsPage(c *gin.Context) {
	runId64, err := strconv.ParseUint(c.Param("runid"), 10, 32)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to parse project run id")
		return
	}
	runId := uint(runId64)
	run, ok := models.DB.GetProjectByRunId(c, runId, middleware.ORGANISATION_ID_KEY)
	if !ok {
		return
	}

	stateSyncOutput := ""
	terraformPlanOutput := ""
	runOutput := string(ansihtml.ConvertToHTMLWithClasses([]byte(run.Output), "terraform-output-", true))
	runOutput = strings.Replace(runOutput, "  ", "&nbsp;&nbsp;", -1)
	runOutput = strings.Replace(runOutput, "\n", "<br>\n", -1)

	planIndex := strings.Index(runOutput, "Terraform used the selected providers to generate the following execution")
	if planIndex != -1 {
		stateSyncOutput = runOutput[:planIndex]
		terraformPlanOutput = runOutput[planIndex:]

		pageContext := services.GetMessages(c)
		maps.Copy(pageContext, gin.H{
			"Run":                      run,
			"TerraformStateSyncOutput": template.HTML(stateSyncOutput),
			"TerraformPlanOutput":      template.HTML(terraformPlanOutput),
		})
		c.HTML(http.StatusOK, "run_details.tmpl", pageContext)
	} else {
		pageContext := services.GetMessages(c)
		maps.Copy(pageContext, gin.H{
			"Run":       run,
			"RunOutput": template.HTML(runOutput),
		})
		c.HTML(http.StatusOK, "run_details.tmpl", pageContext)
	}
}

func (web *WebController) ProjectDetailsUpdatePage(c *gin.Context) {
	project, ok := web.validateRequestProjectId(c)
	if !ok {
		return
	}

	projectName := c.PostForm("project_name")
	if projectName != project.Name {
		project.Name = projectName
		models.DB.GormDB.Save(project)
		log.Printf("project name has been updated to %s\n", projectName)
		services.AddMessage(c, "Project has been updated successfully")
	}

	pageContext := services.GetMessages(c)
	maps.Copy(pageContext, gin.H{
		"Project": project,
	})
	c.HTML(http.StatusOK, "project_details.tmpl", pageContext)
}

func (web *WebController) PolicyDetailsUpdatePage(c *gin.Context) {
	policyId64, err := strconv.ParseUint(c.Param("policyid"), 10, 32)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to parse policy id")
		return
	}
	policyId := uint(policyId64)
	policy, ok := models.DB.GetPolicyByPolicyId(c, policyId, middleware.ORGANISATION_ID_KEY)
	if !ok {
		return
	}

	policyText := c.PostForm("policy")
	log.Printf("policyText: %v\n", policyText)

	if policyText == "" {
		services.AddWarning(c, "Policy can't be empty.")
	} else if policyText != policy.Policy {
		policy.Policy = policyText
		models.DB.GormDB.Save(policy)
		log.Printf("Policy has been updated. policy id: %v\n", policy.ID)
		services.AddMessage(c, "Policy has been updated successfully")
		c.Redirect(http.StatusFound, "/policies")
		return
	} else {
		services.AddMessage(c, "No changes to policy")
	}

	pageContext := services.GetMessages(c)
	maps.Copy(pageContext, gin.H{
		"Policy": policy,
	})
	c.HTML(http.StatusOK, "policy_details.tmpl", pageContext)
}
