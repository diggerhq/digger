package main

import (
	"embed"
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/backend/utils"
	controllers "github.com/diggerhq/digger/next/controllers"
	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed templates
var templates embed.FS

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Initialized the logger successfully")
}

var Version = "dev"

func main() {

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:           os.Getenv("SENTRY_DSN"),
		EnableTracing: true,
		// Set TracesSampleRate to 1.0 to capture 100%
		// of transactions for performance monitoring.
		// We recommend adjusting this value in production,
		TracesSampleRate: 0.1,
		Release:          "api@" + Version,
		Debug:            true,
	}); err != nil {
		log.Printf("Sentry initialization failed: %v\n", err)
	}

	r := gin.Default()

	if _, err := os.Stat("templates"); err != nil {
		matches, _ := fs.Glob(templates, "templates/*.tmpl")
		for _, match := range matches {
			r.LoadHTMLFiles(match)
		}
		r.StaticFS("/static", http.FS(templates))
	} else {
		r.Static("/static", "./templates/static")
		r.LoadHTMLGlob("templates/*.tmpl")
	}

	diggerController := controllers.DiggerController{
		CiBackendProvider:    ci_backends.DefaultBackendProvider{},
		GithubClientProvider: utils.DiggerGithubRealClientProvider{},
	}

	r.GET("/", controllers.Home)

	r.GET("/github/callback", diggerController.GithubAppCallbackPage)
	port := config.GetPort()
	r.Run(fmt.Sprintf(":%d", port))

}
