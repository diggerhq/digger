package middleware

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
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
			log.Printf("could not create client: %v", err)
			c.String(http.StatusBadRequest, "error checking auth")
			c.Abort()
			return
		}
		supbaseProjectId := os.Getenv("DIGGER_SUPABASE_PROJECT_REF")
		authTokenCookie, err := c.Cookie(fmt.Sprintf("sb-%v-auth-token", supbaseProjectId))
		var authTokenCookieItems []string
		err = json.Unmarshal([]byte(authTokenCookie), &authTokenCookieItems)
		if err != nil {
			log.Printf("could not find supabase auth cookie: %v", err)
			c.String(http.StatusBadRequest, "error checking cookie")
			c.Abort()
			return
		}
		if len(authTokenCookieItems) == 0 {
			log.Printf("could not find supabase auth cookie token: %v", err)
			c.String(http.StatusBadRequest, "error checking cookie")
			c.Abort()
			return
		}
		authToken := authTokenCookieItems[0]
		authenticatedClient := client.Auth.WithToken(authToken)
		user, err := authenticatedClient.GetUser()
		if err != nil {
			log.Printf("err: %v", err)
		}
		userId := user.ID.String()

		// TODO: We will have an additional cookie representing the orgId of the user, and we will just query
		// for membership to verify
		var orgsForUser []model.OrganizationMember

		_, err = client.From("organization_members").Select("*", "exact", false).Eq("member_id", userId).ExecuteTo(&orgsForUser)
		if err != nil {
			log.Printf("could not get org members: %v", err)
		}

		if len(orgsForUser) == 0 {
			log.Printf("could not find any orgs for user: %v", userId)
			c.String(http.StatusBadRequest, "User does not belong to any orgs")
			c.Abort()
			return
		}

		selectedOrg, err := dbmodels.DB.GetUserOrganizationsFirstMatch(userId)
		if err != nil {
			log.Printf("error while finding organisation: %v", err)
			c.String(http.StatusBadRequest, "User does not belong to any orgs")
			c.Abort()
			return
		}
		c.Set(ORGANISATION_ID_KEY, selectedOrg.ID)
		c.Next()
	}
}
