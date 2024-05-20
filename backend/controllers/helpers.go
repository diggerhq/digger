package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strings"
)

func RedirectToLoginOrProjects(context *gin.Context) {
	host := context.Request.Host
	if os.Getenv("REDIRECT_TO_LOGIN") == "true" {
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
