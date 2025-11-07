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

	"github.com/diggerhq/digger/opentaco/internal/analytics"
	"github.com/diggerhq/digger/opentaco/internal/api"
	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/queryfactory"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/repositories"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/kelseyhightower/envconfig"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

// change this random number to bump version of statesman: 421
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
	err := envconfig.Process("opentaco", &queryCfg) // The prefix "TACO" will be used for all vars.
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

	log.Printf("Query backend initialized: %s", queryCfg.Backend)


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


	// sync units to query index 
	existingUnits, err := queryStore.ListUnits(context.Background(), "")
	if err != nil {
		log.Printf("Warning: Failed to check for existing units: %v", err)
	}
	
	if len(existingUnits) == 0 {
		log.Println("Query backend has no units, syncing from storage...")
		units, err := blobStore.List(context.Background(), "")
		if err != nil {
			log.Printf("Warning: Failed to list units from storage: %v", err)
		} else {
			for _, unit := range units {
				if err := queryStore.SyncEnsureUnit(context.Background(), unit.ID); err != nil {
					log.Printf("Warning: Failed to sync unit %s: %v", unit.ID, err)
					continue
				}
				
				if err := queryStore.SyncUnitMetadata(context.Background(), unit.ID, unit.Size, unit.Updated); err != nil {
					log.Printf("Warning: Failed to sync metadata for unit %s: %v", unit.ID, err)
				}
			}
			log.Printf("Synced %d units from storage to database", len(units))
		}
	} else {
		log.Printf("Query backend already has %d units, skipping sync", len(existingUnits))
	}

	// create repository
	// repository coordinates blob storage with query index internally
	// Get the underlying *gorm.DB from the query store
	db := repositories.GetDBFromQueryStore(queryStore)
	if db == nil {
		log.Fatalf("Query store does not provide GetDB method")
	}
	
	// Ensure default organization exists
	defaultOrgUUID, err := repositories.EnsureDefaultOrganization(context.Background(), db)
	if err != nil {
		log.Fatalf("Failed to ensure default organization: %v", err)
	}
	log.Printf("Default organization ensured: %s", defaultOrgUUID)
	
	repo := repositories.NewUnitRepository(db, blobStore)
	log.Println("Repository initialized (database-first with blob storage backend)")
	
	// Create RBAC Manager
	rbacManager, err := rbac.NewRBACManagerFromQueryStore(queryStore)
	if err != nil {
		log.Fatalf("Failed to create RBAC manager: %v", err)
	}
	
	// --- Create Domain Interfaces with Optional Authorization ---
	// These interfaces are what handlers will use
	var fullRepo domain.UnitRepository = repo
	
	// Wrap with authorization if auth is enabled
	if !*authDisable {
		log.Println("Authorization is ENABLED. Wrapping repository with RBAC.")
		
		// Create bootstrap context with default org for RBAC check
		// During startup, we need org context to check RBAC status
		bootstrapCtx := domain.ContextWithOrg(context.Background(), defaultOrgUUID)
		
		// Verify RBAC manager was created successfully (fail closed for security)
		canInit, err := rbacManager.IsEnabled(bootstrapCtx)
		if err != nil {
			log.Fatalf("Failed to verify RBAC manager: %v", err)
		}
		
		if !canInit {
			log.Println("RBAC is NOT initialized. System will operate in permissive mode until RBAC is initialized via /v1/rbac/init")
		}
		
		fullRepo = repositories.NewAuthorizingRepository(repo, rbacManager)
	} else {
		log.Println("Authorization is DISABLED via flag. All operations allowed.")
	}

	// Initialize analytics with system ID management (always create system ID)
	// Analytics uses the blob store for storage operations
	analytics.InitGlobalWithSystemID("production", blobStore)
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


	// Create a signer for JWTs (this may need to be configured from env vars)
	signer, err := auth.NewSignerFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize JWT signer: %v", err)
	}

	// Register routes with interface-based dependencies
	api.RegisterRoutes(e, api.Dependencies{
		Repository:          fullRepo,      // RBAC-wrapped repository (used by authenticated routes)
		UnwrappedRepository: repo,          // Unwrapped repository (for pre-authorized operations like signed URLs)
		BlobStore:           blobStore,     // Direct blob access (for legacy components)
		QueryStore:          queryStore,    // Direct query access
		RBACManager:         rbacManager,   // RBAC management
		Signer:              signer,        // JWT signing
		AuthEnabled:         !*authDisable, // Auth flag
	})

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

