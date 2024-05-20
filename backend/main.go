package main

import (
	"embed"
	"fmt"
	"github.com/diggerhq/digger/backend/bootstrap"
	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/controllers"
)

//go:embed templates
var templates embed.FS

func main() {
	r := bootstrap.Bootstrap(templates)
	r.GET("/", controllers.Home)
	port := config.GetPort()
	r.Run(fmt.Sprintf(":%d", port))
}
