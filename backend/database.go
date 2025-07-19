package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/jackc/pgx/v4/stdlib"
)

type Database struct {
	Pool   *pgxpool.Pool
	config *Config
}

// NewDatabase creates a new database connection pool
func NewDatabase(config *Config) (*Database, error) {
	var connStr string

	// Use DATABASE_URL if provided, otherwise construct from individual components
	if config.DatabaseURL != "" {
		connStr = config.DatabaseURL
	} else {
		connStr = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			config.DatabaseUser,
			config.DatabasePassword,
			config.DatabaseHost,
			config.DatabasePort,
			config.DatabaseName,
			config.DatabaseSSLMode,
		)
	}

	// Configure connection pool
	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %v", err)
	}

	// Set pool configuration
	poolConfig.MaxConns = int32(config.DatabaseMaxConns)
	poolConfig.MinConns = int32(config.DatabaseMinConns)
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Create connection pool
	pool, err := pgxpool.ConnectConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	db := &Database{
		Pool:   pool,
		config: config,
	}

	// Test connection
	if err := db.Ping(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	log.Printf("Successfully connected to PostgreSQL database")
	return db, nil
}

// Ping tests the database connection
func (db *Database) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return db.Pool.Ping(ctx)
}

// Close closes the database connection pool
func (db *Database) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// RunMigrations applies database schema migrations
func (db *Database) RunMigrations() error {
	log.Printf("Running database migrations...")

	// Read schema file
	schemaPath := filepath.Join(".", "schema.sql")
	schemaSQL, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %v", err)
	}

	// Execute schema
	ctx := context.Background()
	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire database connection: %v", err)
	}
	defer conn.Release()

	// Start transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Execute schema SQL
	if _, err := tx.Exec(ctx, string(schemaSQL)); err != nil {
		return fmt.Errorf("failed to execute schema: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit migration transaction: %v", err)
	}

	log.Printf("Database migrations completed successfully")
	return nil
}

// CheckSchemaExists checks if the database schema is already initialized
func (db *Database) CheckSchemaExists() (bool, error) {
	ctx := context.Background()
	
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'files'
		);
	`
	
	var exists bool
	err := db.Pool.QueryRow(ctx, query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check schema existence: %v", err)
	}
	
	return exists, nil
}

// CleanupExpiredData removes expired files and old data
func (db *Database) CleanupExpiredData() error {
	ctx := context.Background()
	
	// Call the cleanup function defined in schema
	var deletedCount int
	err := db.Pool.QueryRow(ctx, "SELECT cleanup_expired_data()").Scan(&deletedCount)
	if err != nil {
		return fmt.Errorf("failed to cleanup expired data: %v", err)
	}
	
	if deletedCount > 0 {
		log.Printf("Cleaned up %d expired files from database", deletedCount)
	}
	
	return nil
}

// FileStorage represents file metadata and content in the database
type FileStorage struct {
	ID              string    `db:"id"`
	Filename        string    `db:"filename"`
	OriginalSize    int64     `db:"original_size"`
	CompressedSize  *int64    `db:"compressed_size"`
	MimeType        string    `db:"mime_type"`
	CompressionType string    `db:"compression_type"`
	StorageType     string    `db:"storage_type"`
	StoragePath     *string   `db:"storage_path"`
	FileContent     []byte    `db:"file_content"`
	UploadTime      time.Time `db:"upload_time"`
	ExpiresAt       time.Time `db:"expires_at"`
	DeletePassword  string    `db:"delete_password"`
	DownloadPassword *string  `db:"download_password"`
	HasDownloadPassword bool  `db:"has_download_password"`
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

// SaveFile saves file metadata and content to the database
func (db *Database) SaveFile(file *FileStorage) error {
	ctx := context.Background()
	
	query := `
		INSERT INTO files (
			id, filename, original_size, compressed_size, mime_type, compression_type,
			storage_type, storage_path, file_content, upload_time, expires_at, delete_password,
			download_password, has_download_password
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
	`
	
	_, err := db.Pool.Exec(ctx, query,
		file.ID, file.Filename, file.OriginalSize, file.CompressedSize,
		file.MimeType, file.CompressionType, file.StorageType, file.StoragePath,
		file.FileContent, file.UploadTime, file.ExpiresAt, file.DeletePassword,
		file.DownloadPassword, file.HasDownloadPassword,
	)
	
	if err != nil {
		return fmt.Errorf("failed to save file metadata and content: %v", err)
	}
	
	return nil
}

// GetFile retrieves file metadata and content from the database
func (db *Database) GetFile(fileID string) (*FileStorage, error) {
	ctx := context.Background()
	
	query := `
		SELECT id, filename, original_size, compressed_size, mime_type, compression_type,
			   storage_type, storage_path, file_content, upload_time, expires_at, delete_password,
			   download_password, has_download_password, created_at, updated_at
		FROM files
		WHERE id = $1 AND expires_at > NOW()
	`
	
	var file FileStorage
	err := db.Pool.QueryRow(ctx, query, fileID).Scan(
		&file.ID, &file.Filename, &file.OriginalSize, &file.CompressedSize,
		&file.MimeType, &file.CompressionType, &file.StorageType, &file.StoragePath,
		&file.FileContent, &file.UploadTime, &file.ExpiresAt, &file.DeletePassword,
		&file.DownloadPassword, &file.HasDownloadPassword,
		&file.CreatedAt, &file.UpdatedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // File not found or expired
		}
		return nil, fmt.Errorf("failed to get file metadata and content: %v", err)
	}
	
	return &file, nil
}

// GetFileMetadata retrieves only file metadata (without content) from the database
func (db *Database) GetFileMetadata(fileID string) (*FileStorage, error) {
	ctx := context.Background()
	
	query := `
		SELECT id, filename, original_size, compressed_size, mime_type, compression_type,
			   storage_type, storage_path, upload_time, expires_at, delete_password,
			   download_password, has_download_password, created_at, updated_at
		FROM files
		WHERE id = $1 AND expires_at > NOW()
	`
	
	var file FileStorage
	err := db.Pool.QueryRow(ctx, query, fileID).Scan(
		&file.ID, &file.Filename, &file.OriginalSize, &file.CompressedSize,
		&file.MimeType, &file.CompressionType, &file.StorageType, &file.StoragePath,
		&file.UploadTime, &file.ExpiresAt, &file.DeletePassword,
		&file.DownloadPassword, &file.HasDownloadPassword,
		&file.CreatedAt, &file.UpdatedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // File not found or expired
		}
		return nil, fmt.Errorf("failed to get file metadata: %v", err)
	}
	
	return &file, nil
}

// GetFileContent retrieves only file content from the database
func (db *Database) GetFileContent(fileID string) ([]byte, error) {
	ctx := context.Background()
	
	query := `
		SELECT file_content
		FROM files
		WHERE id = $1 AND expires_at > NOW()
	`
	
	var content []byte
	err := db.Pool.QueryRow(ctx, query, fileID).Scan(&content)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("file not found or expired")
		}
		return nil, fmt.Errorf("failed to get file content: %v", err)
	}
	
	return content, nil
}

// DeleteFile removes file metadata from the database
func (db *Database) DeleteFile(fileID string) error {
	ctx := context.Background()
	
	query := `DELETE FROM files WHERE id = $1`
	result, err := db.Pool.Exec(ctx, query, fileID)
	if err != nil {
		return fmt.Errorf("failed to delete file metadata: %v", err)
	}
	
	if result.RowsAffected() == 0 {
		return fmt.Errorf("file not found")
	}
	
	return nil
}

// ChunkUploadStorage represents chunk upload session in the database
type ChunkUploadStorage struct {
	UploadID           string    `db:"upload_id"`
	Filename           string    `db:"filename"`
	TotalSize          int64     `db:"total_size"`
	TotalChunks        int       `db:"total_chunks"`
	ChunkSize          int64     `db:"chunk_size"`
	ReceivedChunks     []bool    `db:"received_chunks"`
	FileHash           *string   `db:"file_hash"`
	DownloadPassword   *string   `db:"download_password"`
	HasDownloadPassword bool     `db:"has_download_password"`
	CreatedAt          time.Time `db:"created_at"`
	LastActivity       time.Time `db:"last_activity"`
	ExpiresAt          time.Time `db:"expires_at"`
	Status             string    `db:"status"`
}

// SaveChunkUpload saves chunk upload session to the database
func (db *Database) SaveChunkUpload(upload *ChunkUploadStorage) error {
	ctx := context.Background()
	
	// Convert []bool to JSONB format
	receivedChunksJSON, err := json.Marshal(upload.ReceivedChunks)
	if err != nil {
		return fmt.Errorf("failed to marshal received chunks: %v", err)
	}
	
	query := `
		INSERT INTO chunk_uploads (
			upload_id, filename, total_size, total_chunks, chunk_size,
			received_chunks, file_hash, download_password, has_download_password,
			last_activity, expires_at, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		ON CONFLICT (upload_id) DO UPDATE SET
			received_chunks = EXCLUDED.received_chunks,
			last_activity = EXCLUDED.last_activity,
			status = EXCLUDED.status
	`
	
	_, err = db.Pool.Exec(ctx, query,
		upload.UploadID, upload.Filename, upload.TotalSize, upload.TotalChunks,
		upload.ChunkSize, receivedChunksJSON, upload.FileHash,
		upload.DownloadPassword, upload.HasDownloadPassword,
		upload.LastActivity, upload.ExpiresAt, upload.Status,
	)
	
	if err != nil {
		return fmt.Errorf("failed to save chunk upload: %v", err)
	}
	
	return nil
}

// GetChunkUpload retrieves chunk upload session from the database
func (db *Database) GetChunkUpload(uploadID string) (*ChunkUploadStorage, error) {
	ctx := context.Background()
	
	query := `
		SELECT upload_id, filename, total_size, total_chunks, chunk_size,
			   received_chunks, file_hash, download_password, has_download_password,
			   created_at, last_activity, expires_at, status
		FROM chunk_uploads
		WHERE upload_id = $1 AND expires_at > NOW()
	`
	
	var upload ChunkUploadStorage
	var receivedChunksJSON []byte
	
	err := db.Pool.QueryRow(ctx, query, uploadID).Scan(
		&upload.UploadID, &upload.Filename, &upload.TotalSize, &upload.TotalChunks,
		&upload.ChunkSize, &receivedChunksJSON, &upload.FileHash,
		&upload.DownloadPassword, &upload.HasDownloadPassword,
		&upload.CreatedAt, &upload.LastActivity, &upload.ExpiresAt, &upload.Status,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Upload not found or expired
		}
		return nil, fmt.Errorf("failed to get chunk upload: %v", err)
	}
	
	// Unmarshal received chunks
	if err := json.Unmarshal(receivedChunksJSON, &upload.ReceivedChunks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal received chunks: %v", err)
	}
	
	return &upload, nil
}

// DeleteChunkUpload removes chunk upload session from the database
func (db *Database) DeleteChunkUpload(uploadID string) error {
	ctx := context.Background()
	
	query := `DELETE FROM chunk_uploads WHERE upload_id = $1`
	_, err := db.Pool.Exec(ctx, query, uploadID)
	if err != nil {
		return fmt.Errorf("failed to delete chunk upload: %v", err)
	}
	
	return nil
}

// ProcessingJobStorage represents processing job in the database
type ProcessingJobStorage struct {
	JobID       string     `db:"job_id"`
	UploadID    string     `db:"upload_id"`
	FileID      *string    `db:"file_id"`
	Status      string     `db:"status"`
	Progress    int        `db:"progress"`
	ErrorMessage *string   `db:"error_message"`
	ResultData  []byte     `db:"result_data"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	CompletedAt *time.Time `db:"completed_at"`
}

// SaveProcessingJob saves processing job to the database
func (db *Database) SaveProcessingJob(job *ProcessingJobStorage) error {
	ctx := context.Background()
	
	query := `
		INSERT INTO processing_jobs (
			job_id, upload_id, file_id, status, progress, error_message,
			result_data, completed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		ON CONFLICT (job_id) DO UPDATE SET
			file_id = EXCLUDED.file_id,
			status = EXCLUDED.status,
			progress = EXCLUDED.progress,
			error_message = EXCLUDED.error_message,
			result_data = EXCLUDED.result_data,
			completed_at = EXCLUDED.completed_at,
			updated_at = NOW()
	`
	
	_, err := db.Pool.Exec(ctx, query,
		job.JobID, job.UploadID, job.FileID, job.Status, job.Progress,
		job.ErrorMessage, job.ResultData, job.CompletedAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to save processing job: %v", err)
	}
	
	return nil
}

// GetProcessingJob retrieves processing job from the database
func (db *Database) GetProcessingJob(jobID string) (*ProcessingJobStorage, error) {
	ctx := context.Background()
	
	query := `
		SELECT job_id, upload_id, file_id, status, progress, error_message,
			   result_data, created_at, updated_at, completed_at
		FROM processing_jobs
		WHERE job_id = $1
	`
	
	var job ProcessingJobStorage
	err := db.Pool.QueryRow(ctx, query, jobID).Scan(
		&job.JobID, &job.UploadID, &job.FileID, &job.Status, &job.Progress,
		&job.ErrorMessage, &job.ResultData, &job.CreatedAt, &job.UpdatedAt,
		&job.CompletedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Job not found
		}
		return nil, fmt.Errorf("failed to get processing job: %v", err)
	}
	
	return &job, nil
}

// LogFileAccess logs file access for analytics
func (db *Database) LogFileAccess(fileID, accessType, ipAddress, userAgent string) error {
	ctx := context.Background()
	
	query := `
		INSERT INTO file_access_logs (file_id, access_type, ip_address, user_agent)
		VALUES ($1, $2, $3, $4)
	`
	
	_, err := db.Pool.Exec(ctx, query, fileID, accessType, ipAddress, userAgent)
	if err != nil {
		// Don't fail the request if logging fails, just log the error
		log.Printf("Failed to log file access: %v", err)
		return nil
	}
	
	return nil
}