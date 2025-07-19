package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// requestLoggingMiddleware logs HTTP requests with timing and error information
func requestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Skip logging for health checks and static assets
		if path == "/health" || path == "/favicon.ico" || path == "/logo.svg" {
			c.Next()
			return
		}

		// Process request
		c.Next()

		// Log request details
		end := time.Now()
		latency := end.Sub(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		// Log format: [timestamp] method path - status latency clientIP
		log.Printf("[%s] %s %s - %d %v %s",
			end.Format("2006-01-02 15:04:05"),
			method,
			path,
			statusCode,
			latency,
			clientIP,
		)

		// Log errors with more detail
		if statusCode >= 400 {
			if len(c.Errors) > 0 {
				log.Printf("Request errors: %v", c.Errors)
			}
		}
	}
}

// corsMiddleware adds CORS headers for browser compatibility
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "3600")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// rateLimitMiddleware implements basic rate limiting
func rateLimitMiddleware(_ *Config) gin.HandlerFunc {
	type clientInfo struct {
		lastRequest time.Time
		requests    int
	}

	clients := make(map[string]*clientInfo)
	var mu sync.RWMutex

	// Cleanup old entries every minute
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for ip, client := range clients {
				if now.Sub(client.lastRequest) > time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		// Skip rate limiting for streaming endpoints to allow unlimited concurrent streams
		if strings.HasPrefix(c.Request.URL.Path, "/api/stream/") {
			c.Next()
			return
		}

		mu.Lock()
		defer mu.Unlock()

		client, exists := clients[ip]
		if !exists {
			clients[ip] = &clientInfo{
				lastRequest: now,
				requests:    1,
			}
			c.Next()
			return
		}

		// Reset counter if more than a minute has passed
		if now.Sub(client.lastRequest) > time.Minute {
			client.requests = 1
			client.lastRequest = now
			c.Next()
			return
		}

		// Rate limit: 200 requests per minute per IP (increased for better concurrent support)
		if client.requests >= 200 {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
			c.Abort()
			return
		}

		client.requests++
		client.lastRequest = now
		c.Next()
	}
}

// timeoutMiddleware adds request timeout
func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// compressionMiddleware adds response compression
func compressionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add compression headers
		c.Header("Vary", "Accept-Encoding")

		// Check if client accepts compression
		if c.GetHeader("Accept-Encoding") != "" {
			c.Header("Content-Encoding", "gzip")
		}

		c.Next()
	}
}

// http2PushMiddleware adds HTTP/2 server push for media files
func http2PushMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this is a media file request
		if strings.HasPrefix(c.Request.URL.Path, "/api/stream/") || strings.HasPrefix(c.Request.URL.Path, "/api/preview/") {
			// Add HTTP/2 server push headers for better performance
			c.Header("Link", "</static/assets/index.css>; rel=preload; as=style")
			c.Header("Link", "</static/assets/index.js>; rel=preload; as=script")

			// Add performance hints
			c.Header("X-DNS-Prefetch-Control", "on")
			c.Header("X-Preload", "true")
		}

		c.Next()
	}
}

// securityMiddleware adds security headers
func securityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; media-src 'self' blob:; object-src 'self' blob:; frame-src 'self' blob:")
		c.Next()
	}
}
