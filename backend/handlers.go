package main

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type FileMetadata struct {
	ID                  string          `json:"id"`
	Filename            string          `json:"filename"`
	Size                int64           `json:"size"`
	CompressedSize      int64           `json:"compressed_size"`
	MimeType            string          `json:"mime_type"`
	Compression         CompressionType `json:"compression"`
	UploadTime          time.Time       `json:"upload_time"`
	ExpiresAt           time.Time       `json:"expires_at"`
	DeletePassword      string          `json:"delete_password,omitempty"`
	DownloadPassword    string          `json:"download_password,omitempty"`
	HasDownloadPassword bool            `json:"has_download_password"`
}

// convertToUTF8 tries to convert string from various Japanese encodings to UTF-8
func convertToUTF8(input string) string {
	// First check if it's already valid UTF-8
	if utf8.ValidString(input) {
		return input
	}

	// Convert string to bytes for better encoding detection
	inputBytes := []byte(input)

	// Try to convert from Shift_JIS (most common for Windows ZIP files)
	decoder := japanese.ShiftJIS.NewDecoder()
	if result, _, err := transform.Bytes(decoder, inputBytes); err == nil {
		resultStr := string(result)
		if utf8.ValidString(resultStr) && containsJapanese(resultStr) {
			return resultStr
		}
	}

	// Try to convert from EUC-JP
	decoder = japanese.EUCJP.NewDecoder()
	if result, _, err := transform.Bytes(decoder, inputBytes); err == nil {
		resultStr := string(result)
		if utf8.ValidString(resultStr) && containsJapanese(resultStr) {
			return resultStr
		}
	}

	// Try to convert from ISO-2022-JP
	decoder = japanese.ISO2022JP.NewDecoder()
	if result, _, err := transform.Bytes(decoder, inputBytes); err == nil {
		resultStr := string(result)
		if utf8.ValidString(resultStr) && containsJapanese(resultStr) {
			return resultStr
		}
	}

	// If all conversions fail, return the original string
	return input
}

// getFileStatus returns processing status or direct access for files
func (s *FileService) getFileStatus(c *gin.Context) {
	fileID := c.Param("id")
	ctx := context.Background()

	// First check if there's a processing status for this file
	processingJSON, err := s.redis.Get(ctx, "processing:"+fileID).Result()
	if err == nil {
		var processingStatus map[string]interface{}
		if json.Unmarshal([]byte(processingJSON), &processingStatus) == nil {
			status, _ := processingStatus["status"].(string)
			if status == "processing" {
				filename, _ := processingStatus["filename"].(string)
				c.JSON(http.StatusAccepted, gin.H{
					"status": "processing",
					"message": "Your file is currently being processed. Please wait a moment and try again.",
					"filename": filename,
					"estimated_time": "A few moments",
				})
				return
			} else if status == "completed" {
				// File processing is completed, remove processing status and continue to check file availability
				s.redis.Del(ctx, "processing:"+fileID)
			} else if status == "failed" {
				// File processing failed, return detailed error information
				errorMsg := "File processing failed. Please try uploading again."
				if errorDetail, exists := processingStatus["error"].(string); exists {
					errorMsg = errorDetail
				}
				c.JSON(http.StatusBadRequest, gin.H{
					"status": "failed",
					"message": errorMsg,
					"error_type": "processing_failed",
				})
				return
			}
		}
	}

	// Get file metadata from PostgreSQL (primary source)
	fileStorage, dbErr := s.db.GetFileMetadata(fileID)
	var metadata FileMetadata
	var fileFound bool

	if dbErr != nil {
		log.Printf("Failed to get file metadata from database: %v", dbErr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"message": "Failed to check file status"})
		return
	}

	if fileStorage != nil {
		// File found in database
		fileFound = true
		metadata = FileMetadata{
			ID:                  fileStorage.ID,
			Filename:           fileStorage.Filename,
			Size:               fileStorage.OriginalSize,
			CompressedSize:     0,
			MimeType:           fileStorage.MimeType,
			Compression:        CompressionType(fileStorage.CompressionType),
			UploadTime:         fileStorage.UploadTime,
			ExpiresAt:          fileStorage.ExpiresAt,
			DeletePassword:     fileStorage.DeletePassword,
			DownloadPassword:   "",
			HasDownloadPassword: fileStorage.HasDownloadPassword,
		}
		
		if fileStorage.CompressedSize != nil {
			metadata.CompressedSize = *fileStorage.CompressedSize
		}
		
		if fileStorage.DownloadPassword != nil {
			metadata.DownloadPassword = *fileStorage.DownloadPassword
		}
	} else {
		// File not found in PostgreSQL
		c.JSON(http.StatusNotFound, gin.H{
			"status": "not_found",
			"message": "File not found or may have expired"})
		return
	}

	if !fileFound {
		c.JSON(http.StatusNotFound, gin.H{
			"status": "not_found",
			"message": "File not found or may still be processing"})
		return
	}

	// Check if file content is available
	var contentAvailable bool
	
	// If file is stored on disk, check file existence
	if fileStorage.StorageType == "disk" && fileStorage.StoragePath != nil {
		if _, err := os.Stat(*fileStorage.StoragePath); err == nil {
			contentAvailable = true
		}
	} else {
		// For PostgreSQL storage, content is always available if metadata exists
		contentAvailable = true
	}

	if contentAvailable {
		// File is ready, remove processing status
		s.redis.Del(ctx, "processing:"+fileID)
		
		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"message": "File is ready for download",
			"metadata": metadata,
			"download_url": "/api/file/" + fileID,
			"preview_url": "/api/preview/" + fileID,
		})
	} else {
		// File metadata exists but content is not ready (still processing)
		c.JSON(http.StatusAccepted, gin.H{
			"status": "processing",
			"message": "Your file is currently being processed. Please wait a moment and try again.",
			"filename": metadata.Filename,
			"estimated_time": "A few moments",
		})
	}
}

// containsJapanese checks if the string contains Japanese characters
func containsJapanese(s string) bool {
	for _, r := range s {
		// Check for Hiragana, Katakana, and Kanji ranges
		if (r >= 0x3040 && r <= 0x309F) || // Hiragana
			(r >= 0x30A0 && r <= 0x30FF) || // Katakana
			(r >= 0x4E00 && r <= 0x9FAF) { // Kanji
			return true
		}
	}
	return false
}

// detectAndConvertFilename attempts to convert filename from various encodings
func detectAndConvertFilename(name string) string {
	// If it's already valid UTF-8 and contains readable characters, return as-is
	if utf8.ValidString(name) && isReadableText(name) {
		return name
	}

	// Convert the filename string back to raw bytes
	// Go's ZIP reader reads filenames as latin-1, so we need to convert back to bytes
	rawBytes := make([]byte, len(name))
	for i, r := range []byte(name) {
		rawBytes[i] = r
	}

	// Try Shift_JIS conversion (most common for Japanese Windows ZIP files)
	decoder := japanese.ShiftJIS.NewDecoder()
	if converted, _, err := transform.Bytes(decoder, rawBytes); err == nil {
		result := string(converted)
		if utf8.ValidString(result) && containsJapanese(result) {
			return result
		}
	}

	// Try EUC-JP conversion
	decoder = japanese.EUCJP.NewDecoder()
	if converted, _, err := transform.Bytes(decoder, rawBytes); err == nil {
		result := string(converted)
		if utf8.ValidString(result) && containsJapanese(result) {
			return result
		}
	}

	// If conversion fails, try the original convertToUTF8 function
	return convertToUTF8(name)
}

// isReadableText checks if the string contains mostly readable characters
func isReadableText(s string) bool {
	if len(s) == 0 {
		return true
	}

	readableCount := 0
	for _, r := range s {
		// Count printable ASCII, Japanese characters, and common punctuation
		if (r >= 32 && r <= 126) || // ASCII printable
			(r >= 0x3040 && r <= 0x309F) || // Hiragana
			(r >= 0x30A0 && r <= 0x30FF) || // Katakana
			(r >= 0x4E00 && r <= 0x9FAF) || // Kanji
			r == '/' || r == '\\' || r == '.' || r == '-' || r == '_' {
			readableCount++
		}
	}

	// If more than 70% of characters are readable, consider it valid
	return float64(readableCount)/float64(len([]rune(s))) > 0.7
}

// generateRandomPassword generates a random password for file deletion
func generateRandomPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 12

	password := make([]byte, length)
	for i := range password {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		password[i] = charset[num.Int64()]
	}
	return string(password)
}

func (s *FileService) uploadFile(c *gin.Context) {
	// Acquire upload semaphore
	if err := s.uploadSem.Acquire(c.Request.Context(), 1); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Server busy, please try again later",
		})
		return
	}
	defer s.uploadSem.Release(1)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// Check if file exceeds chunk threshold
	if header.Size > s.config.ChunkThreshold {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "File too large for standard upload",
			"message": "Files larger than 100MB must use chunked upload",
			"max_size": s.config.ChunkThreshold,
			"use_chunked": true,
		})
		return
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	// Generate unique file ID
	fileID := generateFileID()
	ctx := context.Background()

	// Get optional download password from form
	downloadPassword := c.PostForm("download_password")
	hasDownloadPassword := downloadPassword != ""

	// Generate random delete password
	deletePassword := generateRandomPassword()

	// Select compression type
	compressionType := s.compressor.SelectCompressionType(header.Filename, header.Size)

	// Compress file
	compressedContent, err := s.compressor.Compress(content, compressionType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to compress file"})
		return
	}

	// Create metadata with 24-hour expiration
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	detectedMimeType := GetMimeType(header.Filename)
	log.Printf("uploadFile: filename=%s, detected MIME type=%s", header.Filename, detectedMimeType)

	metadata := FileMetadata{
		ID:                  fileID,
		Filename:            header.Filename,
		Size:                header.Size,
		CompressedSize:      int64(len(compressedContent)),
		MimeType:            detectedMimeType,
		Compression:         compressionType,
		UploadTime:          now,
		ExpiresAt:           expiresAt,
		DeletePassword:      deletePassword,
		DownloadPassword:    downloadPassword,
		HasDownloadPassword: hasDownloadPassword,
	}

	// Determine storage strategy based on file size
	var storageType string
	var storagePath *string
	var fileContent []byte
	
	// For very large files (>1GB), store on disk; otherwise store in PostgreSQL
	if header.Size > 1024*1024*1024 { // 1GB threshold
		storageType = "disk"
		// Create storage directory
		filesDir := filepath.Join(s.config.TempDir, "files")
		if err := os.MkdirAll(filesDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create storage directory"})
			return
		}
		
		// Save to disk
		diskPath := filepath.Join(filesDir, fileID)
		if err := os.WriteFile(diskPath, compressedContent, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file to disk"})
			return
		}
		storagePath = &diskPath
		fileContent = nil // Don't store content in database for disk files
	} else {
		storageType = "postgresql"
		storagePath = nil
		fileContent = compressedContent
	}

	// Store file metadata and content in PostgreSQL
	fileStorage := &FileStorage{
		ID:                  fileID,
		Filename:           header.Filename,
		OriginalSize:       header.Size,
		CompressedSize:     &metadata.CompressedSize,
		MimeType:           detectedMimeType,
		CompressionType:    string(compressionType),
		StorageType:        storageType,
		StoragePath:        storagePath,
		FileContent:        fileContent,
		UploadTime:         now,
		ExpiresAt:          expiresAt,
		DeletePassword:     deletePassword,
		DownloadPassword:   nil,
		HasDownloadPassword: hasDownloadPassword,
	}

	if hasDownloadPassword {
		fileStorage.DownloadPassword = &downloadPassword
	}

	if err := s.db.SaveFile(fileStorage); err != nil {
		// If database save fails, clean up disk file if it was created
		if storageType == "disk" && storagePath != nil {
			os.Remove(*storagePath)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Cache metadata in Redis for faster access (optional)
	metadataJSON, err := json.Marshal(metadata)
	if err == nil {
		expiration := 24 * time.Hour
		s.redis.Set(ctx, "file:"+fileID, metadataJSON, expiration)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully",
		"file_id":  fileID,
		"metadata": metadata,
	})
}

func (s *FileService) getFile(c *gin.Context) {
	// Acquire download semaphore
	if err := s.downloadSem.Acquire(c.Request.Context(), 1); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Server busy, please try again later",
		})
		return
	}
	defer s.downloadSem.Release(1)

	fileID := c.Param("id")

	// Get file from PostgreSQL (primary source)
	fileStorage, err := s.db.GetFile(fileID)
	if err != nil {
		log.Printf("Failed to get file from database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	if fileStorage == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Convert database record to metadata
	metadata := FileMetadata{
		ID:                  fileStorage.ID,
		Filename:           fileStorage.Filename,
		Size:               fileStorage.OriginalSize,
		CompressedSize:     0,
		MimeType:           fileStorage.MimeType,
		Compression:        CompressionType(fileStorage.CompressionType),
		UploadTime:         fileStorage.UploadTime,
		ExpiresAt:          fileStorage.ExpiresAt,
		DeletePassword:     fileStorage.DeletePassword,
		DownloadPassword:   "",
		HasDownloadPassword: fileStorage.HasDownloadPassword,
	}
	
	if fileStorage.CompressedSize != nil {
		metadata.CompressedSize = *fileStorage.CompressedSize
	}
	
	if fileStorage.DownloadPassword != nil {
		metadata.DownloadPassword = *fileStorage.DownloadPassword
	}

	// Check if file has expired
	if metadata.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File has expired"})
		return
	}

	// Check download password if required (bypass for admin)
	if metadata.HasDownloadPassword {
		providedPassword := c.Query("password")
		adminToken := c.Query("admin_token")
		
		isAdminAccess := false
		if adminToken != "" {
			if _, err := s.validateAdminToken(adminToken); err == nil {
				isAdminAccess = true
				log.Printf("Admin access granted for file %s", fileID)
			}
		}
		
		if !isAdminAccess && providedPassword != metadata.DownloadPassword {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Password required",
				"message": "This file is password protected. Please provide the correct password.",
			})
			return
		}
	}

	// Get file content based on storage type
	var content []byte
	if fileStorage.StorageType == "disk" && fileStorage.StoragePath != nil {
		// Read from disk
		diskContent, err := os.ReadFile(*fileStorage.StoragePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file from disk"})
			return
		}

		// Decompress file
		content, err = s.compressor.Decompress(diskContent, metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	} else {
		// Read from PostgreSQL
		if fileStorage.FileContent == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "File content not found"})
			return
		}

		// Decompress file
		content, err = s.compressor.Decompress(fileStorage.FileContent, metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	}

	// Set appropriate headers
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", metadata.Filename))
	c.Header("Content-Type", metadata.MimeType)
	c.Header("Content-Length", strconv.FormatInt(metadata.Size, 10))

	c.Data(http.StatusOK, metadata.MimeType, content)
}

func (s *FileService) deleteFile(c *gin.Context) {
	fileID := c.Param("id")
	ctx := context.Background()

	// Get file metadata from PostgreSQL
	fileStorage, err := s.db.GetFileMetadata(fileID)
	if err != nil {
		log.Printf("Failed to get file metadata: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	if fileStorage == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Check delete password (bypass for admin)
	providedPassword := c.Query("delete_password")
	adminToken := c.Query("admin_token")
	
	isAdminAccess := false
	if adminToken != "" {
		if _, err := s.validateAdminToken(adminToken); err == nil {
			isAdminAccess = true
			log.Printf("Admin access granted for file deletion %s", fileID)
		}
	}
	
	if !isAdminAccess && providedPassword != fileStorage.DeletePassword {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid delete password",
			"message": "The provided delete password is incorrect.",
		})
		return
	}

	// Delete from PostgreSQL
	if err := s.db.DeleteFile(fileID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file from database"})
		return
	}

	// Delete disk file if it exists
	if fileStorage.StorageType == "disk" && fileStorage.StoragePath != nil {
		if err := os.Remove(*fileStorage.StoragePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to delete file from disk: %v", err)
		}
	}

	// Remove from Redis cache (optional)
	s.redis.Del(ctx, "file:"+fileID)

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

func (s *FileService) previewFile(c *gin.Context) {
	// Acquire download semaphore for preview
	if err := s.downloadSem.Acquire(c.Request.Context(), 1); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Server busy, please try again later",
		})
		return
	}
	defer s.downloadSem.Release(1)

	fileID := c.Param("id")

	// Get file from PostgreSQL (primary source)
	fileStorage, err := s.db.GetFile(fileID)
	if err != nil {
		log.Printf("Failed to get file from database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	if fileStorage == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Convert database record to metadata
	metadata := FileMetadata{
		ID:                  fileStorage.ID,
		Filename:           fileStorage.Filename,
		Size:               fileStorage.OriginalSize,
		CompressedSize:     0,
		MimeType:           fileStorage.MimeType,
		Compression:        CompressionType(fileStorage.CompressionType),
		UploadTime:         fileStorage.UploadTime,
		ExpiresAt:          fileStorage.ExpiresAt,
		DeletePassword:     fileStorage.DeletePassword,
		DownloadPassword:   "",
		HasDownloadPassword: fileStorage.HasDownloadPassword,
	}
	
	if fileStorage.CompressedSize != nil {
		metadata.CompressedSize = *fileStorage.CompressedSize
	}
	
	if fileStorage.DownloadPassword != nil {
		metadata.DownloadPassword = *fileStorage.DownloadPassword
	}

	// Check if file has expired
	if metadata.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File has expired"})
		return
	}

	// Check download password if required (bypass for admin)
	if metadata.HasDownloadPassword {
		providedPassword := c.Query("password")
		adminToken := c.Query("admin_token")
		
		isAdminAccess := false
		if adminToken != "" {
			if _, err := s.validateAdminToken(adminToken); err == nil {
				isAdminAccess = true
				log.Printf("Admin access granted for file %s", fileID)
			}
		}
		
		if !isAdminAccess && providedPassword != metadata.DownloadPassword {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Password required",
				"message": "This file is password protected. Please provide the correct password.",
			})
			return
		}
	}

	// Check if file type is previewable
	log.Printf("previewFile: checking if %s (MIME: %s) is previewable", metadata.Filename, metadata.MimeType)
	if !isPreviewable(metadata.MimeType) {
		log.Printf("previewFile: file type %s not previewable", metadata.MimeType)
		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"error":            "File type not previewable",
			"message":          "This file type cannot be previewed in the browser. Please download the file to view it.",
			"mime_type":        metadata.MimeType,
			"suggested_action": "download",
		})
		return
	}

	// Set appropriate headers for preview
	c.Header("Content-Type", metadata.MimeType)
	c.Header("Content-Length", strconv.FormatInt(metadata.Size, 10))
	c.Header("Accept-Ranges", "bytes")

	// Handle range requests for large files
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		s.handleRangeRequestFromDB(c, fileStorage, metadata, rangeHeader)
		return
	}

	// For media files, redirect to optimized streaming endpoint
	if isMediaFile(metadata.MimeType) && metadata.Size > 5*1024*1024 { // 5MB threshold for media
		// Add cache headers for media files
		c.Header("Cache-Control", "public, max-age=3600")
		c.Header("ETag", fmt.Sprintf("\"%s\"", fileID))
		
		// Check for conditional requests
		if match := c.GetHeader("If-None-Match"); match != "" {
			if strings.Trim(match, "\"") == fileID {
				c.Status(http.StatusNotModified)
				return
			}
		}
		
		s.streamContentFromDB(c, fileStorage, metadata)
		return
	}
	
	// For large images, also add cache headers
	if isImageFile(metadata.MimeType) && metadata.Size > 1*1024*1024 { // 1MB threshold for images
		c.Header("Cache-Control", "public, max-age=3600")
		c.Header("ETag", fmt.Sprintf("\"%s\"", fileID))
		
		// Check for conditional requests
		if match := c.GetHeader("If-None-Match"); match != "" {
			if strings.Trim(match, "\"") == fileID {
				c.Status(http.StatusNotModified)
				return
			}
		}
	}

	// For large files, use streaming
	if metadata.Size > 10*1024*1024 { // 10MB threshold
		s.streamContentFromDB(c, fileStorage, metadata)
		return
	}

	// Small files - get content based on storage type
	var content []byte
	if fileStorage.StorageType == "disk" && fileStorage.StoragePath != nil {
		// Read from disk
		diskContent, err := os.ReadFile(*fileStorage.StoragePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file from disk"})
			return
		}

		// Decompress file
		content, err = s.compressor.Decompress(diskContent, metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	} else {
		// Read from PostgreSQL
		if fileStorage.FileContent == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "File content not found"})
			return
		}

		// Decompress file
		content, err = s.compressor.Decompress(fileStorage.FileContent, metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	}

	c.Data(http.StatusOK, metadata.MimeType, content)
}

// handleRangeRequestFromDB handles range requests for files stored in database
func (s *FileService) handleRangeRequestFromDB(c *gin.Context, fileStorage *FileStorage, metadata FileMetadata, rangeHeader string) {
	// Parse range header
	ranges, err := parseRangeHeader(rangeHeader, metadata.Size)
	if err != nil {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", metadata.Size))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	if len(ranges) != 1 {
		// Multi-range not supported
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", metadata.Size))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	rangeSpec := ranges[0]
	contentLength := rangeSpec.end - rangeSpec.start + 1

	// Set headers for partial content
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeSpec.start, rangeSpec.end, metadata.Size))
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Content-Type", metadata.MimeType)
	c.Header("Cache-Control", "public, max-age=3600")
	c.Status(http.StatusPartialContent)

	// Get file content and stream the requested range
	if fileStorage.StorageType == "disk" && fileStorage.StoragePath != nil {
		s.streamRangeFromDisk(c, *fileStorage.StoragePath, metadata, rangeSpec)
	} else {
		// For PostgreSQL storage, decompress and stream range
		if fileStorage.FileContent == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File content not found"})
			return
		}

		content, err := s.compressor.Decompress(fileStorage.FileContent, metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}

		// Validate range
		if rangeSpec.start >= int64(len(content)) || rangeSpec.end >= int64(len(content)) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid range"})
			return
		}

		// Stream the requested range
		rangeContent := content[rangeSpec.start : rangeSpec.end+1]
		if _, err := c.Writer.Write(rangeContent); err != nil {
			log.Printf("Error writing range response: %v", err)
		}
	}
}

// streamContentFromDB streams file content from database storage
func (s *FileService) streamContentFromDB(c *gin.Context, fileStorage *FileStorage, metadata FileMetadata) {
	if fileStorage.StorageType == "disk" && fileStorage.StoragePath != nil {
		// Stream from disk
		s.streamFromDisk(c, *fileStorage.StoragePath, metadata)
	} else {
		// Stream from PostgreSQL
		if fileStorage.FileContent == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File content not found"})
			return
		}

		// Decompress content
		content, err := s.compressor.Decompress(fileStorage.FileContent, metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}

		// Set response headers
		c.Writer.Header().Set("Content-Type", metadata.MimeType)
		c.Writer.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))
		c.Writer.WriteHeader(http.StatusOK)

		// Stream with buffer for better performance
		reader := bytes.NewReader(content)
		buffer := make([]byte, 1024*1024) // 1MB buffer
		_, err = io.CopyBuffer(c.Writer, reader, buffer)
		if err != nil {
			log.Printf("Error streaming file: %v", err)
		}
	}
}

// fastStreamFile provides optimized streaming for large media files
func (s *FileService) fastStreamFile(c *gin.Context) {
	// Note: No semaphore acquisition for streaming to allow unlimited concurrent streams
	// Streaming is bandwidth-limited rather than CPU/memory intensive

	fileID := c.Param("id")
	log.Printf("fastStreamFile called for fileID: %s", fileID)
	ctx := context.Background()

	// Get metadata
	metadataJSON, err := s.redis.Get(ctx, "file:"+fileID).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	var metadata FileMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse metadata"})
		return
	}

	// Check if file has expired
	if metadata.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File has expired"})
		return
	}

	// Check download password if required
	if metadata.HasDownloadPassword {
		providedPassword := c.Query("password")
		adminToken := c.Query("admin_token")
		
		isAdminAccess := false
		if adminToken != "" {
			if _, err := s.validateAdminToken(adminToken); err == nil {
				isAdminAccess = true
				log.Printf("Admin access granted for file %s", fileID)
			}
		}
		
		if !isAdminAccess && providedPassword != metadata.DownloadPassword {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Password required",
				"message": "This file is password protected. Please provide the correct password.",
			})
			return
		}
	}

	// Get file from PostgreSQL for streaming
	fileStorageForStream, err := s.db.GetFile(fileID)
	if err != nil {
		log.Printf("Failed to get file for streaming: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	if fileStorageForStream == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Set optimized headers for media streaming
	c.Header("Content-Type", metadata.MimeType)
	c.Header("Content-Length", strconv.FormatInt(metadata.Size, 10))
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("ETag", fmt.Sprintf("\"%s\"", fileID))

	// Check for conditional requests (If-None-Match)
	if match := c.GetHeader("If-None-Match"); match != "" {
		if strings.Trim(match, "\"") == fileID {
			c.Status(http.StatusNotModified)
			return
		}
	}

	// Handle range requests for media files
	rangeHeader := c.GetHeader("Range")
	if rangeHeader != "" {
		s.handleRangeRequestFromDB(c, fileStorageForStream, metadata, rangeHeader)
		return
	}

	// For large media files, use optimized streaming
	s.streamContentFromDB(c, fileStorageForStream, metadata)
}

// streamMediaContent provides optimized streaming for media files
func (s *FileService) streamMediaContent(c *gin.Context, compressedContent string, metadata FileMetadata) {
	log.Printf("streamMediaContent: content prefix check, length=%d", len(compressedContent))
	if strings.HasPrefix(compressedContent, "DISK:") {
		// Stream directly from disk for best performance
		diskPath := strings.TrimPrefix(compressedContent, "DISK:")
		log.Printf("Streaming from disk: %s", diskPath)
		s.streamMediaFromDisk(c, diskPath, metadata)
	} else {
		// Stream from Redis
		log.Printf("Streaming from Redis, content length: %d bytes", len(compressedContent))
		s.streamMediaFromRedis(c, compressedContent, metadata)
	}
}

// streamMediaFromDisk optimized disk streaming for media files
func (s *FileService) streamMediaFromDisk(c *gin.Context, diskPath string, metadata FileMetadata) {
	// Open file directly for uncompressed files (media files are typically uncompressed)
	if metadata.Compression == CompressionNone {
		log.Printf("Attempting to open file at path: %s", diskPath)
		file, err := os.Open(diskPath)
		if err != nil {
			log.Printf("Failed to open file at path %s: %v", diskPath, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to open file",
				"path":  diskPath,
				"details": err.Error(),
			})
			return
		}
		defer file.Close()

		// Set response headers
		c.Writer.Header().Set("Content-Type", metadata.MimeType)
		c.Writer.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))
		c.Writer.WriteHeader(http.StatusOK)

		// Use larger buffer for media files (1MB for better throughput)
		buffer := make([]byte, 1024*1024)
		_, err = io.CopyBuffer(c.Writer, file, buffer)
		if err != nil {
			log.Printf("Error streaming media file: %v", err)
		}
		return
	}

	// Fallback to compressed streaming
	s.streamFromDisk(c, diskPath, metadata)
}

// streamMediaFromRedis optimized Redis streaming for media files
func (s *FileService) streamMediaFromRedis(c *gin.Context, compressedContent string, metadata FileMetadata) {
	// Decompress if needed
	var content []byte
	var err error

	log.Printf("streamMediaFromRedis: compression=%s, content_length=%d", metadata.Compression, len(compressedContent))
	
	if metadata.Compression == CompressionNone {
		log.Printf("No compression, using content directly")
		content = []byte(compressedContent)
	} else {
		log.Printf("Decompressing content with %s", metadata.Compression)
		content, err = s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
		if err != nil {
			log.Printf("Failed to decompress file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	}

	// Set response headers
	c.Writer.Header().Set("Content-Type", metadata.MimeType)
	c.Writer.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))
	c.Writer.WriteHeader(http.StatusOK)

	// Stream with larger buffer for media files
	reader := bytes.NewReader(content)
	buffer := make([]byte, 1024*1024) // 1MB buffer
	_, err = io.CopyBuffer(c.Writer, reader, buffer)
	if err != nil {
		log.Printf("Error streaming media file: %v", err)
	}
}

// handleOptimizedRangeRequest handles range requests with optimizations for media files
func (s *FileService) handleOptimizedRangeRequest(c *gin.Context, compressedContent string, metadata FileMetadata, rangeHeader string) {
	// Parse range header
	ranges, err := parseRangeHeader(rangeHeader, metadata.Size)
	if err != nil {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", metadata.Size))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	if len(ranges) != 1 {
		// Multi-range not supported
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", metadata.Size))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	rangeSpec := ranges[0]
	contentLength := rangeSpec.end - rangeSpec.start + 1

	// Set headers for partial content
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeSpec.start, rangeSpec.end, metadata.Size))
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Content-Type", metadata.MimeType)
	c.Header("Cache-Control", "public, max-age=3600")
	c.Status(http.StatusPartialContent)

	// Stream the requested range with optimizations
	if strings.HasPrefix(compressedContent, "DISK:") {
		diskPath := strings.TrimPrefix(compressedContent, "DISK:")
		s.streamOptimizedRangeFromDisk(c, diskPath, metadata, rangeSpec)
	} else {
		s.streamOptimizedRangeFromRedis(c, compressedContent, metadata, rangeSpec)
	}
}

// streamOptimizedRangeFromDisk optimized range streaming from disk
func (s *FileService) streamOptimizedRangeFromDisk(c *gin.Context, diskPath string, metadata FileMetadata, rangeSpec Range) {
	// For uncompressed files, seek directly (most efficient)
	if metadata.Compression == CompressionNone {
		file, err := os.Open(diskPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
			return
		}
		defer file.Close()

		// Seek to start position
		if _, err := file.Seek(rangeSpec.start, 0); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to seek file"})
			return
		}

		// Stream the requested range with optimized buffer
		contentLength := rangeSpec.end - rangeSpec.start + 1
		buffer := make([]byte, 256*1024) // 256KB buffer for range requests
		remaining := contentLength

		for remaining > 0 {
			toRead := int64(len(buffer))
			if remaining < toRead {
				toRead = remaining
			}

			n, err := file.Read(buffer[:toRead])
			if err != nil && err != io.EOF {
				log.Printf("Error reading file range: %v", err)
				return
			}

			if n == 0 {
				break
			}

			if _, err := c.Writer.Write(buffer[:n]); err != nil {
				log.Printf("Error writing range response: %v", err)
				return
			}
			remaining -= int64(n)
		}
		return
	}

	// Fallback to compressed range streaming
	s.streamRangeFromDisk(c, diskPath, metadata, rangeSpec)
}

// streamOptimizedRangeFromRedis optimized range streaming from Redis
func (s *FileService) streamOptimizedRangeFromRedis(c *gin.Context, compressedContent string, metadata FileMetadata, rangeSpec Range) {
	// Decompress if needed
	var content []byte
	var err error

	if metadata.Compression == CompressionNone {
		content = []byte(compressedContent)
	} else {
		content, err = s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	}

	// Validate range
	if rangeSpec.start >= int64(len(content)) || rangeSpec.end >= int64(len(content)) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid range"})
		return
	}

	// Stream the requested range
	rangeContent := content[rangeSpec.start : rangeSpec.end+1]
	if _, err := c.Writer.Write(rangeContent); err != nil {
		log.Printf("Error writing range response: %v", err)
	}
}

func isPreviewable(mimeType string) bool {
	previewable := []string{
		"image/", "text/", "application/json", "application/xml",
		"video/", "audio/", "application/pdf",
	}

	for _, prefix := range previewable {
		if strings.HasPrefix(mimeType, prefix) {
			return true
		}
	}
	return false
}

func isMediaFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/") || strings.HasPrefix(mimeType, "audio/")
}

func isImageFile(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

func (s *FileService) getMetadata(c *gin.Context) {
	fileID := c.Param("id")

	// Get file metadata from PostgreSQL
	fileStorage, err := s.db.GetFileMetadata(fileID)
	if err != nil {
		log.Printf("Failed to get file metadata: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	
	if fileStorage == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found or expired"})
		return
	}

	// Convert to safe metadata (don't expose passwords)
	safeMetadata := FileMetadata{
		ID:                  fileStorage.ID,
		Filename:            fileStorage.Filename,
		Size:                fileStorage.OriginalSize,
		CompressedSize:      0,
		MimeType:            fileStorage.MimeType,
		Compression:         CompressionType(fileStorage.CompressionType),
		UploadTime:          fileStorage.UploadTime,
		ExpiresAt:           fileStorage.ExpiresAt,
		HasDownloadPassword: fileStorage.HasDownloadPassword,
	}
	
	if fileStorage.CompressedSize != nil {
		safeMetadata.CompressedSize = *fileStorage.CompressedSize
	}

	c.JSON(http.StatusOK, safeMetadata)
}

func (s *FileService) browseZip(c *gin.Context) {
	fileID := c.Param("id")
	ctx := context.Background()

	// Get metadata
	metadataJSON, err := s.redis.Get(ctx, "file:"+fileID).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	var metadata FileMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse metadata"})
		return
	}

	// Check if file has expired
	if metadata.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File has expired"})
		return
	}

	// Check if file is a ZIP
	if !strings.HasSuffix(strings.ToLower(metadata.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is not a ZIP archive"})
		return
	}

	// Get file content
	compressedContent, err := s.redis.Get(ctx, "content:"+fileID).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File content not found"})
		return
	}

	var content []byte
	// Check if file is stored on disk
	if strings.HasPrefix(compressedContent, "DISK:") {
		// Read from disk
		diskPath := strings.TrimPrefix(compressedContent, "DISK:")
		diskContent, err := os.ReadFile(diskPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file from disk"})
			return
		}

		// Decompress file
		content, err = s.compressor.Decompress(diskContent, metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	} else {
		// Read from Redis
		content, err = s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	}

	// Read ZIP contents
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read ZIP archive"})
		return
	}

	// Extract file list
	var files []map[string]interface{}
	for _, file := range zipReader.File {
		// Try to detect and convert encoding of filename
		fileName := detectAndConvertFilename(file.Name)

		fileInfo := map[string]interface{}{
			"name":       fileName,
			"size":       file.UncompressedSize64,
			"compressed": file.CompressedSize64,
			"modified":   file.Modified,
			"is_dir":     file.FileInfo().IsDir(),
			"method":     file.Method,
		}
		files = append(files, fileInfo)
	}

	c.JSON(http.StatusOK, gin.H{
		"filename": metadata.Filename,
		"files":    files,
		"total":    len(files),
	})
}

func (s *FileService) extractZipFile(c *gin.Context) {
	log.Printf("extractZipFile function called")
	fileID := c.Param("id")
	fileName := c.Query("filename")

	if fileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filename parameter is required"})
		return
	}

	log.Printf("Extracting file '%s' from ZIP %s", fileName, fileID)

	ctx := context.Background()

	// Get metadata
	metadataJSON, err := s.redis.Get(ctx, "file:"+fileID).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	var metadata FileMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse metadata"})
		return
	}

	// Check if file has expired
	if metadata.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "File has expired"})
		return
	}

	// Check if file is a ZIP
	if !strings.HasSuffix(strings.ToLower(metadata.Filename), ".zip") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is not a ZIP archive"})
		return
	}

	// Get file content
	compressedContent, err := s.redis.Get(ctx, "content:"+fileID).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File content not found"})
		return
	}

	var content []byte
	// Check if file is stored on disk
	if strings.HasPrefix(compressedContent, "DISK:") {
		// Read from disk
		diskPath := strings.TrimPrefix(compressedContent, "DISK:")
		diskContent, err := os.ReadFile(diskPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file from disk"})
			return
		}

		// Decompress file
		content, err = s.compressor.Decompress(diskContent, metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	} else {
		// Read from Redis
		content, err = s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}
	}

	// Read ZIP contents
	zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read ZIP archive"})
		return
	}

	// Find the requested file
	var targetFile *zip.File
	for _, file := range zipReader.File {
		convertedName := detectAndConvertFilename(file.Name)
		// Debug log for troubleshooting
		log.Printf("Comparing requested '%s' with ZIP file '%s' (converted: '%s')", fileName, file.Name, convertedName)
		if convertedName == fileName || file.Name == fileName {
			targetFile = file
			break
		}
	}

	if targetFile == nil {
		// Enhanced error message with available files
		var availableFiles []string
		for _, file := range zipReader.File {
			availableFiles = append(availableFiles, detectAndConvertFilename(file.Name))
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error":           "File not found in ZIP archive",
			"requested_file":  fileName,
			"available_files": availableFiles,
		})
		return
	}

	// Check if it's a directory
	if targetFile.FileInfo().IsDir() {
		log.Printf("Target file is a directory")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot preview directory"})
		return
	}
	log.Printf("Target file is not a directory, proceeding to open")

	// Open the file from ZIP
	rc, err := targetFile.Open()
	if err != nil {
		log.Printf("Failed to open file from ZIP: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file from ZIP"})
		return
	}
	defer rc.Close()
	log.Printf("File opened successfully from ZIP")

	// Read file content
	fileContent, err := io.ReadAll(rc)
	if err != nil {
		log.Printf("Failed to read file content: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file content"})
		return
	}
	log.Printf("File content read successfully, size: %d bytes", len(fileContent))

	// Determine MIME type
	convertedName := detectAndConvertFilename(targetFile.Name)
	log.Printf("About to call GetMimeType with: %s", convertedName)
	mimeType := GetMimeType(convertedName)
	log.Printf("GetMimeType returned: %s", mimeType)
	log.Printf("File: %s, Converted name: %s, MIME type: %s", targetFile.Name, convertedName, mimeType)

	// Check if file type is previewable
	if !isPreviewable(mimeType) {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"error":     "File type not previewable",
			"message":   "This file type cannot be previewed in the browser.",
			"mime_type": mimeType,
		})
		return
	}

	// Set appropriate headers for preview
	c.Header("Content-Type", mimeType)
	c.Header("Content-Length", strconv.FormatInt(int64(len(fileContent)), 10))
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%s", detectAndConvertFilename(targetFile.Name)))

	c.Data(http.StatusOK, mimeType, fileContent)
}

// streamFileContent streams large files to avoid memory issues
func (s *FileService) streamFileContent(c *gin.Context, compressedContent string, metadata FileMetadata) {
	if strings.HasPrefix(compressedContent, "DISK:") {
		// Stream from disk
		diskPath := strings.TrimPrefix(compressedContent, "DISK:")
		s.streamFromDisk(c, diskPath, metadata)
	} else {
		// Stream from Redis (less common for large files)
		s.streamFromRedis(c, compressedContent, metadata)
	}
}

// streamFromDisk streams file content from disk with compression support
func (s *FileService) streamFromDisk(c *gin.Context, diskPath string, metadata FileMetadata) {
	// Open compressed file
	log.Printf("Opening file from disk: %s", diskPath)
	file, err := os.Open(diskPath)
	if err != nil {
		log.Printf("Failed to open file from disk %s: %v", diskPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to open file from disk",
			"path":  diskPath,
			"details": err.Error(),
		})
		return
	}
	defer file.Close()

	// Create decompression reader based on compression type
	var reader io.Reader
	switch metadata.Compression {
	case CompressionNone:
		reader = file
	case CompressionGzip:
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create gzip reader"})
			return
		}
		defer gzReader.Close()
		reader = gzReader
	case CompressionZstd:
		zstdReader, err := zstd.NewReader(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create zstd reader"})
			return
		}
		defer zstdReader.Close()
		reader = zstdReader
	case CompressionLZ4:
		lz4Reader := lz4.NewReader(file)
		reader = lz4Reader
	default:
		reader = file
	}

	// Stream content to client
	c.Writer.Header().Set("Content-Type", metadata.MimeType)
	c.Writer.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))
	c.Writer.WriteHeader(http.StatusOK)

	// Copy with buffering to control memory usage
	buffer := make([]byte, 64*1024) // 64KB buffer
	_, err = io.CopyBuffer(c.Writer, reader, buffer)
	if err != nil {
		log.Printf("Error streaming file: %v", err)
	}
}

// streamFromRedis streams file content from Redis (for smaller large files)
func (s *FileService) streamFromRedis(c *gin.Context, compressedContent string, metadata FileMetadata) {
	// Decompress content
	content, err := s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
		return
	}

	// Stream to client
	c.Writer.Header().Set("Content-Type", metadata.MimeType)
	c.Writer.Header().Set("Content-Length", strconv.FormatInt(metadata.Size, 10))
	c.Writer.WriteHeader(http.StatusOK)

	// Write in chunks to avoid memory spikes
	reader := bytes.NewReader(content)
	buffer := make([]byte, 64*1024) // 64KB buffer
	_, err = io.CopyBuffer(c.Writer, reader, buffer)
	if err != nil {
		log.Printf("Error streaming file: %v", err)
	}
}

// handleRangeRequest handles HTTP Range requests for partial content
func (s *FileService) handleRangeRequest(c *gin.Context, compressedContent string, metadata FileMetadata, rangeHeader string) {
	// Parse range header
	ranges, err := parseRangeHeader(rangeHeader, metadata.Size)
	if err != nil {
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", metadata.Size))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	if len(ranges) != 1 {
		// Multi-range not supported for now
		c.Header("Content-Range", fmt.Sprintf("bytes */%d", metadata.Size))
		c.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	rangeSpec := ranges[0]
	contentLength := rangeSpec.end - rangeSpec.start + 1

	// Set headers for partial content
	c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeSpec.start, rangeSpec.end, metadata.Size))
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))
	c.Header("Content-Type", metadata.MimeType)
	c.Status(http.StatusPartialContent)

	// Stream the requested range
	if strings.HasPrefix(compressedContent, "DISK:") {
		diskPath := strings.TrimPrefix(compressedContent, "DISK:")
		s.streamRangeFromDisk(c, diskPath, metadata, rangeSpec)
	} else {
		s.streamRangeFromRedis(c, compressedContent, metadata, rangeSpec)
	}
}

// Range represents a byte range
type Range struct {
	start int64
	end   int64
}

// parseRangeHeader parses HTTP Range header
func parseRangeHeader(rangeHeader string, fileSize int64) ([]Range, error) {
	// Remove "bytes=" prefix
	rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
	
	// Parse range specifications
	rangeSpecs := strings.Split(rangeHeader, ",")
	var ranges []Range
	
	for _, spec := range rangeSpecs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		
		// Parse individual range spec
		if strings.HasPrefix(spec, "-") {
			// Suffix range: -500 (last 500 bytes)
			suffix, err := strconv.ParseInt(spec[1:], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid range suffix: %s", spec)
			}
			start := fileSize - suffix
			if start < 0 {
				start = 0
			}
			ranges = append(ranges, Range{start: start, end: fileSize - 1})
		} else if strings.HasSuffix(spec, "-") {
			// Start range: 500- (from byte 500 to end)
			start, err := strconv.ParseInt(spec[:len(spec)-1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid range start: %s", spec)
			}
			if start >= fileSize {
				return nil, fmt.Errorf("range start beyond file size")
			}
			ranges = append(ranges, Range{start: start, end: fileSize - 1})
		} else {
			// Full range: 500-1000
			parts := strings.Split(spec, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid range format: %s", spec)
			}
			start, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid range start: %s", parts[0])
			}
			end, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid range end: %s", parts[1])
			}
			if start > end || start >= fileSize {
				return nil, fmt.Errorf("invalid range: %d-%d", start, end)
			}
			if end >= fileSize {
				end = fileSize - 1
			}
			ranges = append(ranges, Range{start: start, end: end})
		}
	}
	
	return ranges, nil
}

// streamRangeFromDisk streams a specific range from disk
func (s *FileService) streamRangeFromDisk(c *gin.Context, diskPath string, metadata FileMetadata, rangeSpec Range) {
	// For compressed files, we need to decompress first (less efficient for ranges)
	// In a production system, consider storing large files uncompressed for better range support
	if metadata.Compression != CompressionNone {
		// Decompress entire file first (not ideal but necessary for compressed files)
		file, err := os.Open(diskPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
			return
		}
		defer file.Close()

		content, err := s.compressor.Decompress(readFileContent(file), metadata.Compression)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
			return
		}

		// Stream the requested range
		rangeContent := content[rangeSpec.start : rangeSpec.end+1]
		c.Writer.Write(rangeContent)
		return
	}

	// For uncompressed files, we can seek directly
	file, err := os.Open(diskPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer file.Close()

	// Seek to start position
	if _, err := file.Seek(rangeSpec.start, 0); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to seek file"})
		return
	}

	// Stream the requested range
	contentLength := rangeSpec.end - rangeSpec.start + 1
	buffer := make([]byte, 64*1024) // 64KB buffer
	remaining := contentLength

	for remaining > 0 {
		toRead := int64(len(buffer))
		if remaining < toRead {
			toRead = remaining
		}

		n, err := file.Read(buffer[:toRead])
		if err != nil && err != io.EOF {
			log.Printf("Error reading file range: %v", err)
			return
		}

		if n == 0 {
			break
		}

		c.Writer.Write(buffer[:n])
		remaining -= int64(n)
	}
}

// streamRangeFromRedis streams a specific range from Redis
func (s *FileService) streamRangeFromRedis(c *gin.Context, compressedContent string, metadata FileMetadata, rangeSpec Range) {
	// Decompress content
	content, err := s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
		return
	}

	// Stream the requested range
	rangeContent := content[rangeSpec.start : rangeSpec.end+1]
	c.Writer.Write(rangeContent)
}

// readFileContent reads all content from a file
func readFileContent(file *os.File) []byte {
	content, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Error reading file content: %v", err)
		return nil
	}
	return content
}

type UpdateExpirationRequest struct {
	AdminPassword string `json:"admin_password"`
	ExpiresAt     string `json:"expires_at"`
}

type AdminRequest struct {
	AdminPassword string `json:"admin_password"`
}

type AdminAuthResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type AdminClaims struct {
	IsAdmin bool `json:"is_admin"`
	jwt.RegisteredClaims
}

var jwtSecret = []byte("admin-jwt-secret-key-change-in-production")

func (s *FileService) generateAdminToken() (string, int64, error) {
	expirationTime := time.Now().Add(2 * time.Hour)
	claims := &AdminClaims{
		IsAdmin: true,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "admin",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", 0, err
	}

	return tokenString, expirationTime.Unix(), nil
}

func (s *FileService) validateAdminToken(tokenString string) (*AdminClaims, error) {
	claims := &AdminClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid || !claims.IsAdmin {
		return nil, fmt.Errorf("invalid admin token")
	}

	return claims, nil
}

func (s *FileService) adminAuth(c *gin.Context) {
	var req AdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if s.config.AdminPassword == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Admin functionality not configured",
			"message": "ADMIN_PASSWORD environment variable not set",
		})
		return
	}

	if req.AdminPassword != s.config.AdminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid admin password",
			"message": "The provided admin password is incorrect",
		})
		return
	}

	token, expiresAt, err := s.generateAdminToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AdminAuthResponse{
		Token:     token,
		ExpiresAt: expiresAt,
	})
}

func (s *FileService) updateFileExpiration(c *gin.Context) {
	fileID := c.Param("id")
	ctx := context.Background()

	var req UpdateExpirationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if s.config.AdminPassword == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Admin functionality not configured",
			"message": "ADMIN_PASSWORD environment variable not set",
		})
		return
	}

	if req.AdminPassword != s.config.AdminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid admin password",
			"message": "The provided admin password is incorrect",
		})
		return
	}

	expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid expiration time format",
			"message": "Please use RFC3339 format (e.g., 2023-12-31T23:59:59Z)",
		})
		return
	}

	metadataJSON, err := s.redis.Get(ctx, "file:"+fileID).Result()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file metadata"})
		return
	}

	var metadata FileMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse file metadata"})
		return
	}

	oldExpiresAt := metadata.ExpiresAt
	metadata.ExpiresAt = expiresAt

	updatedMetadataJSON, err := json.Marshal(metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize updated metadata"})
		return
	}

	newExpiration := time.Until(expiresAt)
	if newExpiration <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid expiration time",
			"message": "Expiration time must be in the future",
		})
		return
	}

	pipe := s.redis.Pipeline()
	pipe.Set(ctx, "file:"+fileID, updatedMetadataJSON, newExpiration)
	pipe.Expire(ctx, "content:"+fileID, newExpiration)
	pipe.ZAdd(ctx, "files", &redis.Z{
		Score:  float64(expiresAt.Unix()),
		Member: fileID,
	})

	if _, err := pipe.Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update file expiration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File expiration updated successfully",
		"file_id": fileID,
		"old_expires_at": oldExpiresAt,
		"new_expires_at": expiresAt,
		"metadata": metadata,
	})
}

func (s *FileService) adminDeleteFile(c *gin.Context) {
	fileID := c.Param("id")
	ctx := context.Background()

	var req AdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if s.config.AdminPassword == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Admin functionality not configured",
			"message": "ADMIN_PASSWORD environment variable not set",
		})
		return
	}

	if req.AdminPassword != s.config.AdminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid admin password",
			"message": "The provided admin password is incorrect",
		})
		return
	}

	// Check if file exists
	metadataJSON, err := s.redis.Get(ctx, "file:"+fileID).Result()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file metadata"})
		return
	}

	var metadata FileMetadata
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse file metadata"})
		return
	}

	// Delete from Redis
	pipe := s.redis.Pipeline()
	pipe.Del(ctx, "file:"+fileID)
	pipe.Del(ctx, "content:"+fileID)
	pipe.ZRem(ctx, "files", fileID)

	// Delete from disk if stored there (large files > 100MB)
	// Check if file size indicates disk storage
	if metadata.Size > 100*1024*1024 {
		filesDir := filepath.Join(s.config.TempDir, "files")
		storagePath := filepath.Join(filesDir, fileID)
		if err := os.Remove(storagePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to delete file from disk: %v", err)
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File deleted successfully",
		"file_id": fileID,
		"filename": metadata.Filename,
	})
}

func (s *FileService) getAdminFileList(c *gin.Context) {
	ctx := context.Background()

	var req AdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if s.config.AdminPassword == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Admin functionality not configured",
			"message": "ADMIN_PASSWORD environment variable not set",
		})
		return
	}

	if req.AdminPassword != s.config.AdminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid admin password",
			"message": "The provided admin password is incorrect",
		})
		return
	}

	// Get all files from PostgreSQL database
	query := `
		SELECT id, filename, original_size, compressed_size, mime_type, compression_type,
			   storage_type, storage_path, upload_time, expires_at, has_download_password
		FROM files 
		WHERE expires_at > NOW()
		ORDER BY upload_time DESC
	`
	
	rows, err := s.db.Pool.Query(ctx, query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file list from database"})
		return
	}
	defer rows.Close()

	files := make([]map[string]interface{}, 0)

	for rows.Next() {
		var fileID, filename, mimeType, compressionType, storageType string
		var originalSize int64
		var compressedSize *int64
		var storagePath *string
		var uploadTime, expiresAt time.Time
		var hasDownloadPassword bool

		err := rows.Scan(&fileID, &filename, &originalSize, &compressedSize, &mimeType, 
			&compressionType, &storageType, &storagePath, &uploadTime, &expiresAt, &hasDownloadPassword)
		if err != nil {
			log.Printf("Failed to scan file row: %v", err)
			continue
		}

		// Get actual file size and storage info
		var actualFileSize int64
		var compressed bool
		
		if storageType == "disk" && storagePath != nil {
			// For disk-stored files, get actual file size
			if fileInfo, err := os.Stat(*storagePath); err == nil {
				actualFileSize = fileInfo.Size()
			} else {
				actualFileSize = originalSize
				log.Printf("Warning: disk file not found for %s at %s", fileID, *storagePath)
			}
			compressed = compressionType != "none"
		} else if storageType == "postgresql" {
			// For PostgreSQL-stored files, use compressed size if available
			if compressedSize != nil {
				actualFileSize = *compressedSize
			} else {
				actualFileSize = originalSize
			}
			compressed = compressionType != "none"
		} else {
			// Fallback
			actualFileSize = originalSize
			compressed = false
		}

		files = append(files, map[string]interface{}{
			"file_id":       fileID,
			"filename":      filename,
			"size":          actualFileSize,
			"original_size": originalSize,
			"uploaded_at":   uploadTime,
			"expires_at":    expiresAt,
			"storage_type":  storageType, // "postgresql" or "disk"
			"storage_path":  storagePath, // disk path if applicable
			"compressed":    compressed,
			"compression":   compressionType,
			"mime_type":     mimeType,
			"has_password":  hasDownloadPassword,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "File list retrieved successfully",
		"count":   len(files),
		"files":   files,
	})
}
