package controllers

import (
	"github.com/gin-gonic/gin"
)

func (mc MainController) Ping(c *gin.Context) {
	c.String(200, "pong")
}
