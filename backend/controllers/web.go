package controllers

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/gin-gonic/gin"
	"github.com/robert-nix/ansihtml"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
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

	githubAppId := os.Getenv("GITHUB_APP_ID")
	githubApp, err := models.DB.GetGithubApp(githubAppId)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to find GitHub app")
		return
	}

	pageContext := services.GetMessages(c)

	maps.Copy(pageContext, gin.H{
		"Repos":     repos,
		"GithubApp": githubApp,
	})
	c.HTML(http.StatusOK, "repos.tmpl", pageContext)
}

func (web *WebController) RunsPage(c *gin.Context) {
	runs, done := models.DB.GetProjectRunsFromContext(c, middleware.ORGANISATION_ID_KEY)
	if !done {
		return
	}

	pageContext := services.GetMessages(c)
	maps.Copy(pageContext, gin.H{
		"Runs": runs,
	})
	c.HTML(http.StatusOK, "runs.tmpl", pageContext)
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
	organisationId, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	if !exists {
		c.String(http.StatusForbidden, "Not allowed to access this resource")
		return
	}

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
		if projectIdStr != "" {
			projectId64, err := strconv.ParseUint(projectIdStr, 10, 32)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to parse project id")
				return
			}
			projectIdPtr := uint(projectId64)
			projectId := &projectIdPtr
			project, ok := models.DB.GetProjectByProjectId(c, *projectId, middleware.ORGANISATION_ID_KEY)
			if !ok {
				log.Printf("Failed to fetch specified project by id: %v, %v\n", projectIdStr, err)
				message := "Failed to create a policy"
				services.AddError(c, message)
				pageContext := services.GetMessages(c)
				c.HTML(http.StatusOK, "policy_add.tmpl", pageContext)
			}
			log.Printf("repo: %v\n", project.Repo)
			policy := models.Policy{ProjectID: projectId, Policy: policyText, Type: policyType, Organisation: project.Organisation, Repo: project.Repo}
			err = models.DB.GormDB.Create(&policy).Error
			if err != nil {
				log.Printf("Failed to create a new policy, %v\n", err)
				message := "Failed to create a policy"
				services.AddError(c, message)
				pageContext := services.GetMessages(c)
				c.HTML(http.StatusOK, "policy_add.tmpl", pageContext)
			}

		} else {
			org, err := models.DB.GetOrganisationById(organisationId)
			if err = models.DB.UpsertPolicyForOrg(policyType, *org, policyText); err != nil {
				c.String(http.StatusInternalServerError, "Error creating policy for organisation: %v", org)
			}
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

func (web *WebController) RedirectToLoginSubdomainIfDiggerDevOtherwiseToProjects(context *gin.Context) {
	host := context.Request.Host
	if strings.Contains(host, "digger.dev") || strings.Contains(host, "uselemon.cloud") {
		hostParts := strings.Split(host, ".")
		if len(hostParts) > 2 {
			hostParts[0] = "login"
			host = strings.Join(hostParts, ".")
		}
		context.Redirect(http.StatusMovedPermanently, fmt.Sprintf("https://%s", host))

	} else {
		context.Redirect(http.StatusMovedPermanently, "/projects")
	}
}

func (web *WebController) UpdateRepoPage(c *gin.Context) {
	repoId := c.Param("repoid")
	if repoId == "" {
		c.String(http.StatusInternalServerError, "Repo ID can't be empty")
		return
	}
	orgId, exists := c.Get(middleware.ORGANISATION_ID_KEY)
	if !exists {
		log.Printf("Org %v not found in the context\n", middleware.ORGANISATION_ID_KEY)
		c.String(http.StatusInternalServerError, "Not allowed to access this resource")
		return
	}

	repo, err := models.DB.GetRepoById(orgId, repoId)
	if err != nil {
		c.String(http.StatusForbidden, "Failed to find repo")
		return
	}

	if c.Request.Method == "GET" {
		pageContext := services.GetMessages(c)
		maps.Copy(pageContext, gin.H{
			"Repo": repo,
		})
		c.HTML(http.StatusOK, "repo_add.tmpl", pageContext)
		return
	} else if c.Request.Method == "POST" {
		diggerConfigYaml := c.PostForm("diggerconfig")
		if diggerConfigYaml == "" {
			services.AddWarning(c, "Digger config can't be empty")

			pageContext := services.GetMessages(c)
			maps.Copy(pageContext, gin.H{
				"Repo": repo,
			})
			c.HTML(http.StatusOK, "repo_add.tmpl", pageContext)
			return
		}

		messages, err := models.DB.UpdateRepoDiggerConfig(orgId, diggerConfigYaml, repo)
		if err != nil {
			if strings.HasPrefix(err.Error(), "validation error, ") {
				services.AddError(c, errors.Unwrap(err).Error())

				pageContext := services.GetMessages(c)
				maps.Copy(pageContext, gin.H{
					"Repo": repo,
				})
				c.HTML(http.StatusOK, "repo_add.tmpl", pageContext)
				return
			}
			log.Printf("failed to updated repo %v, %v", repoId, err)
			services.AddError(c, "failed to update repo")

			pageContext := services.GetMessages(c)
			maps.Copy(pageContext, gin.H{
				"Repo": repo,
			})
			c.HTML(http.StatusOK, "repo_add.tmpl", pageContext)
			return
		}
		for _, m := range messages {
			services.AddMessage(c, m)
		}
		c.Redirect(http.StatusFound, "/repos")
	}
}

func (web *WebController) Checkout(c *gin.Context) {
	stripe.Key = os.Getenv("STRIPE_KEY")

	params := &stripe.CheckoutSessionParams{
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			&stripe.CheckoutSessionLineItemParams{
				Price:    stripe.String(os.Getenv("STRIPE_PRICE_ID")),
				Quantity: stripe.Int64(1),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			TrialPeriodDays: stripe.Int64(14),
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String("https://" + c.Request.Host + "/projects"),
		CancelURL:  stripe.String("https://login.digger.dev"), //TODO use different login pages in different envs
	}

	s, err := session.New(params)

	if err != nil {
		log.Printf("session.New: %v", err)
	}

	c.Redirect(http.StatusSeeOther, s.URL)

}
