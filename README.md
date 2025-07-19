# ONE

![Preview](./images/preview.png)

ONE is a fast and ultra-lightweight file storage service that enables you to securely upload, store, and preview files in your browser. With advanced compression, deduplication, and easy sharing features, ONE makes file management simple and efficient.

## Features

- **Large File Support**: Upload files up to 10GB using advanced chunked upload technology
- **Advanced Multi-Algorithm Compression**: Automatic selection of optimal compression (Brotli, Zstandard, LZ4, Gzip) based on file type and size
- **Real-time Progress Tracking**: Visual progress bars with detailed chunk upload information
- **Error Recovery**: Automatic retry mechanism for failed chunk uploads with exponential backoff
- **Memory Efficient**: Large files are processed in chunks and stored on disk to prevent memory exhaustion
- **Browser-Based Preview**: View images, videos, audio, text files, PDFs, and more directly in your browser
- **ZIP Archive Browsing**: Browse and extract files from ZIP archives with encoding support
- **UUID-based File Management**: Secure file access with unique UUIDs
- **Password Protection**: Optional download passwords for enhanced security
- **24-Hour Auto-Delete**: Files automatically expire after 24 hours
- **Real-time Countdown**: Live countdown timer showing time until expiration
- **Streaming I/O**: Minimal memory usage for uploads and downloads
- **Docker Ready**: Single-command deployment with Docker Compose
- **Ultra-Lightweight**: Go-based backend with minimal resource footprint

## Architecture

### Hybrid Storage System

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web Browser   │◄──►│   Go Backend    │◄──►│   Redis Cache   │
│                 │    │                 │    │                 │
│ • Upload UI     │    │ • Compression   │    │ • Sessions      │
│ • Preview       │    │ • ZIP Browsing  │    │ • Processing    │
│ • ZIP Preview   │    │ • Password Auth │    │ • Temp Data     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   PostgreSQL    │    │   File System   │
                       │                 │    │                 │
                       │ • File Metadata │    │ • Large Files   │
                       │ • Upload History│    │ • Media Content │
                       │ • Job Tracking  │    │ • ZIP Archives  │
                       └─────────────────┘    └─────────────────┘
```

### Storage Strategy

- **Redis**: High-speed cache for sessions, processing status, and temporary data
- **PostgreSQL**: Persistent metadata storage and file tracking
- **File System**: Large files (>100MB) stored directly on disk
- **Hybrid Mode**: Automatic selection based on file size and type

## Quick Start

### Prerequisites

- Docker and Docker Compose installed
- 8080 port available

### 1. Clone and Deploy

```bash
git clone <repository-url>
cd one
docker-compose up -d
```

### 2. Access ONE

Open your browser and go to: `http://localhost:8080`

That's it!

## Environment Configuration

Edit the `compose.yml` file to configure the service:

```yaml
environment:
  # Server Configuration
  - PORT=8080 # Service port
  
  # Redis Configuration (Cache & Sessions)
  - REDIS_ADDR=redis:6379 # Redis connection
  
  # PostgreSQL Configuration (Metadata & Persistence)
  - DATABASE_URL=postgres://user:password@postgres:5432/filedb
  - DB_HOST=postgres
  - DB_PORT=5432
  - DB_USER=user
  - DB_PASSWORD=password
  - DB_NAME=filedb
  - DB_SSLMODE=disable
  - DB_MAX_CONNS=20
  - DB_MIN_CONNS=5
  
  # File Storage Configuration
  - MAX_FILE_SIZE=10737418240 # Maximum file size (10GB)
  - CHUNK_SIZE=104857600 # Chunk size for large files (100MB - optimized for fewer requests)
  - MAX_CHUNKS_PER_FILE=100 # Maximum chunks per file (100 chunks = 10GB)
  - TEMP_DIR=./temp # Directory for temporary chunk storage
  - CHUNK_TIMEOUT=30m # Timeout for chunk upload sessions (increased for larger chunks)
```

## Security Features

### UUID-based File Access

- Files are accessed via unique UUIDs instead of predictable IDs
- Only users with the exact UUID can access the file
- No authentication required - security through obscurity

### Auto-Expiration

- All files automatically expire after 24 hours
- Real-time countdown shows remaining time
- Expired files are automatically cleaned up every 5 minutes

### Security Best Practices

- Files are only accessible with the exact UUID
- Consider running behind a reverse proxy with HTTPS
- Regular security updates via Docker image rebuilds
- Monitor file access patterns if needed

## Compression Technology

### Intelligent Algorithm Selection

The service automatically selects the optimal compression algorithm based on:

| File Type                          | Size        | Algorithm | Reason                     |
| ---------------------------------- | ----------- | --------- | -------------------------- |
| Already compressed (jpg, mp4, zip) | Any         | None      | Avoid double compression   |
| Text/Code                          | < 10KB      | LZ4       | Speed over compression     |
| Documents                          | 10KB - 10MB | Zstandard | Balanced speed/compression |
| Large files                        | > 10MB      | Gzip      | Maximum compression        |

### Compression Performance

- **Zstandard**: 40-60% compression ratio, 3x faster than Gzip
- **LZ4**: 30-40% compression ratio, 10x faster than Gzip
- **Gzip**: 50-70% compression ratio, industry standard
- **Brotli**: 60-80% compression ratio, best for text

## API Documentation

### Upload File

```bash
curl -X POST -F "file=@example.txt" http://localhost:8080/api/upload
```

With download password:

```bash
curl -X POST -F "file=@example.txt" -F "download_password=mypassword" http://localhost:8080/api/upload
```

Response includes file_id and delete_password for file management.

### Large File Upload (Chunked)

For files larger than 50MB, the system automatically uses chunked upload:

1. **Initiate chunked upload:**

```bash
curl -X POST http://localhost:8080/api/chunk/initiate \
  -H "Content-Type: application/json" \
  -d '{
    "filename": "largefile.mp4",
    "total_size": 1073741824,
    "chunk_size": 10485760,
    "download_password": "optional_password"
  }'
```

2. **Upload chunks:**

```bash
curl -X POST http://localhost:8080/api/chunk/{upload_id}/0 \
  -F "chunk=@chunk_0.bin"
```

3. **Complete upload:**

```bash
curl -X POST http://localhost:8080/api/chunk/{upload_id}/complete
```

4. **Check upload status:**

```bash
curl http://localhost:8080/api/chunk/{upload_id}/status
```

### Get File Metadata

```bash
curl http://localhost:8080/api/metadata/{file_id}
```

Returns file information without the actual content.

### Download File

```bash
curl http://localhost:8080/api/file/{file_id} -o downloaded_file
```

With password (if required):

```bash
curl http://localhost:8080/api/file/{file_id}?password=mypassword -o downloaded_file
```

### Preview File

```bash
curl http://localhost:8080/api/preview/{file_id}
```

With password (if required):

```bash
curl http://localhost:8080/api/preview/{file_id}?password=mypassword
```

Returns file content for browser preview (images, videos, text, PDFs, etc.).

### Delete File

```bash
curl -X DELETE "http://localhost:8080/api/file/{file_id}?delete_password=your_delete_password"
```

Requires the delete_password returned during file upload.

### Browse ZIP Archive Contents

```bash
curl http://localhost:8080/api/zip/{file_id}
```

Returns a list of files contained within a ZIP archive, including file names, sizes, and modification dates.

### Extract File from ZIP Archive

```bash
curl "http://localhost:8080/api/zip/{file_id}/extract?filename=path/to/file.txt"
```

Extracts and previews a specific file from within a ZIP archive. Supports the same preview capabilities as regular files.

## Admin Features

### Update File Expiration

Administrators can extend or modify file expiration times using a secure API endpoint.

**Setup:**

First, set the admin password as an environment variable:

```bash
export ADMIN_PASSWORD="your_secure_admin_password"
```

Or add it to your `compose.yml`:

```yaml
environment:
  - ADMIN_PASSWORD=your_secure_admin_password
```

**API Usage:**

### Update File Expiration
```bash
curl -X PUT "http://localhost:8080/api/admin/file/{file_id}/expires" \
  -H "Content-Type: application/json" \
  -d '{
    "admin_password": "your_secure_admin_password",
    "expires_at": "2025-07-20T23:59:59Z"
  }'
```

**Request Parameters:**
- `admin_password`: Admin password (set via ADMIN_PASSWORD environment variable)
- `expires_at`: New expiration time in RFC3339 format (must be in the future)

### Delete File
```bash
curl -X DELETE "http://localhost:8080/api/admin/file/{file_id}" \
  -H "Content-Type: application/json" \
  -d '{
    "admin_password": "your_secure_admin_password"
  }'
```

**Request Parameters:**
- `admin_password`: Admin password (set via ADMIN_PASSWORD environment variable)

### Get File List
```bash
curl -X GET "http://localhost:8080/api/admin/files" \
  -H "Content-Type: application/json" \
  -d '{
    "admin_password": "your_secure_admin_password"
  }'
```

**Request Parameters:**
- `admin_password`: Admin password (set via ADMIN_PASSWORD environment variable)

**Response:**
```json
{
  "message": "File expiration updated successfully",
  "file_id": "8e0e8842-2aac-456c-b83a-192321b1e6ae",
  "old_expires_at": "2025-07-19T12:00:00Z",
  "new_expires_at": "2025-07-20T23:59:59Z",
  "metadata": {
    "id": "8e0e8842-2aac-456c-b83a-192321b1e6ae",
    "filename": "example.txt",
    "size": 1024,
    "expires_at": "2025-07-20T23:59:59Z"
  }
}
```

**Security Features:**
- Requires admin password authentication
- Admin password must be set via environment variable
- Only accepts future expiration times
- Returns error if admin functionality is not configured

### File Access with UUID

```bash
# Access file with UUID (returned from upload)
curl http://localhost:8080/api/file/8e0e8842-2aac-456c-b83a-192321b1e6ae

# Preview file with UUID
curl http://localhost:8080/api/preview/8e0e8842-2aac-456c-b83a-192321b1e6ae

# Get metadata for file with UUID
curl http://localhost:8080/api/metadata/8e0e8842-2aac-456c-b83a-192321b1e6ae
```

## Performance Characteristics

### Resource Usage

- **Memory**: < 50MB base usage
- **CPU**: Minimal, scales with compression workload
- **Storage**: Hybrid Redis + PostgreSQL + File System
- **Network**: Streaming I/O for efficient transfers

### Scalability

- **Horizontal**: Deploy multiple instances behind load balancer
- **Vertical**: Scale PostgreSQL and Redis independently
- **Storage**: PostgreSQL clustering, Redis clustering, distributed file storage
- **Performance**: Automatic storage tier selection based on file size

### Improved Reliability

- **Persistent Metadata**: PostgreSQL ensures file metadata survives restarts
- **Large File Support**: Direct disk storage for files > 100MB
- **Better Error Handling**: Enhanced status tracking and error recovery
- **Consistent State**: Hybrid storage prevents "File not found" errors

## Advanced Configuration

### Redis Configuration

Customize Redis settings in `compose.yml`:

```yaml
redis:
  command: redis-server --save 20 1 --loglevel warning --maxmemory 1gb --maxmemory-policy allkeys-lru
```

### Health Checks

Built-in health checks ensure service reliability:

- Application health endpoint: `GET /health`
- Redis connectivity checks at startup
- Automatic restart on failure (via Docker)

### Monitoring

Basic monitoring capabilities:

- Health check endpoint returns service status
- Application logs provide request and error information
- Docker Compose logs for debugging

## Development

### Local Development

```bash
# Install dependencies
go mod tidy

# Run locally
go run .

# With Redis
docker run -d -p 6379:6379 redis:7-alpine
```

### Building

```bash
# Build binary
go build -o file-storage-service

# Build Docker image
docker build -t file-storage-service .
```

## Troubleshooting

### Common Issues

**Service won't start:**

```bash
# Check logs
docker-compose logs app

# Verify Redis connection
docker-compose logs redis
```

**Files not uploading:**

- Check available disk space
- Verify Redis is running
- Check file permissions

**Preview not working:**

- Ensure file type is supported
- Check browser console for errors
- Verify file isn't corrupted

### Support

For issues and feature requests, please check the logs and error messages. The service provides detailed error information to help diagnose problems.

## License

This project is open source and available under the [MIT License](LICENSE).
