package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/backend/middleware"
	"github.com/gin-gonic/gin"
)

func GithubAppConnections(c *gin.Context) {
	orgId := c.GetUint(middleware.ORGANISATION_ID_KEY)
	if orgId == 0 {
		log.Printf("could not get orgId")
		c.String(500, "could not get orgId")
		return
	}

	type githubWebhook struct {
		URL    string `json:"url"`
		Active bool   `json:"active"`
	}

	type githubAppRequest struct {
		Description           string            `json:"description"`
		Events                []string          `json:"default_events"`
		Name                  string            `json:"name"`
		Permissions           map[string]string `json:"default_permissions"`
		Public                bool              `json:"public"`
		RedirectURL           string            `json:"redirect_url"`
		CallbackUrls          []string          `json:"callback_urls"`
		RequestOauthOnInstall bool              `json:"request_oauth_on_install"`
		SetupOnUpdate         bool              `json:"setup_on_update"`
		URL                   string            `json:"url"`
		Webhook               *githubWebhook    `json:"hook_attributes"`
	}

	host := os.Getenv("HOSTNAME")
	manifest := &githubAppRequest{
		Name:        fmt.Sprintf("Digger app %v", rand.Int31()),
		Description: fmt.Sprintf("Digger hosted at %s", host),
		URL:         host,
		RedirectURL: fmt.Sprintf("%s/github/connections/confirm", host),
		Public:      false,
		Webhook: &githubWebhook{
			Active: true,
			URL:    fmt.Sprintf("%s/github-app-webhook", host),
		},
		CallbackUrls:          []string{fmt.Sprintf("%s/github/callback", host)},
		SetupOnUpdate:         true,
		RequestOauthOnInstall: true,
		Events: []string{
			"check_run",
			"create",
			"delete",
			"issue_comment",
			"issues",
			"status",
			"pull_request_review_thread",
			"pull_request_review_comment",
			"pull_request_review",
			"pull_request",
			"push",
		},
		Permissions: map[string]string{
			"actions":          "write",
			"contents":         "write",
			"issues":           "write",
			"pull_requests":    "write",
			"repository_hooks": "write",
			"statuses":         "write",
			"administration":   "read",
			"checks":           "write",
			"members":          "read",
			"workflows":        "write",
		},
	}

	githubHostname := utils.GetGithubHostname()
	url := &url.URL{
		Scheme: "https",
		Host:   githubHostname,
		Path:   "/settings/apps/new",
	}

	// https://developer.github.com/apps/building-github-apps/creating-github-apps-using-url-parameters/#about-github-app-url-parameters
	githubOrg := os.Getenv("GITHUB_ORG")
	if githubOrg != "" {
		url.Path = fmt.Sprintf("organizations/%s%s", githubOrg, url.Path)
	}

	jsonManifest, err := json.MarshalIndent(manifest, "", " ")
	if err != nil {
		c.Error(fmt.Errorf("failed to serialize manifest %s", err))
		return
	}

	var connections []models.VCSConnection

	// GORM query
	result := models.DB.GormDB.Where("organisation_id = ?", orgId).Find(&connections)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "github_connections.tmpl", gin.H{"Target": url.String(), "connections": connections, "Manifest": string(jsonManifest)})
}

// GithubAppConnectionsConfirm handles the user coming back from creating their app
// A code query parameter is exchanged for this app's ID, key, and webhook_secret
// Implements https://developer.github.com/apps/building-github-apps/creating-github-apps-from-a-manifest/#implementing-the-github-app-manifest-flow
func (d DiggerEEController) GithubAppConnectionsConfirm(c *gin.Context) {
	orgId := c.GetUint(middleware.ORGANISATION_ID_KEY)
	if orgId == 0 {
		log.Printf("could not get orgId")
		c.String(500, "could not get orgId")
		return
	}

	code := c.Query("code")
	if code == "" {
		c.Error(fmt.Errorf("Ignoring callback, missing code query parameter"))
	}

	client, err := d.GithubClientProvider.NewClient(nil)
	if err != nil {
		c.Error(fmt.Errorf("could not create github client: %v", err))
	}
	cfg, _, err := client.Apps.CompleteAppManifest(context.Background(), code)
	if err != nil {
		c.Error(fmt.Errorf("Failed to exchange code for github app: %s", err))
		return
	}
	log.Printf("Found credentials for GitHub app %v with id %d", *cfg.Name, cfg.GetID())

	PEM := cfg.GetPEM()
	PemBase64 := base64.StdEncoding.EncodeToString([]byte(PEM))

	encrypt := func(val string) (string, error) {

		secret := os.Getenv("DIGGER_ENCRYPTION_SECRET")
		if secret == "" {
			log.Printf("ERROR: no encryption secret specified, please specify DIGGER_ENCRYPTION_SECRET as 32 bytes base64 string")
			return "", fmt.Errorf("no encryption secret specified, please specify DIGGER_ENCRYPTION_SECRET as 32 bytes base64 string")
		}

		// Encrypt
		encrypted, err := utils.AESEncrypt([]byte(secret), val)
		if err != nil {
			log.Printf("error while encrypting: %v", err)
			return "", fmt.Errorf("error while encrypting: %v", err)
		}
		return encrypted, nil
	}

	clientSecretEnc, err := encrypt(*cfg.ClientSecret)
	if err != nil {
		log.Printf("error while encrypting clientSecret: %v", err)
		c.String(500, "error while encrypting clientSecret")
		return
	}

	PEMEnc, err := encrypt(PEM)
	if err != nil {
		log.Printf("error while encrypting PEM: %v", err)
		c.String(500, "error while encrypting PEM")
		return
	}

	PEM64Enc, err := encrypt(PemBase64)
	if err != nil {
		log.Printf("error while encrypting PEM: %v", err)
		c.String(500, "error while encrypting PEM")
		return
	}

	webhookSecretEnc, err := encrypt(cfg.GetWebhookSecret())
	if err != nil {
		log.Printf("error while encrypting webhookSecret: %v", err)
		c.String(500, "error while encrypting webhookSecret")
		return
	}

	_, err = models.DB.CreateVCSConnection(cfg.GetName(), models.DiggerVCSGithub, cfg.GetID(), cfg.GetClientID(), clientSecretEnc, webhookSecretEnc, PEMEnc, PEM64Enc, *cfg.Owner.Login, cfg.GetHTMLURL(), "", "", "", "", orgId)
	if err != nil {
		log.Printf("failed to create github app connection record: %v", err)
		c.String(500, fmt.Sprintf("Failed to create github app record on callback"))
		return
	}

	c.Redirect(301, "/github/connections?success=true")
}

func (d DiggerEEController) GithubAppConnectionsDelete(c *gin.Context) {
	orgId := c.GetUint(middleware.ORGANISATION_ID_KEY)
	if orgId == 0 {
		log.Printf("could not get orgId")
		c.String(500, "could not get orgId")
		return
	}

	connectionId := c.Param("connection_id")
	if connectionId == "" {
		c.Error(fmt.Errorf("Ignoring callback, missing code query parameter"))
	}

	connection := models.VCSConnection{}
	result := models.DB.GormDB.Where("id = ?", connectionId).Delete(&connection)
	if result.Error != nil {
		log.Printf("error while deleting record %v: %v", connectionId, result.Error.Error())
		c.String(500, "error while deleting record")
		return
	}

	c.Redirect(301, "/github/connections?success=true")
}
