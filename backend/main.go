package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"golang.org/x/sync/semaphore"
)

type FileService struct {
	redis        *redis.Client
	db           *Database
	compressor   *CompressionManager
	config       *Config
	chunkManager *ChunkUploadManager
	uploadSem    *semaphore.Weighted
	downloadSem  *semaphore.Weighted
}

func main() {
	// Load configuration
	config := LoadConfig()

	// Initialize Redis with optimized settings
	redisClient := redis.NewClient(&redis.Options{
		Addr:         config.RedisAddr,
		Password:     config.RedisPassword,
		DB:           config.RedisDB,
		PoolSize:     config.RedisPoolSize,
		MinIdleConns: config.RedisMaxIdleConns,
		MaxRetries:   3,
		ReadTimeout:  30 * time.Second, // Reduced for better concurrency
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  config.RedisIdleTimeout,
		PoolTimeout:  5 * time.Second, // Timeout when getting connection from pool
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	// Initialize PostgreSQL database
	database, err := NewDatabase(config)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer database.Close()

	// Check if schema exists and run migrations if needed
	schemaExists, err := database.CheckSchemaExists()
	if err != nil {
		log.Fatal("Failed to check schema existence:", err)
	}

	if !schemaExists {
		log.Printf("Database schema not found, running migrations...")
		if err := database.RunMigrations(); err != nil {
			log.Fatal("Failed to run database migrations:", err)
		}
	} else {
		log.Printf("Database schema already exists")
	}

	// Initialize services
	compressor := NewCompressionManager()
	chunkManager := NewChunkUploadManager(redisClient, config)

	service := &FileService{
		redis:        redisClient,
		db:           database,
		compressor:   compressor,
		config:       config,
		chunkManager: chunkManager,
		uploadSem:    semaphore.NewWeighted(int64(config.MaxConcurrentUploads)),
		downloadSem:  semaphore.NewWeighted(100), // 100 concurrent downloads
	}

	// Start expired file cleanup goroutines
	go service.startExpiredFileCleanup()
	go service.startDatabaseCleanup()

	// Setup Gin router with optimizations
	gin.SetMode(gin.DebugMode)

	router := gin.New()

	// Middleware for performance and security
	router.Use(gin.Recovery())
	router.Use(requestLoggingMiddleware())
	router.Use(corsMiddleware())
	router.Use(securityMiddleware())
	router.Use(rateLimitMiddleware(config))
	router.Use(http2PushMiddleware())

	// Add request timeout middleware
	router.Use(timeoutMiddleware(config.RequestTimeout))

	// Middleware to make fileService available in handlers
	router.Use(func(c *gin.Context) {
		c.Set("fileService", service)
		c.Next()
	})

	// API routes MUST come before static file routes
	api := router.Group("/api")
	{
		api.POST("/upload", service.uploadFile)
		api.GET("/file/:id", service.getFile)
		api.DELETE("/file/:id", service.deleteFile)
		api.GET("/metadata/:id", service.getMetadata)
		api.GET("/preview/:id", service.previewFile)
		api.GET("/stream/:id", service.fastStreamFile) // Optimized streaming endpoint
		// ZIP file extraction endpoint with query parameter
		api.GET("/zip/:id/extract", service.extractZipFile)
		api.GET("/zip/:id", service.browseZip)

		// Chunk upload endpoints
		api.POST("/chunk/initiate", service.chunkManager.InitiateUpload)
		api.POST("/chunk/:upload_id/:chunk_index", service.chunkManager.UploadChunk)
		api.POST("/chunk/:upload_id/complete", service.chunkManager.CompleteUpload)
		api.GET("/chunk/:upload_id/status", service.chunkManager.GetUploadStatus)
		api.GET("/file/:id/status", service.getFileStatus)

		// Admin endpoints
		api.POST("/admin/auth", service.adminAuth)
		api.PUT("/admin/file/:id/expires", service.updateFileExpiration)
		api.DELETE("/admin/file/:id", service.adminDeleteFile)
		api.POST("/admin/files", service.getAdminFileList)
	}

	// Serve static files (React build) - AFTER API routes
	router.Static("/assets", "./static/assets")
	router.StaticFile("/favicon.ico", "./static/favicon.ico")
	router.StaticFile("/logo.svg", "./static/logo.svg")
	router.StaticFile("/ogp.png", "./static/ogp.png")

	// SPA routes - serve React app for any non-API route
	router.NoRoute(func(c *gin.Context) {
		// Don't serve SPA for API routes that don't exist
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			return
		}
		// Serve index.html for SPA routes
		c.File("./static/index.html")
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	log.Printf("Server starting on %s:%s", config.Host, config.Port)
	log.Printf("Max file size: %d MB", config.MaxFileSize/(1024*1024))
	log.Printf("File retention: 24 hours")

	// Print all registered routes for debugging
	routes := router.Routes()
	log.Printf("Total routes registered: %d", len(routes))
	for _, route := range routes {
		log.Printf("Route: %s %s -> %s", route.Method, route.Path, route.Handler)
	}

	server := &http.Server{
		Addr:           config.Host + ":" + config.Port,
		Handler:        router,
		ReadTimeout:    0,  // No read timeout for streaming support
		WriteTimeout:   0,  // No write timeout for streaming support
		IdleTimeout:    120 * time.Second, // Close idle connections after 2 minutes
		MaxHeaderBytes: 1 << 20,           // 1MB max header size
	}

	log.Fatal(server.ListenAndServe())
}

func generateFileID() string {
	return uuid.New().String()
}

func (s *FileService) startExpiredFileCleanup() {
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupExpiredFiles()
	}
}

func (s *FileService) startDatabaseCleanup() {
	ticker := time.NewTicker(1 * time.Hour) // Check every hour
	defer ticker.Stop()

	for range ticker.C {
		if err := s.db.CleanupExpiredData(); err != nil {
			log.Printf("Error during database cleanup: %v", err)
		}
	}
}

func (s *FileService) cleanupExpiredFiles() {
	ctx := context.Background()
	now := time.Now()

	// Get all files that have expired
	expiredFiles, err := s.redis.ZRangeByScore(ctx, "files", &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%d", now.Unix()),
	}).Result()

	if err != nil {
		log.Printf("Error getting expired files: %v", err)
		return
	}

	for _, fileID := range expiredFiles {
		// Remove file content and metadata
		pipe := s.redis.Pipeline()
		pipe.Del(ctx, "file:"+fileID)
		pipe.Del(ctx, "content:"+fileID)
		pipe.ZRem(ctx, "files", fileID)

		if _, err := pipe.Exec(ctx); err != nil {
			log.Printf("Error deleting expired file %s: %v", fileID, err)
		} else {
			log.Printf("Deleted expired file: %s", fileID)
		}
	}
}
