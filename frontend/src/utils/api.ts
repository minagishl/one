import { FileMetadata, UploadResult, ZipContents } from '../types';

export const uploadFile = async (file: File, downloadPassword?: string): Promise<UploadResult> => {
	const formData = new FormData();
	formData.append('file', file);
	if (downloadPassword) {
		formData.append('download_password', downloadPassword);
	}

	try {
		const response = await fetch('/api/upload', {
			method: 'POST',
			body: formData,
		});

		const result = await response.json();

		if (response.ok) {
			return {
				success: true,
				filename: file.name,
				fileId: result.file_id,
				metadata: result.metadata,
				delete_password: result.metadata.delete_password,
			};
		} else {
			return {
				success: false,
				filename: file.name,
				error: result.error || 'Upload failed',
			};
		}
	} catch {
		return {
			success: false,
			filename: file.name,
			error: 'Network error',
		};
	}
};

export const downloadFile = async (
	fileId: string,
	filename: string,
	password?: string
): Promise<void> => {
	try {
		const url = new URL(`/api/file/${fileId}`, window.location.origin);
		if (password) {
			url.searchParams.append('password', password);
		}

		const response = await fetch(url.toString());
		if (!response.ok) {
			if (response.status === 401) {
				throw new Error('Password required');
			}
			throw new Error('Download failed');
		}

		const blob = await response.blob();
		const downloadUrl = URL.createObjectURL(blob);

		const link = document.createElement('a');
		link.href = downloadUrl;
		link.download = filename;
		link.click();

		// Clean up the object URL
		URL.revokeObjectURL(downloadUrl);
	} catch (error) {
		console.error('Download error:', error);
		throw error;
	}
};

export const deleteFile = async (
	fileId: string,
	deletePassword: string
): Promise<{ success: boolean; error?: string }> => {
	try {
		const url = new URL(`/api/file/${fileId}`, window.location.origin);
		url.searchParams.append('delete_password', deletePassword);

		const response = await fetch(url.toString(), { method: 'DELETE' });
		const data = await response.json();

		if (response.ok) {
			return { success: true };
		} else {
			return { success: false, error: data.error || 'Delete failed' };
		}
	} catch {
		return { success: false, error: 'Network error' };
	}
};

export const getFileMetadata = async (fileId: string): Promise<FileMetadata> => {
	const response = await fetch(`/api/metadata/${fileId}`);
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'File not found or expired');
	}

	return await response.json();
};

export const getFilePreview = async (
	fileId: string,
	password?: string
): Promise<{ blob: Blob; contentType: string }> => {
	const url = new URL(`/api/preview/${fileId}`, window.location.origin);
	if (password) {
		url.searchParams.append('password', password);
	}

	const response = await fetch(url.toString());
	if (!response.ok) {
		if (response.status === 401) {
			throw new Error('Password required');
		}
		throw new Error('File not found or expired');
	}

	const blob = await response.blob();
	const contentType = response.headers.get('content-type') || 'application/octet-stream';

	return { blob, contentType };
};

export const getFilePreviewWithProgress = async (
	fileId: string,
	onProgress: (progress: number) => void,
	password?: string
): Promise<{ blob: Blob; contentType: string }> => {
	const url = new URL(`/api/preview/${fileId}`, window.location.origin);
	if (password) {
		url.searchParams.append('password', password);
	}

	const response = await fetch(url.toString());
	if (!response.ok) {
		if (response.status === 401) {
			throw new Error('Password required');
		}
		throw new Error('File not found or expired');
	}

	const contentLength = response.headers.get('content-length');
	const total = contentLength ? parseInt(contentLength, 10) : 0;
	const contentType = response.headers.get('content-type') || 'application/octet-stream';

	if (!response.body) {
		throw new Error('Response body is null');
	}

	const reader = response.body.getReader();
	const chunks: Uint8Array[] = [];
	let receivedLength = 0;

	// Read stream with progress tracking
	while (true) {
		const { done, value } = await reader.read();

		if (done) break;

		chunks.push(value);
		receivedLength += value.length;

		if (total > 0) {
			const progress = Math.round((receivedLength / total) * 100);
			onProgress(progress);
		}
	}

	// Combine chunks into single Uint8Array
	const combinedChunks = new Uint8Array(receivedLength);
	let position = 0;
	for (const chunk of chunks) {
		combinedChunks.set(chunk, position);
		position += chunk.length;
	}

	const blob = new Blob([combinedChunks], { type: contentType });

	return { blob, contentType };
};

export const getZipContents = async (fileId: string): Promise<ZipContents> => {
	const response = await fetch(`/api/zip/${fileId}`);
	const data = await response.json();

	if (!response.ok) {
		throw new Error(data.error || 'Failed to load ZIP contents');
	}

	return data;
};

export const getZipFilePreview = async (
	fileId: string,
	fileName: string
): Promise<{ blob: Blob; contentType: string }> => {
	const url = new URL(`/api/zip/${fileId}/extract`, window.location.origin);
	url.searchParams.append('filename', fileName);
	
	const response = await fetch(url.toString());
	
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to load file from ZIP');
	}

	const blob = await response.blob();
	const contentType = response.headers.get('content-type') || 'application/octet-stream';

	return { blob, contentType };
};
