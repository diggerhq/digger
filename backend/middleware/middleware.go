package middleware

import (
	"github.com/diggerhq/digger/backend/services"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
)

func GetWebMiddleware() gin.HandlerFunc {
	if _, ok := os.LookupEnv("JWT_AUTH"); ok {
		log.Printf("Using JWT middleware for web routes")
		auth := services.Auth{
			HttpClient: http.Client{},
			Host:       os.Getenv("AUTH_HOST"),
			Secret:     os.Getenv("AUTH_SECRET"),
			ClientId:   os.Getenv("FRONTEGG_CLIENT_ID"),
		}
		return JWTWebAuth(auth)
	} else if _, ok := os.LookupEnv("HTTP_BASIC_AUTH"); ok {
		log.Printf("Using http basic auth middleware for web routes")
		return HttpBasicWebAuth()
	} else if _, ok := os.LookupEnv("NOOP_AUTH"); ok {
		log.Printf("Using noop auth for web routes")
		return NoopWebAuth()
	} else {
		log.Fatalf("Please specify one of JWT_AUTH or HTTP_BASIC_AUTH")
		return nil
	}
}

func GetApiMiddleware() gin.HandlerFunc {
	if _, ok := os.LookupEnv("JWT_AUTH"); ok {
		log.Printf("Using JWT middleware for API routes")
		auth := services.Auth{
			HttpClient: http.Client{},
			Host:       os.Getenv("AUTH_HOST"),
			Secret:     os.Getenv("AUTH_SECRET"),
			ClientId:   os.Getenv("FRONTEGG_CLIENT_ID"),
		}
		return JWTBearerTokenAuth(auth)
	} else if _, ok := os.LookupEnv("HTTP_BASIC_AUTH"); ok {
		log.Printf("Using http basic auth middleware for API routes")
		return HttpBasicApiAuth()
	} else if _, ok := os.LookupEnv("NOOP_AUTH"); ok {
		return NoopApiAuth()
	} else {
		log.Fatalf("Please specify one of JWT_AUTH or HTTP_BASIC_AUTH")
		return nil
	}
}
