package controllers

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

func Home(c *gin.Context) {

	client, err := supa.GetClient()
	if err != nil {
		log.Printf("could not create client")
		return
	}
	supbaseProjectId := os.Getenv("DIGGER_SUPABASE_PROJECT_REF")
	authToken, err := c.Cookie(fmt.Sprintf("sb-%v-auth-token", supbaseProjectId))
	var items []string
	json.Unmarshal([]byte(authToken), &items)
	log.Printf("%v", items)
	authenticatedClient := client.Auth.WithToken(items[0])
	user, err := authenticatedClient.GetUser()
	if err != nil {
		log.Printf("err: %v", err)
	}
	email := user.Email
	userId := user.ID.String()
	log.Printf("the email is: %v id: %v", email, userId)

	var orgMembers []models.PublicOrganizationMembersSelect
	_, err = client.From("organization_members").Select("*", "exact", false).Eq("member_id", userId).ExecuteTo(&orgMembers)
	if err != nil {
		log.Printf("could not get org members: %v", err)
	}
	log.Printf("The found orgs for this user: %v", orgMembers)

	c.HTML(http.StatusOK, "home.tmpl", gin.H{})
}
