-- PostgreSQL schema for file sharing application
-- This schema provides persistent storage for file metadata and upload sessions

-- Enable necessary extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Files table: Store file metadata and content
CREATE TABLE files (
    id VARCHAR(36) PRIMARY KEY,  -- File ID (generated by generateFileID())
    filename TEXT NOT NULL,
    original_size BIGINT NOT NULL,
    compressed_size BIGINT,
    mime_type VARCHAR(255) NOT NULL,
    compression_type VARCHAR(20) DEFAULT 'none',
    storage_type VARCHAR(20) NOT NULL DEFAULT 'postgresql', -- 'postgresql', 'disk' (for very large files)
    storage_path TEXT, -- Path for disk-stored files (only for files > 1GB)
    file_content BYTEA, -- Store compressed file content directly in PostgreSQL
    upload_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    delete_password VARCHAR(255) NOT NULL,
    download_password VARCHAR(255),
    has_download_password BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Chunk uploads table: Track chunked upload sessions
CREATE TABLE chunk_uploads (
    upload_id VARCHAR(36) PRIMARY KEY,
    filename TEXT NOT NULL,
    total_size BIGINT NOT NULL,
    total_chunks INTEGER NOT NULL,
    chunk_size BIGINT NOT NULL,
    received_chunks JSONB NOT NULL DEFAULT '[]', -- Array of boolean values
    file_hash VARCHAR(64), -- SHA-256 hash
    download_password VARCHAR(255),
    has_download_password BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_activity TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active' -- 'active', 'completed', 'failed', 'expired'
);

-- Processing jobs table: Track background file processing jobs
CREATE TABLE processing_jobs (
    job_id VARCHAR(36) PRIMARY KEY,
    upload_id VARCHAR(36) REFERENCES chunk_uploads(upload_id) ON DELETE CASCADE,
    file_id VARCHAR(36), -- Will be set when file is created
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'failed'
    progress INTEGER NOT NULL DEFAULT 0, -- 0-100
    error_message TEXT,
    result_data JSONB, -- Store FileResult as JSON
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

-- File access logs table: Track file downloads and access (optional, for analytics)
CREATE TABLE file_access_logs (
    id SERIAL PRIMARY KEY,
    file_id VARCHAR(36) REFERENCES files(id) ON DELETE CASCADE,
    access_type VARCHAR(20) NOT NULL, -- 'download', 'preview', 'stream'
    ip_address INET,
    user_agent TEXT,
    access_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers to automatically update updated_at
CREATE TRIGGER update_files_updated_at 
    BEFORE UPDATE ON files 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_chunk_uploads_updated_at 
    BEFORE UPDATE ON chunk_uploads 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_processing_jobs_updated_at 
    BEFORE UPDATE ON processing_jobs 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to cleanup expired files and uploads
CREATE OR REPLACE FUNCTION cleanup_expired_data()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER := 0;
BEGIN
    -- Delete expired files
    DELETE FROM files WHERE expires_at < NOW();
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    
    -- Delete expired chunk uploads
    DELETE FROM chunk_uploads WHERE expires_at < NOW();
    
    -- Delete old processing jobs (keep for 7 days)
    DELETE FROM processing_jobs WHERE created_at < NOW() - INTERVAL '7 days';
    
    -- Delete old access logs (keep for 30 days)
    DELETE FROM file_access_logs WHERE access_time < NOW() - INTERVAL '30 days';
    
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Create initial admin user (optional)
-- You can modify this or remove if not needed
-- INSERT INTO users (username, password_hash, role) VALUES ('admin', 'changeme', 'admin');

-- Create views for common queries
CREATE VIEW active_files AS
SELECT 
    id,
    filename,
    original_size,
    mime_type,
    storage_type,
    upload_time,
    expires_at,
    has_download_password,
    (expires_at > NOW()) AS is_active
FROM files 
WHERE expires_at > NOW()
ORDER BY upload_time DESC;

CREATE VIEW upload_statistics AS
SELECT 
    DATE_TRUNC('day', upload_time) AS upload_date,
    COUNT(*) AS files_uploaded,
    SUM(original_size) AS total_size,
    AVG(original_size) AS avg_size,
    COUNT(CASE WHEN storage_type = 'disk' THEN 1 END) AS large_files,
    COUNT(CASE WHEN has_download_password THEN 1 END) AS protected_files
FROM files 
GROUP BY DATE_TRUNC('day', upload_time)
ORDER BY upload_date DESC;

-- Indexes for better performance
CREATE INDEX files_expires_at_idx ON files (expires_at);
CREATE INDEX files_upload_time_idx ON files (upload_time);
CREATE INDEX files_storage_type_idx ON files (storage_type);
CREATE INDEX files_filename_idx ON files (filename);

CREATE INDEX chunk_uploads_expires_at_idx ON chunk_uploads (expires_at);
CREATE INDEX chunk_uploads_last_activity_idx ON chunk_uploads (last_activity);
CREATE INDEX chunk_uploads_status_idx ON chunk_uploads (status);

CREATE INDEX processing_jobs_status_idx ON processing_jobs (status);
CREATE INDEX processing_jobs_created_at_idx ON processing_jobs (created_at);
CREATE INDEX processing_jobs_file_id_idx ON processing_jobs (file_id);

CREATE INDEX file_access_logs_file_id_idx ON file_access_logs (file_id);
CREATE INDEX file_access_logs_access_time_idx ON file_access_logs (access_time);
CREATE INDEX file_access_logs_access_type_idx ON file_access_logs (access_type);

CREATE INDEX files_filename_trgm ON files USING gin (filename gin_trgm_ops);
CREATE INDEX files_composite_lookup ON files (id, expires_at);
CREATE INDEX chunk_uploads_active ON chunk_uploads (upload_id, status) WHERE status = 'active';

-- Comments for documentation
COMMENT ON TABLE files IS 'Stores metadata for uploaded files with expiration and storage information';
COMMENT ON TABLE chunk_uploads IS 'Tracks chunked upload sessions for large files';
COMMENT ON TABLE processing_jobs IS 'Manages background processing jobs for file assembly and compression';
COMMENT ON TABLE file_access_logs IS 'Optional logging table for file access analytics';

COMMENT ON COLUMN files.storage_type IS 'Indicates where file content is stored: postgresql (default), disk (for files > 1GB)';
COMMENT ON COLUMN files.storage_path IS 'File system path for disk-stored files (only for very large files > 1GB)';
COMMENT ON COLUMN files.file_content IS 'Compressed file content stored as BYTEA (NULL for disk-stored files)';
COMMENT ON COLUMN chunk_uploads.received_chunks IS 'JSONB array tracking which chunks have been received';
COMMENT ON COLUMN processing_jobs.result_data IS 'JSON object containing FileResult data upon completion';