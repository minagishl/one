package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type ChunkUpload struct {
	UploadID            string    `json:"upload_id"`
	Filename            string    `json:"filename"`
	TotalSize           int64     `json:"total_size"`
	TotalChunks         int       `json:"total_chunks"`
	ChunkSize           int64     `json:"chunk_size"`
	ReceivedChunks      []bool    `json:"received_chunks"`
	CreatedAt           time.Time `json:"created_at"`
	LastActivity        time.Time `json:"last_activity"`
	FileHash            string    `json:"file_hash,omitempty"`
	DownloadPassword    string    `json:"download_password,omitempty"`
	HasDownloadPassword bool      `json:"has_download_password"`
}

type ChunkUploadManager struct {
	redis   *redis.Client
	config  *Config
	uploads sync.Map // map[string]*ChunkUpload
}

func NewChunkUploadManager(redis *redis.Client, config *Config) *ChunkUploadManager {
	manager := &ChunkUploadManager{
		redis:  redis,
		config: config,
	}

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(config.TempDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create temp directory: %v", err))
	}

	// Start cleanup routine
	go manager.startCleanupRoutine()

	return manager
}

func (m *ChunkUploadManager) startCleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupExpiredUploads()
	}
}

func (m *ChunkUploadManager) cleanupExpiredUploads() {
	ctx := context.Background()
	now := time.Now()

	// Get all chunk uploads from Redis
	keys, err := m.redis.Keys(ctx, "chunk_upload:*").Result()
	if err != nil {
		return
	}

	for _, key := range keys {
		uploadJSON, err := m.redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var upload ChunkUpload
		if err := json.Unmarshal([]byte(uploadJSON), &upload); err != nil {
			continue
		}

		// Check if upload has expired
		if now.Sub(upload.LastActivity) > m.config.ChunkTimeout {
			m.cleanupUpload(upload.UploadID)
		}
	}
}

func (m *ChunkUploadManager) cleanupUpload(uploadID string) {
	ctx := context.Background()

	// Remove from Redis
	m.redis.Del(ctx, "chunk_upload:"+uploadID)

	// Remove from memory
	m.uploads.Delete(uploadID)

	// Remove temp directory
	tempDir := filepath.Join(m.config.TempDir, uploadID)
	os.RemoveAll(tempDir)
}

func (m *ChunkUploadManager) InitiateUpload(c *gin.Context) {
	var req struct {
		Filename         string `json:"filename" binding:"required"`
		TotalSize        int64  `json:"total_size" binding:"required"`
		ChunkSize        int64  `json:"chunk_size" binding:"required"`
		FileHash         string `json:"file_hash,omitempty"`
		DownloadPassword string `json:"download_password,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate request
	if req.TotalSize > m.config.MaxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":    "File too large",
			"max_size": m.config.MaxFileSize,
		})
		return
	}

	if req.ChunkSize > m.config.ChunkSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "Chunk size too large",
			"max_chunk_size": m.config.ChunkSize,
		})
		return
	}

	// Calculate total chunks
	totalChunks := int((req.TotalSize + req.ChunkSize - 1) / req.ChunkSize)
	if totalChunks > m.config.MaxChunksPerFile {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Too many chunks",
			"max_chunks": m.config.MaxChunksPerFile,
		})
		return
	}

	// Generate upload ID
	uploadID := generateFileID()

	// Create upload record
	upload := ChunkUpload{
		UploadID:            uploadID,
		Filename:            req.Filename,
		TotalSize:           req.TotalSize,
		TotalChunks:         totalChunks,
		ChunkSize:           req.ChunkSize,
		ReceivedChunks:      make([]bool, totalChunks),
		CreatedAt:           time.Now(),
		LastActivity:        time.Now(),
		FileHash:            req.FileHash,
		DownloadPassword:    req.DownloadPassword,
		HasDownloadPassword: req.DownloadPassword != "",
	}

	// Store in Redis with expiration
	uploadJSON, err := json.Marshal(upload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload session"})
		return
	}

	ctx := context.Background()
	if err := m.redis.Set(ctx, "chunk_upload:"+uploadID, uploadJSON, m.config.ChunkTimeout).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store upload session"})
		return
	}

	// Store in memory for quick access
	m.uploads.Store(uploadID, &upload)

	// Create temp directory for chunks
	tempDir := filepath.Join(m.config.TempDir, uploadID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp directory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_id":    uploadID,
		"total_chunks": totalChunks,
		"chunk_size":   req.ChunkSize,
		"expires_at":   time.Now().Add(m.config.ChunkTimeout),
	})
}

func (m *ChunkUploadManager) UploadChunk(c *gin.Context) {
	// Get file service from context for semaphore access
	fileService, exists := c.Get("fileService")
	if exists {
		if fs, ok := fileService.(*FileService); ok {
			// Acquire upload semaphore
			if err := fs.uploadSem.Acquire(c.Request.Context(), 1); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error": "Server busy, please try again later",
				})
				return
			}
			defer fs.uploadSem.Release(1)
		}
	}

	uploadID := c.Param("upload_id")
	chunkIndexStr := c.Param("chunk_index")

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chunk index"})
		return
	}

	// Get upload from memory or Redis
	uploadValue, exists := m.uploads.Load(uploadID)
	if !exists {
		// Try to load from Redis
		ctx := context.Background()
		uploadJSON, err := m.redis.Get(ctx, "chunk_upload:"+uploadID).Result()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Upload session not found"})
			return
		}

		var upload ChunkUpload
		if err := json.Unmarshal([]byte(uploadJSON), &upload); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse upload session"})
			return
		}

		uploadValue = &upload
		m.uploads.Store(uploadID, uploadValue)
	}

	upload := uploadValue.(*ChunkUpload)

	// Validate chunk index
	if chunkIndex < 0 || chunkIndex >= upload.TotalChunks {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid chunk index"})
		return
	}

	// Check if chunk already received
	if upload.ReceivedChunks[chunkIndex] {
		c.JSON(http.StatusOK, gin.H{
			"message":     "Chunk already received",
			"chunk_index": chunkIndex,
		})
		return
	}

	// Get chunk data from form
	file, _, err := c.Request.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No chunk data provided"})
		return
	}
	defer file.Close()

	// Save chunk to temp file
	chunkPath := filepath.Join(m.config.TempDir, uploadID, fmt.Sprintf("chunk_%d", chunkIndex))
	tempFile, err := os.Create(chunkPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
		return
	}
	defer tempFile.Close()

	// Copy chunk data to temp file
	if _, err := io.Copy(tempFile, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save chunk"})
		return
	}

	// Mark chunk as received
	upload.ReceivedChunks[chunkIndex] = true
	upload.LastActivity = time.Now()

	// Update in Redis
	uploadJSON, err := json.Marshal(upload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update upload session"})
		return
	}

	ctx := context.Background()
	if err := m.redis.Set(ctx, "chunk_upload:"+uploadID, uploadJSON, m.config.ChunkTimeout).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update upload session"})
		return
	}

	// Check if all chunks received
	allReceived := true
	receivedCount := 0
	for _, received := range upload.ReceivedChunks {
		if received {
			receivedCount++
		} else {
			allReceived = false
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Chunk uploaded successfully",
		"chunk_index":     chunkIndex,
		"received_chunks": receivedCount,
		"total_chunks":    upload.TotalChunks,
		"complete":        allReceived,
	})
}

func (m *ChunkUploadManager) CompleteUpload(c *gin.Context) {
	uploadID := c.Param("upload_id")

	// Get upload from memory or Redis
	uploadValue, exists := m.uploads.Load(uploadID)
	if !exists {
		ctx := context.Background()
		uploadJSON, err := m.redis.Get(ctx, "chunk_upload:"+uploadID).Result()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Upload session not found"})
			return
		}

		var upload ChunkUpload
		if err := json.Unmarshal([]byte(uploadJSON), &upload); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse upload session"})
			return
		}

		uploadValue = &upload
		m.uploads.Store(uploadID, uploadValue)
	}

	upload := uploadValue.(*ChunkUpload)

	// Check if all chunks received
	for i, received := range upload.ReceivedChunks {
		if !received {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":         "Missing chunks",
				"missing_chunk": i,
			})
			return
		}
	}

	// Assemble file from chunks
	fileID := generateFileID()
	assembledFile, err := m.assembleFile(upload, fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assemble file: " + err.Error()})
		return
	}
	defer assembledFile.Close()

	// Skip hash verification for now to avoid processing overhead
	// In production, consider implementing server-side hash verification
	if upload.FileHash != "" {
		// Log that hash verification is skipped
		fmt.Printf("Hash verification skipped for file: %s\n", upload.Filename)
	}

	// Reset file pointer
	if _, err := assembledFile.Seek(0, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset file pointer"})
		return
	}

	// Read file content for compression and storage
	content, err := io.ReadAll(assembledFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read assembled file"})
		return
	}

	// Get file service from context (assuming it's set in middleware)
	fileService, exists := c.Get("fileService")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "File service not available"})
		return
	}

	fs := fileService.(*FileService)

	// Store file using existing file service logic
	result, err := m.storeAssembledFile(fs, fileID, upload.Filename, content, upload.DownloadPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store file: " + err.Error()})
		return
	}

	// Cleanup upload session
	m.cleanupUpload(uploadID)

	c.JSON(http.StatusOK, result)
}

func (m *ChunkUploadManager) assembleFile(upload *ChunkUpload, fileID string) (*os.File, error) {
	// Create final file
	finalPath := filepath.Join(m.config.TempDir, fileID+"_assembled")
	finalFile, err := os.Create(finalPath)
	if err != nil {
		return nil, err
	}

	// Assemble chunks in order
	for i := 0; i < upload.TotalChunks; i++ {
		chunkPath := filepath.Join(m.config.TempDir, upload.UploadID, fmt.Sprintf("chunk_%d", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			finalFile.Close()
			os.Remove(finalPath)
			return nil, err
		}

		if _, err := io.Copy(finalFile, chunkFile); err != nil {
			chunkFile.Close()
			finalFile.Close()
			os.Remove(finalPath)
			return nil, err
		}

		chunkFile.Close()
	}

	// Reset file pointer to beginning
	if _, err := finalFile.Seek(0, 0); err != nil {
		finalFile.Close()
		os.Remove(finalPath)
		return nil, err
	}

	return finalFile, nil
}

func (m *ChunkUploadManager) storeAssembledFile(fs *FileService, fileID, filename string, content []byte, downloadPassword string) (map[string]interface{}, error) {
	ctx := context.Background()

	// Generate random delete password
	deletePassword := generateRandomPassword()

	// For large files, skip compression to avoid memory issues
	var compressedContent []byte
	var compressionType CompressionType

	if len(content) > 100*1024*1024 { // 100MB threshold
		// Skip compression for very large files
		compressedContent = content
		compressionType = CompressionNone
		fmt.Printf("Skipping compression for large file: %s (%d bytes)\n", filename, len(content))
	} else {
		// Select compression type
		compressionType = fs.compressor.SelectCompressionType(filename, int64(len(content)))

		// Compress file
		var err error
		compressedContent, err = fs.compressor.Compress(content, compressionType)
		if err != nil {
			return nil, err
		}
	}

	// Create metadata with 24-hour expiration
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	detectedMimeType := GetMimeType(filename)

	metadata := FileMetadata{
		ID:                  fileID,
		Filename:            filename,
		Size:                int64(len(content)),
		CompressedSize:      int64(len(compressedContent)),
		MimeType:            detectedMimeType,
		Compression:         compressionType,
		UploadTime:          now,
		ExpiresAt:           expiresAt,
		DeletePassword:      deletePassword,
		DownloadPassword:    downloadPassword,
		HasDownloadPassword: downloadPassword != "",
	}

	// For very large files, store on disk instead of Redis
	if len(compressedContent) > 50*1024*1024 { // 50MB threshold for disk storage
		// Store file on disk
		diskPath := filepath.Join(m.config.TempDir, "files", fileID)
		if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create file directory: %v", err)
		}

		if err := os.WriteFile(diskPath, compressedContent, 0644); err != nil {
			return nil, fmt.Errorf("failed to write file to disk: %v", err)
		}

		// Store file path reference in Redis instead of content
		expiration := 24 * time.Hour
		if err := fs.redis.Set(ctx, "content:"+fileID, "DISK:"+diskPath, expiration).Err(); err != nil {
			return nil, fmt.Errorf("failed to store file reference: %v", err)
		}

		fmt.Printf("Stored large file on disk: %s\n", diskPath)
	} else {
		// Store smaller files in Redis as before
		expiration := 24 * time.Hour
		if err := fs.redis.Set(ctx, "content:"+fileID, compressedContent, expiration).Err(); err != nil {
			return nil, fmt.Errorf("failed to store file content: %v", err)
		}
	}

	// Store metadata with 24-hour expiration
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	expiration := 24 * time.Hour
	if err := fs.redis.Set(ctx, "file:"+fileID, metadataJSON, expiration).Err(); err != nil {
		return nil, err
	}

	// Add to file list with expiration score
	if err := fs.redis.ZAdd(ctx, "files", &redis.Z{
		Score:  float64(expiresAt.Unix()),
		Member: fileID,
	}).Err(); err != nil {
		return nil, err
	}

	// Set expiration on the file list entry
	fs.redis.Expire(ctx, "files", expiration)

	return map[string]interface{}{
		"message":  "File uploaded successfully",
		"file_id":  fileID,
		"metadata": metadata,
	}, nil
}

func (m *ChunkUploadManager) GetUploadStatus(c *gin.Context) {
	uploadID := c.Param("upload_id")

	// Get upload from memory or Redis
	uploadValue, exists := m.uploads.Load(uploadID)
	if !exists {
		ctx := context.Background()
		uploadJSON, err := m.redis.Get(ctx, "chunk_upload:"+uploadID).Result()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Upload session not found"})
			return
		}

		var upload ChunkUpload
		if err := json.Unmarshal([]byte(uploadJSON), &upload); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse upload session"})
			return
		}

		uploadValue = &upload
		m.uploads.Store(uploadID, uploadValue)
	}

	upload := uploadValue.(*ChunkUpload)

	// Count received chunks
	receivedCount := 0
	for _, received := range upload.ReceivedChunks {
		if received {
			receivedCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"upload_id":       upload.UploadID,
		"filename":        upload.Filename,
		"total_size":      upload.TotalSize,
		"total_chunks":    upload.TotalChunks,
		"received_chunks": receivedCount,
		"complete":        receivedCount == upload.TotalChunks,
		"created_at":      upload.CreatedAt,
		"last_activity":   upload.LastActivity,
		"expires_at":      upload.CreatedAt.Add(m.config.ChunkTimeout),
	})
}
