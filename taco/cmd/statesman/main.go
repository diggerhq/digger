package main

// Clean release test after fixing component names
// Verifying Release-Please workflow with PAT token
// Expecting automatic tag creation on merge
// Testing binary cleanup to prevent dgctl contamination
// Added workflow exclusions to prevent release collisions
// Testing isolated taco releases without interference  
// Testing multi-arch Docker builds with linux/amd64,arm64,386 support
// Removed linux/386 due to Ubuntu 24.04 platform compatibility
// Fixed semver pattern issue - using manual version for Docker tags
import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/analytics"
	"github.com/diggerhq/digger/opentaco/internal/api"
	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/middleware"
	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/queryfactory"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/diggerhq/digger/opentaco/internal/wiring"
	"github.com/kelseyhightower/envconfig"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

func main() {
	var (
		port        = flag.String("port", "8080", "Server port")
		authDisable = flag.Bool("auth-disable", os.Getenv("OPENTACO_AUTH_DISABLE") == "true", "Disable auth enforcement (default: false)")
		storageType = flag.String("storage", "s3", "Storage type: s3 or memory (default: s3 with fallback to memory)")
		s3Bucket    = flag.String("s3-bucket", os.Getenv("OPENTACO_S3_BUCKET"), "S3 bucket for state storage")
		s3Prefix    = flag.String("s3-prefix", os.Getenv("OPENTACO_S3_PREFIX"), "S3 key prefix (optional)")
		s3Region    = flag.String("s3-region", os.Getenv("OPENTACO_S3_REGION"), "S3 region (optional; uses AWS defaults if empty)")
	)
	flag.Parse()

	// Load configuration from environment variables into our struct.
	var queryCfg query.Config
	err := envconfig.Process("taco", &queryCfg) // The prefix "TACO" will be used for all vars.
	if err != nil {
		log.Fatalf("Failed to process configuration: %v", err)
	}

	// --- Initialize Stores ---

	// Create the database index store using the dedicated factory.
	queryStore, err := queryfactory.NewQueryStore(queryCfg)
	if err != nil {
		log.Fatalf("Failed to initialize query backend: %v", err)
	}
	defer queryStore.Close()

	log.Printf("Query backend initialized: %s (enabled: %v)", queryCfg.Backend, queryStore.IsEnabled())

	if queryStore.IsEnabled(){
		log.Println("Query backend enabled successfully")
	}else{
		log.Println("Query backend disabled. You are in no-op mode.")
	}


	// Initialize storage
	var blobStore storage.UnitStore
	switch *storageType {
	case "s3":
		if *s3Bucket == "" {
			log.Printf("WARNING: S3 storage selected but bucket not provided. Falling back to in-memory storage.")
			blobStore = storage.NewMemStore()
			log.Printf("Using in-memory storage")
			break
		}
		s, err := storage.NewS3Store(context.Background(), *s3Bucket, *s3Prefix, *s3Region)
		if err != nil {
			log.Printf("WARNING: failed to initialize S3 store: %v. Falling back to in-memory storage.", err)
			blobStore = storage.NewMemStore()
			log.Printf("Using in-memory storage")
		} else {
			blobStore = s
			log.Printf("Using S3 storage: bucket=%s prefix=%s region=%s", *s3Bucket, *s3Prefix, *s3Region)

 		}
	default:
		blobStore = storage.NewMemStore()
		log.Printf("Using in-memory storage")
	}


	// 3. Create the base OrchestratingStore
	orchestratingStore := storage.NewOrchestratingStore(blobStore, queryStore)

	// --- Sync RBAC Data ---
	if queryStore.IsEnabled() {
		if err := wiring.SyncRBACFromStorage(context.Background(), blobStore, queryStore); err != nil {
			log.Printf("Warning: Failed to sync RBAC data: %v", err)
		}
		
		// Sync existing units from storage to database
		log.Println("Syncing existing units from storage to database...")
		units, err := blobStore.List(context.Background(), "")
		if err != nil {
			log.Printf("Warning: Failed to list units from storage: %v", err)
		} else {
			log.Printf("DEBUG: Got %d units from storage", len(units))
			for _, unit := range units {
				log.Printf("DEBUG: Unit from storage: ID=%s, Size=%d, Updated=%v", unit.ID, unit.Size, unit.Updated)
				
				// Always ensure unit exists first
				if err := queryStore.SyncEnsureUnit(context.Background(), unit.ID); err != nil {
					log.Printf("Warning: Failed to sync unit %s: %v", unit.ID, err)
					continue
				}
				
				// Always sync metadata to update existing records
				log.Printf("Syncing metadata for %s: size=%d, updated=%v", unit.ID, unit.Size, unit.Updated)
				if err := queryStore.SyncUnitMetadata(context.Background(), unit.ID, unit.Size, unit.Updated); err != nil {
					log.Printf("Warning: Failed to sync metadata for unit %s: %v", unit.ID, err)
				}
			}
			log.Printf("Synced %d units from storage to database", len(units))
		}
	}

	// --- Conditionally Apply Authorization Layer with a SMART CHECK ---
	var finalStore storage.UnitStore
	
	// Check if there are any RBAC roles defined in the database.
	rbacIsConfigured, err := queryStore.HasRBACRoles(context.Background())
	if err != nil {
		log.Fatalf("Failed to check for RBAC configuration: %v", err)
	}

	// The condition is now two-part: Auth must be enabled AND RBAC roles must exist.
	if !*authDisable && rbacIsConfigured {
		log.Println("RBAC is ENABLED and CONFIGURED. Wrapping store with authorization layer.")
		finalStore = storage.NewAuthorizingStore(orchestratingStore, queryStore)
	} else {
		if !*authDisable {
			log.Println("RBAC is ENABLED but NOT CONFIGURED (no roles found). Authorization layer will be skipped.")
		} else {
			log.Println("RBAC is DISABLED via flag. Authorization layer will be skipped.")
		}
		finalStore = orchestratingStore
	}

	// Initialize analytics with system ID management (always create system ID)
	analytics.InitGlobalWithSystemID("production", finalStore)
	
	// Initialize system ID synchronously during startup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Try to preload existing system ID and user email first
	if err := analytics.PreloadSystemID(ctx); err == nil {
		log.Printf("System ID and user email loaded from storage")
	} else {
		// If preload fails, create new system ID
		if systemID, err := analytics.GetOrCreateSystemID(ctx); err == nil {
			log.Printf("System ID created: %s", systemID)
		} else {
			log.Printf("System ID not available: %v", err)
		}
	}
	analytics.SendEssential("service_startup") 

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.RequestID())
	e.Use(echomiddleware.Gzip())
	e.Use(echomiddleware.Secure())
	e.Use(echomiddleware.CORS())


	signer, err := auth.NewSignerFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize JWT signer: %v", err)
	}

	// Conditionally apply the authentication middleware.
	if !*authDisable {
		e.Use(middleware.JWTAuthMiddleware(signer))
	}

	// Pass the same signer instance to routes
	api.RegisterRoutes(e, finalStore, !*authDisable, queryStore, blobStore, signer)

	// Start server
	go func() {
		addr := fmt.Sprintf(":%s", *port)
		log.Printf("Starting OpenTaco service on %s", addr)
		analytics.SendEssential("server_started")
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			analytics.SendEssential("server_startup_failed")
			log.Fatalf("Server startup failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	// Graceful shutdown
	analytics.SendEssential("server_shutdown_initiated")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		analytics.SendEssential("server_shutdown_failed")
		log.Fatalf("Server shutdown failed: %v", err)
	}

	analytics.SendEssential("server_shutdown_complete")
	log.Println("Server shutdown complete")
}

