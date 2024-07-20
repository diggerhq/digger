package middleware

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/next/models"
	"github.com/diggerhq/digger/next/supa"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
)

func SupabaseCookieAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := supa.GetClient()
		if err != nil {
			log.Printf("could not create client")
			return
		}
		supbaseProjectId := os.Getenv("DIGGER_SUPABASE_PROJECT_REF")
		authTokenCookie, err := c.Cookie(fmt.Sprintf("sb-%v-auth-token", supbaseProjectId))
		var authTokenCookieItems []string
		err = json.Unmarshal([]byte(authTokenCookie), &authTokenCookieItems)
		if err != nil {
			log.Printf("could not find supabase auth cookie: %v", err)
			c.AbortWithStatus(http.StatusForbidden)
		}
		if len(authTokenCookieItems) < 1 {
			log.Printf("could not find supabase auth cookie token: %v", err)
			c.AbortWithStatus(http.StatusForbidden)
		}
		authToken := authTokenCookieItems[0]
		authenticatedClient := client.Auth.WithToken(authToken)
		user, err := authenticatedClient.GetUser()
		if err != nil {
			log.Printf("err: %v", err)
		}
		userId := user.ID.String()

		// TODO: We will have an additional cookie represnting the orgId of the user, and we will just query
		// for membership to verify
		var orgsForUser []models.PublicOrganizationMembersSelect
		_, err = client.From("organization_members").Select("*", "exact", false).Eq("member_id", userId).ExecuteTo(&orgsForUser)
		if err != nil {
			log.Printf("could not get org members: %v", err)
		}
		log.Printf("The found orgs for this user: %v", orgsForUser)

		if len(orgsForUser) == 0 {
			log.Printf("could not find any orgs for user: %v", userId)
			c.AbortWithStatus(http.StatusInternalServerError)
		}

		selectedOrg := orgsForUser[0]
		c.Set(ORGANISATION_ID_KEY, selectedOrg.Id)
		c.Next()
	}
}
