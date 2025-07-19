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

			return {
				success: true,
				fileId: result.file_id,
				metadata: result.metadata,
				delete_password: result.metadata?.delete_password,
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

	private static async calculateFileHash(file: File): Promise<string> {
		try {
			// For large files, we'll calculate hash in chunks to avoid memory issues
			const chunkSize = 1024 * 1024; // 1MB chunks for hashing
			const chunks = Math.ceil(file.size / chunkSize);

			// Create a streaming hash using crypto.subtle
			const hashBuffer = new ArrayBuffer(0);
			let hash = await crypto.subtle.digest('SHA-256', hashBuffer); // Initialize empty hash

			// Process file in chunks
			for (let i = 0; i < chunks; i++) {
				const start = i * chunkSize;
				const end = Math.min(start + chunkSize, file.size);
				const chunk = file.slice(start, end);

				// Read chunk as ArrayBuffer
				const chunkBuffer = await this.readChunkAsArrayBuffer(chunk);

				// Update hash with chunk data
				// Note: crypto.subtle doesn't support streaming, so we'll concatenate and hash at the end
				if (i === 0) {
					hash = await crypto.subtle.digest('SHA-256', chunkBuffer);
				} else {
					// For streaming hash, we need to combine previous hash with new chunk
					// This is a simplified approach - in production, use a proper streaming hash library
					const combined = new Uint8Array(hash.byteLength + chunkBuffer.byteLength);
					combined.set(new Uint8Array(hash));
					combined.set(new Uint8Array(chunkBuffer), hash.byteLength);
					hash = await crypto.subtle.digest('SHA-256', combined);
				}
			}

			const hashArray = Array.from(new Uint8Array(hash));
			const hashHex = hashArray.map((b) => b.toString(16).padStart(2, '0')).join('');
			return hashHex;
		} catch (error) {
			console.error('Error calculating file hash:', error);
			throw new Error(
				'Failed to calculate file hash: ' +
					(error instanceof Error ? error.message : 'Unknown error')
			);
		}
	}

	private static async readChunkAsArrayBuffer(chunk: Blob): Promise<ArrayBuffer> {
		return new Promise((resolve, reject) => {
			const reader = new FileReader();
			reader.onload = (e) => {
				if (e.target?.result instanceof ArrayBuffer) {
					resolve(e.target.result);
				} else {
					reject(new Error('Failed to read chunk as ArrayBuffer'));
				}
			};
			reader.onerror = () => reject(new Error('Failed to read chunk'));
			reader.readAsArrayBuffer(chunk);
		});
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
		
		// Handle async processing
		if (result.job_id) {
			return await this.pollJobStatus(result.job_id);
		}
		
		// Handle legacy synchronous response
		return result;
	}

	private static async pollJobStatus(jobId: string): Promise<any> {
		while (true) {
			const response = await fetch(`/api/job/${jobId}/status`);
			
			if (!response.ok) {
				throw new Error('Failed to check job status');
			}
			
			const job = await response.json();
			
			switch (job.status) {
				case 'completed':
					return {
						file_id: job.result.file_id,
						metadata: {
							delete_password: job.result.delete_password || null,
							filename: job.result.filename,
							size: job.result.size,
						}
					};
				case 'failed':
					throw new Error(job.error || 'File processing failed');
				case 'processing':
				case 'pending':
					// Wait 2 seconds before polling again
					await this.delay(2000);
					break;
				default:
					throw new Error(`Unknown job status: ${job.status}`);
			}
		}
	}

	private static delay(ms: number): Promise<void> {
		return new Promise((resolve) => setTimeout(resolve, ms));
	}

	public static async getUploadStatus(uploadId: string): Promise<any> {
		const response = await fetch(`/api/chunk/${uploadId}/status`);

		if (!response.ok) {
			const error = await response.json();
			throw new Error(error.error || 'Failed to get upload status');
		}

		return await response.json();
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
