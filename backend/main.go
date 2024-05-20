package main

import (
	"embed"
	"fmt"
	"github.com/diggerhq/digger/backend/bootstrap"
	"github.com/diggerhq/digger/backend/config"
)

//go:embed templates
var templates embed.FS

func main() {
	r := bootstrap.Bootstrap(templates)
	port := config.GetPort()
	r.Run(fmt.Sprintf(":%d", port))
}
