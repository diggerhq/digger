package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "time"

	"github.com/diggerhq/digger/opentaco/internal/api"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
    var (
        port        = flag.String("port", "8080", "Server port")
        storageType = flag.String("storage", "s3", "Storage type: s3 or memory (default: s3 with fallback to memory)")
        s3Bucket    = flag.String("s3-bucket", os.Getenv("OPENTACO_S3_BUCKET"), "S3 bucket for state storage")
        s3Prefix    = flag.String("s3-prefix", os.Getenv("OPENTACO_S3_PREFIX"), "S3 key prefix (optional)")
        s3Region    = flag.String("s3-region", os.Getenv("OPENTACO_S3_REGION"), "S3 region (optional; uses AWS defaults if empty)")
    )
    flag.Parse()

    // Initialize storage
    var store storage.StateStore
    switch *storageType {
    case "s3":
        if *s3Bucket == "" {
            log.Printf("WARNING: S3 storage selected but bucket not provided. Falling back to in-memory storage.")
            store = storage.NewMemStore()
            log.Printf("Using in-memory storage")
            break
        }
        s, err := storage.NewS3Store(context.Background(), *s3Bucket, *s3Prefix, *s3Region)
        if err != nil {
            log.Printf("WARNING: failed to initialize S3 store: %v. Falling back to in-memory storage.", err)
            store = storage.NewMemStore()
            log.Printf("Using in-memory storage")
        } else {
            store = s
            log.Printf("Using S3 storage: bucket=%s prefix=%s region=%s", *s3Bucket, *s3Prefix, *s3Region)
        }
    default:
        store = storage.NewMemStore()
        log.Printf("Using in-memory storage")
    }

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.Gzip())
	e.Use(middleware.Secure())
	e.Use(middleware.CORS())

	// Initialize API
	api.RegisterRoutes(e, store)

	// Start server
	go func() {
		addr := fmt.Sprintf(":%s", *port)
		log.Printf("Starting OpenTaco service on %s", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := e.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}
	
	log.Println("Server shutdown complete")
}
