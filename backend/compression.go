package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"mime"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGzip CompressionType = "gzip"
	CompressionZstd CompressionType = "zstd"
	CompressionLZ4  CompressionType = "lz4"
)

type CompressionManager struct {
	zstdEncoder *zstd.Encoder
	zstdDecoder *zstd.Decoder
}

func NewCompressionManager() *CompressionManager {
	encoder, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	decoder, _ := zstd.NewReader(nil)
	
	return &CompressionManager{
		zstdEncoder: encoder,
		zstdDecoder: decoder,
	}
}

func (cm *CompressionManager) SelectCompressionType(filename string, size int64) CompressionType {
	// Don't compress already compressed files
	ext := strings.ToLower(filepath.Ext(filename))
	compressedExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".mp3": true, ".aac": true, ".ogg": true, ".flac": true,
		".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true,
		".pdf": true,
	}
	
	if compressedExts[ext] {
		return CompressionNone
	}

	// For very large files (>500MB), skip compression to avoid memory issues and improve performance
	if size > 500*1024*1024 {
		log.Printf("Skipping compression for very large file: %s (%d bytes)", filename, size)
		return CompressionNone
	}

	// For large files (>100MB), use fast compression only
	if size > 100*1024*1024 {
		return CompressionLZ4
	}

	// For small files, use LZ4 for speed
	if size < 1024*10 { // 10KB
		return CompressionLZ4
	}

	// For medium files, use Zstandard for balance
	if size < 1024*1024*10 { // 10MB
		return CompressionZstd
	}

	// For moderately large files, use LZ4 for better performance
	return CompressionLZ4
}

func (cm *CompressionManager) Compress(data []byte, compressionType CompressionType) ([]byte, error) {
	switch compressionType {
	case CompressionNone:
		return data, nil
	case CompressionGzip:
		return cm.compressGzip(data)
	case CompressionZstd:
		return cm.compressZstd(data)
	case CompressionLZ4:
		return cm.compressLZ4(data)
	default:
		return data, nil
	}
}

func (cm *CompressionManager) Decompress(data []byte, compressionType CompressionType) ([]byte, error) {
	switch compressionType {
	case CompressionNone:
		return data, nil
	case CompressionGzip:
		return cm.decompressGzip(data)
	case CompressionZstd:
		return cm.decompressZstd(data)
	case CompressionLZ4:
		return cm.decompressLZ4(data)
	default:
		return data, nil
	}
}

func (cm *CompressionManager) compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}
	writer.Close()
	return buf.Bytes(), nil
}

func (cm *CompressionManager) decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (cm *CompressionManager) compressZstd(data []byte) ([]byte, error) {
	return cm.zstdEncoder.EncodeAll(data, nil), nil
}

func (cm *CompressionManager) decompressZstd(data []byte) ([]byte, error) {
	return cm.zstdDecoder.DecodeAll(data, nil)
}

func (cm *CompressionManager) compressLZ4(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := lz4.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}
	writer.Close()
	return buf.Bytes(), nil
}

func (cm *CompressionManager) decompressLZ4(data []byte) ([]byte, error) {
	reader := lz4.NewReader(bytes.NewReader(data))
	return io.ReadAll(reader)
}

func GetMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	log.Printf("GetMimeType called with filename: %s, ext: %s", filename, ext)
	
	// Manual mapping for common types (fallback first)
	switch ext {
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "text/javascript"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	// Video files
	case ".mp4":
		log.Printf("GetMimeType: detected .mp4 file, returning video/mp4")
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogv":
		return "video/ogg"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".wmv":
		return "video/x-ms-wmv"
	case ".flv":
		return "video/x-flv"
	case ".mkv":
		return "video/x-matroska"
	// Audio files
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".m4a":
		return "audio/mp4"
	}
	
	// Try Go standard library as fallback
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" {
		log.Printf("GetMimeType: Go standard library returned: %s", mimeType)
		return mimeType
	}
	
	// Default fallback
	log.Printf("GetMimeType: returning default fallback: application/octet-stream")
	return "application/octet-stream"
}