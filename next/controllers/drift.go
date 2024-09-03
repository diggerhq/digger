package controllers

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

type TriggerDriftRequest struct {
	ProjectId string `json:"project_id"`
}

func (d DiggerController) TriggerDriftDetectionForProject(c *gin.Context) {

	var request TriggerDriftRequest

	err := c.BindJSON(&request)

	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload received"})
		return
	}
	projectId := request.ProjectId

	c.JSON(200, gin.H{
		"status":     "successful",
		"project_id": projectId,
	})
	return

}
