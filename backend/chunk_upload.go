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
	"golang.org/x/sys/unix"
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

type ProcessingJob struct {
	JobID     string      `json:"job_id"`
	UploadID  string      `json:"upload_id"`
	FileID    string      `json:"file_id"`
	Status    string      `json:"status"`   // pending, processing, completed, failed
	Progress  int         `json:"progress"` // 0-100
	Error     string      `json:"error,omitempty"`
	Result    *FileResult `json:"result,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type FileResult struct {
	FileID         string `json:"file_id"`
	Filename       string `json:"filename"`
	URL            string `json:"url"`
	Size           int64  `json:"size"`
	DeletePassword string `json:"delete_password,omitempty"`
}

type ChunkUploadManager struct {
	redis   *redis.Client
	config  *Config
	uploads sync.Map // map[string]*ChunkUpload
	jobs    sync.Map // map[string]*ProcessingJob
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

	// First, check if disk space is low and do aggressive cleanup
	if err := m.checkDiskSpace(5 * 1024 * 1024 * 1024); err != nil { // 5GB threshold
		fmt.Printf("Low disk space detected, performing aggressive cleanup: %v\n", err)
		m.aggressiveCleanup()
	}

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

// aggressiveCleanup removes all temporary files when disk space is low
func (m *ChunkUploadManager) aggressiveCleanup() {
	tempDir := m.config.TempDir
	
	// Remove all assembled files older than 1 hour
	filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if info.IsDir() {
			return nil
		}
		
		// Remove old assembled files and orphaned chunks
		if info.ModTime().Before(time.Now().Add(-1 * time.Hour)) {
			fmt.Printf("Removing old temp file: %s\n", path)
			os.Remove(path)
		}
		
		return nil
	})
	
	// Force cleanup of all expired uploads
	m.uploads.Range(func(key, value interface{}) bool {
		upload := value.(*ChunkUpload)
		if time.Since(upload.LastActivity) > 10*time.Minute {
			m.cleanupUpload(upload.UploadID)
		}
		return true
	})
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

	// Create processing job for background processing
	fileID := generateFileID()
	jobID := generateFileID() // Reuse the same function for job ID

	job := &ProcessingJob{
		JobID:     jobID,
		UploadID:  uploadID,
		FileID:    fileID,
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store job in memory and Redis
	m.jobs.Store(jobID, job)
	ctx := context.Background()
	jobJSON, _ := json.Marshal(job)
	m.redis.Set(ctx, "processing_job:"+jobID, jobJSON, 24*time.Hour)

	// Get file service from context
	fileService, exists := c.Get("fileService")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "File service not available"})
		return
	}
	fs := fileService.(*FileService)

	// Store initial processing status in Redis for file status endpoint
	statusJSON, _ := json.Marshal(map[string]interface{}{
		"status": "processing",
		"filename": upload.Filename,
		"job_id": jobID,
	})
	fs.redis.Set(ctx, "processing:"+fileID, statusJSON, 1*time.Hour)

	// Start background processing
	go m.processFileInBackground(job, upload, fs)

	// Return job ID immediately for client polling
	c.JSON(http.StatusAccepted, gin.H{
		"job_id":  jobID,
		"file_id": fileID,
		"status":  "pending",
		"message": "File processing started. Use the file_id to check status at /api/file/{file_id}/status",
	})
}

func (m *ChunkUploadManager) processFileInBackground(job *ProcessingJob, upload *ChunkUpload, fs *FileService) {
	// Update job status to processing
	job.Status = "processing"
	job.Progress = 10
	job.UpdatedAt = time.Now()
	m.updateJob(job)

	// Assemble file from chunks with streaming approach
	assembledFile, err := m.assembleFileStreaming(upload, job.FileID)
	if err != nil {
		job.Status = "failed"
		job.Error = "Failed to assemble file: " + err.Error()
		job.UpdatedAt = time.Now()
		m.updateJob(job)
		return
	}
	defer assembledFile.Close()

	// Update progress
	job.Progress = 50
	job.UpdatedAt = time.Now()
	m.updateJob(job)

	// Get file info
	fileInfo, err := assembledFile.Stat()
	if err != nil {
		job.Status = "failed"
		job.Error = "Failed to get file info: " + err.Error()
		job.UpdatedAt = time.Now()
		m.updateJob(job)
		return
	}

	// Store file with streaming approach
	result, err := m.storeAssembledFileStreaming(fs, job.FileID, upload.Filename, assembledFile, upload.DownloadPassword)
	if err != nil {
		job.Status = "failed"
		job.Error = "Failed to store file: " + err.Error()
		job.UpdatedAt = time.Now()
		m.updateJob(job)
		return
	}

	// Update progress
	job.Progress = 90
	job.UpdatedAt = time.Now()
	m.updateJob(job)

	// Cleanup upload session
	m.cleanupUpload(upload.UploadID)

	// Complete job
	job.Status = "completed"
	job.Progress = 100
	
	// Extract metadata from result
	var deletePassword string
	if metadata, ok := result["metadata"].(FileMetadata); ok {
		deletePassword = metadata.DeletePassword
	}
	
	job.Result = &FileResult{
		FileID:         result["file_id"].(string),
		Filename:       upload.Filename,
		URL:            "/file/" + result["file_id"].(string),
		Size:           fileInfo.Size(),
		DeletePassword: deletePassword,
	}
	job.UpdatedAt = time.Now()
	m.updateJob(job)
	
	// Store processing status in Redis for file status endpoint
	ctx := context.Background()
	statusJSON, _ := json.Marshal(map[string]interface{}{
		"status": "completed",
		"file_id": job.FileID,
	})
	fs.redis.Set(ctx, "processing:"+job.FileID, statusJSON, 1*time.Hour)
}

func (m *ChunkUploadManager) updateJob(job *ProcessingJob) {
	m.jobs.Store(job.JobID, job)
	ctx := context.Background()
	jobJSON, _ := json.Marshal(job)
	m.redis.Set(ctx, "processing_job:"+job.JobID, jobJSON, 24*time.Hour)
}

func (m *ChunkUploadManager) GetJobStatus(c *gin.Context) {
	jobID := c.Param("job_id")

	// Get job from memory or Redis
	jobValue, exists := m.jobs.Load(jobID)
	if !exists {
		ctx := context.Background()
		jobJSON, err := m.redis.Get(ctx, "processing_job:"+jobID).Result()
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		var job ProcessingJob
		if err := json.Unmarshal([]byte(jobJSON), &job); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse job"})
			return
		}

		jobValue = &job
		m.jobs.Store(jobID, jobValue)
	}

	job := jobValue.(*ProcessingJob)
	c.JSON(http.StatusOK, job)
}

func (m *ChunkUploadManager) assembleFileStreaming(upload *ChunkUpload, fileID string) (*os.File, error) {
	// Check available disk space before assembly
	if err := m.checkDiskSpace(upload.TotalSize * 2); err != nil {
		return nil, fmt.Errorf("insufficient disk space: %v", err)
	}

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

// checkDiskSpace checks if there's enough available disk space
func (m *ChunkUploadManager) checkDiskSpace(requiredBytes int64) error {
	tempDir := m.config.TempDir
	
	// Get filesystem stats for temp directory
	var stat unix.Statfs_t
	if err := unix.Statfs(tempDir, &stat); err != nil {
		return fmt.Errorf("failed to get filesystem stats: %v", err)
	}
	
	// Calculate available space
	availableBytes := int64(stat.Bavail) * int64(stat.Bsize)
	
	// Require 1GB buffer + required bytes
	minRequired := requiredBytes + (1024 * 1024 * 1024)
	
	if availableBytes < minRequired {
		return fmt.Errorf("insufficient disk space: available %d bytes, required %d bytes", 
			availableBytes, minRequired)
	}
	
	return nil
}

func (m *ChunkUploadManager) storeAssembledFileStreaming(fs *FileService, fileID, filename string, file *os.File, downloadPassword string) (map[string]interface{}, error) {
	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	// Read file content for storage decision
	var content []byte

	// For very large files (>100MB), store directly on disk without compression
	if fileSize > 100*1024*1024 {
		// Store large file directly without loading into memory
		storagePath := filepath.Join(fs.config.TempDir, "files", fileID)

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(storagePath), 0755); err != nil {
			return nil, err
		}

		// Copy file to final location
		destFile, err := os.Create(storagePath)
		if err != nil {
			return nil, err
		}
		defer destFile.Close()

		// Reset file pointer
		if _, err := file.Seek(0, 0); err != nil {
			return nil, err
		}

		// Stream copy without loading into memory
		if _, err := io.Copy(destFile, file); err != nil {
			return nil, err
		}

		// Generate random delete password
		deletePassword := generateRandomPassword()
		
		// Create metadata for large file
		now := time.Now()
		expiresAt := now.Add(24 * time.Hour)
		detectedMimeType := GetMimeType(filename)
		
		metadata := FileMetadata{
			ID:                  fileID,
			Filename:            filename,
			Size:                fileSize,
			MimeType:            detectedMimeType,
			UploadTime:          now,
			ExpiresAt:           expiresAt,
			Compression:         CompressionNone,
			DeletePassword:      deletePassword,
			DownloadPassword:    downloadPassword,
			HasDownloadPassword: downloadPassword != "",
		}
		
		// Store file reference and metadata in Redis
		ctx := context.Background()
		expiration := 24 * time.Hour
		
		// Store file path reference
		if err := fs.redis.Set(ctx, "content:"+fileID, "DISK:"+storagePath, expiration).Err(); err != nil {
			return nil, fmt.Errorf("failed to store file reference: %v", err)
		}
		
		// Store metadata
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return nil, err
		}
		
		if err := fs.redis.Set(ctx, "file:"+fileID, metadataJSON, expiration).Err(); err != nil {
			return nil, err
		}
		
		// Add to file list
		if err := fs.redis.ZAdd(ctx, "files", &redis.Z{
			Score:  float64(expiresAt.Unix()),
			Member: fileID,
		}).Err(); err != nil {
			return nil, err
		}
		
		return map[string]interface{}{
			"message":  "File uploaded successfully",
			"file_id":  fileID,
			"metadata": metadata,
		}, nil
	}

	// For smaller files, use existing compression logic
	content, err = io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return m.storeAssembledFile(fs, fileID, filename, content, downloadPassword)
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
