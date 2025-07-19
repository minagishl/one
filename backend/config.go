package main

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server configuration
	Port string
	Host string

	// Redis configuration
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// File storage
	MaxFileSize       int64
	MaxFilesPerUser   int
	AllowedExtensions []string
	ChunkThreshold    int64 // Files larger than this will use chunked upload

	// Chunk upload settings
	ChunkSize        int64
	MaxChunksPerFile int
	TempDir          string
	ChunkTimeout     time.Duration

	// Compression
	CompressionLevel int
	EnableStreaming  bool

	// Performance
	MaxConcurrentUploads int
	RequestTimeout       time.Duration
	RedisPoolSize        int
	RedisMaxIdleConns    int
	RedisIdleTimeout     time.Duration
}

func LoadConfig() *Config {
	return &Config{
		Port: getEnv("PORT", "8080"),
		Host: getEnv("HOST", "0.0.0.0"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		MaxFileSize:       getEnvInt64("MAX_FILE_SIZE", 10*1024*1024*1024), // 10GB
		MaxFilesPerUser:   getEnvInt("MAX_FILES_PER_USER", 1000),
		AllowedExtensions: []string{},                                    // Empty means all extensions allowed
		ChunkThreshold:    getEnvInt64("CHUNK_THRESHOLD", 100*1024*1024), // 100MB threshold

		// Chunk upload settings
		ChunkSize:        getEnvInt64("CHUNK_SIZE", 50*1024*1024), // 50MB chunks (optimized for better progress tracking)
		MaxChunksPerFile: getEnvInt("MAX_CHUNKS_PER_FILE", 200),   // 200 chunks max (10GB total)
		TempDir:          getEnv("TEMP_DIR", "./temp"),
		ChunkTimeout:     getEnvDuration("CHUNK_TIMEOUT", "30m"), // Increased timeout for larger chunks

		CompressionLevel:     getEnvInt("COMPRESSION_LEVEL", 6),
		EnableStreaming:      getEnvBool("ENABLE_STREAMING", true),
		MaxConcurrentUploads: getEnvInt("MAX_CONCURRENT_UPLOADS", 50),
		RequestTimeout:       getEnvDuration("REQUEST_TIMEOUT", "15m"), // Increased for large file processing
		RedisPoolSize:        getEnvInt("REDIS_POOL_SIZE", 100),        // Increased for high concurrency
		RedisMaxIdleConns:    getEnvInt("REDIS_MAX_IDLE_CONNS", 20),
		RedisIdleTimeout:     getEnvDuration("REDIS_IDLE_TIMEOUT", "5m"),
	}
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue string) time.Duration {
	value := getEnv(key, defaultValue)
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	duration, _ := time.ParseDuration(defaultValue)
	return duration
}
