package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type FileMetadata struct {
	ID             string          `json:"id"`
	Filename       string          `json:"filename"`
	Size           int64           `json:"size"`
	CompressedSize int64           `json:"compressed_size"`
	MimeType       string          `json:"mime_type"`
	Compression    CompressionType `json:"compression"`
	UploadTime     time.Time       `json:"upload_time"`
	ExpiresAt      time.Time       `json:"expires_at"`
	DeletePassword string          `json:"delete_password,omitempty"`
	DownloadPassword string        `json:"download_password,omitempty"`
	HasDownloadPassword bool       `json:"has_download_password"`
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

// containsJapanese checks if the string contains Japanese characters
func containsJapanese(s string) bool {
	for _, r := range s {
		// Check for Hiragana, Katakana, and Kanji ranges
		if (r >= 0x3040 && r <= 0x309F) || // Hiragana
		   (r >= 0x30A0 && r <= 0x30FF) || // Katakana
		   (r >= 0x4E00 && r <= 0x9FAF) {  // Kanji
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
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

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
		ID:             fileID,
		Filename:       header.Filename,
		Size:           header.Size,
		CompressedSize: int64(len(compressedContent)),
		MimeType:       detectedMimeType,
		Compression:    compressionType,
		UploadTime:     now,
		ExpiresAt:      expiresAt,
		DeletePassword: deletePassword,
		DownloadPassword: downloadPassword,
		HasDownloadPassword: hasDownloadPassword,
	}

	// Store file content with 24-hour expiration
	expiration := 24 * time.Hour
	if err := s.redis.Set(ctx, "content:"+fileID, compressedContent, expiration).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store file"})
		return
	}

	// Store metadata with 24-hour expiration
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to serialize metadata"})
		return
	}

	if err := s.redis.Set(ctx, "file:"+fileID, metadataJSON, expiration).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store metadata"})
		return
	}

	// Add to file list with expiration score
	if err := s.redis.ZAdd(ctx, "files", &redis.Z{
		Score:  float64(expiresAt.Unix()),
		Member: fileID,
	}).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add to file list"})
		return
	}
	
	// Set expiration on the file list entry
	s.redis.Expire(ctx, "files", expiration)

	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully",
		"file_id":  fileID,
		"metadata": metadata,
	})
}


func (s *FileService) getFile(c *gin.Context) {
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

	// Check download password if required
	if metadata.HasDownloadPassword {
		providedPassword := c.Query("password")
		if providedPassword != metadata.DownloadPassword {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Password required",
				"message": "This file is password protected. Please provide the correct password.",
			})
			return
		}
	}

	// Get file content
	compressedContent, err := s.redis.Get(ctx, "content:"+fileID).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File content not found"})
		return
	}

	// Decompress file
	content, err := s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
		return
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

	// Get metadata to check delete password
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

	// Check delete password
	providedPassword := c.Query("delete_password")
	if providedPassword != metadata.DeletePassword {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid delete password",
			"message": "The provided delete password is incorrect.",
		})
		return
	}

	// Delete file content and metadata
	pipe := s.redis.Pipeline()
	pipe.Del(ctx, "file:"+fileID)
	pipe.Del(ctx, "content:"+fileID)
	pipe.ZRem(ctx, "files", fileID)
	
	if _, err := pipe.Exec(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}

func (s *FileService) previewFile(c *gin.Context) {
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

	// Check download password if required
	if metadata.HasDownloadPassword {
		providedPassword := c.Query("password")
		if providedPassword != metadata.DownloadPassword {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Password required",
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
			"error": "File type not previewable",
			"message": "This file type cannot be previewed in the browser. Please download the file to view it.",
			"mime_type": metadata.MimeType,
			"suggested_action": "download",
		})
		return
	}

	// Get file content
	compressedContent, err := s.redis.Get(ctx, "content:"+fileID).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File content not found"})
		return
	}

	// Decompress file
	content, err := s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
		return
	}

	// Set appropriate headers for preview
	c.Header("Content-Type", metadata.MimeType)
	c.Header("Content-Length", strconv.FormatInt(metadata.Size, 10))

	c.Data(http.StatusOK, metadata.MimeType, content)
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

func (s *FileService) getMetadata(c *gin.Context) {
	fileID := c.Param("id")
	ctx := context.Background()

	// Get metadata
	metadataJSON, err := s.redis.Get(ctx, "file:"+fileID).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found or expired"})
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

	// Don't expose passwords in metadata response
	safeMetadata := FileMetadata{
		ID:                  metadata.ID,
		Filename:            metadata.Filename,
		Size:                metadata.Size,
		CompressedSize:      metadata.CompressedSize,
		MimeType:            metadata.MimeType,
		Compression:         metadata.Compression,
		UploadTime:          metadata.UploadTime,
		ExpiresAt:           metadata.ExpiresAt,
		HasDownloadPassword: metadata.HasDownloadPassword,
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

	// Decompress file
	content, err := s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
		return
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
			"name":         fileName,
			"size":         file.UncompressedSize64,
			"compressed":   file.CompressedSize64,
			"modified":     file.Modified,
			"is_dir":       file.FileInfo().IsDir(),
			"method":       file.Method,
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

	// Decompress file
	content, err := s.compressor.Decompress([]byte(compressedContent), metadata.Compression)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decompress file"})
		return
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
			"error": "File not found in ZIP archive",
			"requested_file": fileName,
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
			"error": "File type not previewable",
			"message": "This file type cannot be previewed in the browser.",
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

