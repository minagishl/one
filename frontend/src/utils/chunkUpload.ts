import { FileMetadata } from '../types';

export interface ChunkUploadOptions {
	file: File;
	chunkSize?: number;
	downloadPassword?: string;
	onProgress?: (progress: number) => void;
	onChunkProgress?: (chunkIndex: number, totalChunks: number) => void;
	onError?: (error: string) => void;
	onRetry?: (chunkIndex: number, attempt: number) => void;
}

export interface ChunkUploadResult {
	success: boolean;
	fileId?: string;
	metadata?: FileMetadata;
	delete_password?: string;
	error?: string;
}

export interface UploadSession {
	upload_id: string;
	total_chunks: number;
	chunk_size: number;
	expires_at: string;
}

export class ChunkUploader {
	private static readonly DEFAULT_CHUNK_SIZE = 50 * 1024 * 1024; // 50MB (optimized for better progress tracking)
	private static readonly MAX_RETRIES = 3;
	private static readonly RETRY_DELAY = 2000; // 2 seconds (increased for larger chunks)

	public static async uploadFile(options: ChunkUploadOptions): Promise<ChunkUploadResult> {
		const {
			file,
			chunkSize = this.DEFAULT_CHUNK_SIZE,
			downloadPassword,
			onProgress,
			onChunkProgress,
			onError,
			onRetry,
		} = options;

		try {
			// Skip hash calculation for now to avoid memory issues with large files
			// In production, consider using a streaming hash library or server-side verification
			const fileHash = '';

			// Step 1: Initiate upload session
			const session = await this.initiateUpload(file, chunkSize, fileHash, downloadPassword);

			// Step 2: Upload chunks
			const totalChunks = session.total_chunks;
			let uploadedChunks = 0;

			for (let chunkIndex = 0; chunkIndex < totalChunks; chunkIndex++) {
				const start = chunkIndex * chunkSize;
				const end = Math.min(start + chunkSize, file.size);
				const chunk = file.slice(start, end);

				// Upload chunk with retry logic
				let success = false;
				let attempts = 0;

				while (!success && attempts < this.MAX_RETRIES) {
					try {
						if (attempts > 0) {
							onRetry?.(chunkIndex, attempts);
							await this.delay(this.RETRY_DELAY * attempts);
						}

						await this.uploadChunk(session.upload_id, chunkIndex, chunk);
						success = true;
						uploadedChunks++;

						onChunkProgress?.(chunkIndex + 1, totalChunks);
						onProgress?.(Math.round((uploadedChunks / totalChunks) * 100));

						// Add delay between chunks to avoid rate limiting
						if (chunkIndex < totalChunks - 1) {
							await this.delay(500); // 500ms delay between chunks
						}
					} catch (error) {
						attempts++;
						if (attempts >= this.MAX_RETRIES) {
							throw new Error(
								`Failed to upload chunk ${chunkIndex} after ${this.MAX_RETRIES} attempts: ${error}`
							);
						}
					}
				}
			}

			// Step 3: Complete upload
			const result = await this.completeUpload(session.upload_id);

			// If async processing, wait for completion to get delete_password
			if (result.job_id && result.file_id) {
				try {
					const fileStatus = await this.waitForProcessingCompletion(result.file_id);

					// Try multiple paths to get delete_password for robustness
					const deletePassword =
						fileStatus.metadata?.delete_password ||
						fileStatus.delete_password ||
						result.delete_password ||
						result.metadata?.delete_password;

					return {
						success: true,
						fileId: result.file_id,
						metadata: fileStatus.metadata || result.metadata,
						delete_password: deletePassword,
					};
				} catch {
					// Fallback to immediate result if waiting fails
					const deletePassword = result.delete_password || result.metadata?.delete_password;
					return {
						success: true,
						fileId: result.file_id,
						metadata: result.metadata,
						delete_password: deletePassword,
					};
				}
			}

			// Handle immediate response (non-async)
			const deletePassword = result.metadata?.delete_password || result.delete_password;

			return {
				success: true,
				fileId: result.file_id,
				metadata: result.metadata,
				delete_password: deletePassword,
			};
		} catch (error) {
			const errorMessage = error instanceof Error ? error.message : 'Unknown error occurred';
			onError?.(errorMessage);
			return {
				success: false,
				error: errorMessage,
			};
		}
	}

	private static async initiateUpload(
		file: File,
		chunkSize: number,
		fileHash: string,
		downloadPassword?: string
	): Promise<UploadSession> {
		const response = await fetch('/api/chunk/initiate', {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
			},
			body: JSON.stringify({
				filename: file.name,
				total_size: file.size,
				chunk_size: chunkSize,
				file_hash: fileHash,
				download_password: downloadPassword,
			}),
		});

		if (!response.ok) {
			const error = await response.json();
			throw new Error(error.error || 'Failed to initiate upload');
		}

		return await response.json();
	}

	private static async uploadChunk(
		uploadId: string,
		chunkIndex: number,
		chunk: Blob
	): Promise<void> {
		const formData = new FormData();
		formData.append('chunk', chunk);

		const response = await fetch(`/api/chunk/${uploadId}/${chunkIndex}`, {
			method: 'POST',
			body: formData,
		});

		if (!response.ok) {
			const error = await response.json();
			throw new Error(error.error || 'Failed to upload chunk');
		}
	}

	private static async completeUpload(uploadId: string): Promise<any> {
		const response = await fetch(`/api/chunk/${uploadId}/complete`, {
			method: 'POST',
		});

		if (!response.ok) {
			const error = await response.json();
			throw new Error(error.error || 'Failed to complete upload');
		}

		const result = await response.json();

		// Handle async processing - return original result with job_id
		if (result.job_id && result.file_id) {
			// Return original result for async processing detection
			return result;
		}

		// Handle legacy synchronous response
		return result;
	}

	private static delay(ms: number): Promise<void> {
		return new Promise((resolve) => setTimeout(resolve, ms));
	}

	// Check if a file is ready for download or still processing
	public static async getFileStatus(fileId: string): Promise<{
		status: 'ready' | 'processing' | 'not_found' | 'error';
		message: string;
		download_url?: string;
		preview_url?: string;
		metadata?: any;
		filename?: string;
		delete_password?: string;
	}> {
		const response = await fetch(`/api/file/${fileId}/status`);

		if (!response.ok) {
			return {
				status: 'error',
				message: 'Failed to check file status',
			};
		}

		return await response.json();
	}

	// Wait for file processing to complete and return final metadata with delete_password
	private static async waitForProcessingCompletion(fileId: string): Promise<{
		status: 'ready' | 'processing' | 'not_found' | 'error';
		message: string;
		metadata?: any;
		delete_password?: string;
	}> {
		const maxAttempts = 30; // 30 attempts with 2s delay = 1 minute max wait
		const delayMs = 2000; // 2 seconds

		for (let attempt = 0; attempt < maxAttempts; attempt++) {
			const status = await this.getFileStatus(fileId);

			if (status.status === 'ready') {
				return status;
			}

			if (status.status === 'error' || status.status === 'not_found') {
				throw new Error(`File processing failed: ${status.message}`);
			}

			// Wait before next attempt
			await this.delay(delayMs);
		}

		throw new Error('File processing timeout - please check file status manually');
	}
}

// Helper function to determine if file should use chunk upload
export function shouldUseChunkUpload(file: File, threshold: number = 100 * 1024 * 1024): boolean {
	return file.size > threshold; // Default: 100MB threshold (optimized for fewer requests)
}

// Helper function to format upload progress
export function formatUploadProgress(
	chunkIndex: number,
	totalChunks: number,
	percentage: number
): string {
	return `Uploading chunk ${chunkIndex}/${totalChunks} (${percentage}%)`;
}
