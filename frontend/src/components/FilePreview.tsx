import React, { useState, useEffect } from 'react';
import { AlertTriangle, File, Folder, X, ChevronLeft, ChevronRight } from 'lucide-react';
import { FileMetadata, ZipContents, ZipFile } from '../types';
import {
	getFilePreview,
	getFilePreviewWithProgress,
	getZipContents,
	getZipFilePreview,
} from '../utils/api';
import { formatSize, formatDate } from '../utils/format';

interface FilePreviewProps {
	fileId: string;
	metadata: FileMetadata;
	password?: string;
	adminToken?: string;
}

const FilePreview: React.FC<FilePreviewProps> = ({ fileId, metadata, password, adminToken }) => {
	const [previewUrl, setPreviewUrl] = useState<string>('');
	const [zipContents, setZipContents] = useState<ZipContents | null>(null);
	const [activeTab, setActiveTab] = useState<'preview' | 'zip'>('preview');
	const [error, setError] = useState<string>('');
	const [zipFilePreview, setZipFilePreview] = useState<{
		file: ZipFile;
		previewUrl: string;
		contentType: string;
		currentIndex: number;
	} | null>(null);
	const [isZipFilePreviewOpen, setIsZipFilePreviewOpen] = useState(false);
	const [isLoading, setIsLoading] = useState(false);
	const [loadingProgress, setLoadingProgress] = useState(0);

	useEffect(() => {
		loadPreview();
		if (metadata.filename.toLowerCase().endsWith('.zip')) {
			loadZipContents();
		}

		return () => {
			if (previewUrl) {
				URL.revokeObjectURL(previewUrl);
			}
			if (zipFilePreview?.previewUrl) {
				URL.revokeObjectURL(zipFilePreview.previewUrl);
			}
		};
	}, [fileId, metadata, password]);

	useEffect(() => {
		const handleKeyDown = (event: KeyboardEvent) => {
			if (!isZipFilePreviewOpen || !zipFilePreview) return;

			switch (event.key) {
				case 'ArrowLeft':
					event.preventDefault();
					navigateToFile('prev');
					break;
				case 'ArrowRight':
					event.preventDefault();
					navigateToFile('next');
					break;
				case 'Escape':
					event.preventDefault();
					closeZipFilePreview();
					break;
			}
		};

		if (isZipFilePreviewOpen) {
			document.addEventListener('keydown', handleKeyDown);
		}

		return () => {
			document.removeEventListener('keydown', handleKeyDown);
		};
	}, [isZipFilePreviewOpen, zipFilePreview]);

	const loadPreview = async () => {
		try {
			if (previewUrl) {
				URL.revokeObjectURL(previewUrl);
			}

			setIsLoading(true);
			setLoadingProgress(0);
			setError('');

			// Check if file is large (>50MB) and show special handling
			const isLargeFile = metadata.size > 50 * 1024 * 1024;
			const isMediaFile =
				metadata.mime_type.startsWith('video/') || metadata.mime_type.startsWith('audio/');

			// For large media files, handle password-protected files differently
			if (isMediaFile && metadata.size > 5 * 1024 * 1024) {
				if (metadata.has_download_password && !adminToken) {
					// For password-protected media without admin access, use blob approach to ensure authentication
					const { blob } = await getFilePreviewWithProgress(
						fileId,
						(progress) => {
							setLoadingProgress(progress);
						},
						password,
						adminToken
					);
					const url = URL.createObjectURL(blob);
					setPreviewUrl(url);
				} else {
					// For non-protected media, use direct streaming URL
					// For admin access, use blob approach for security
					if (adminToken) {
						console.log('Admin token detected, using blob approach for streaming');
						const { blob } = await getFilePreviewWithProgress(
							fileId,
							(progress) => {
								setLoadingProgress(progress);
							},
							password,
							adminToken
						);
						const url = URL.createObjectURL(blob);
						setPreviewUrl(url);
					} else {
						let streamUrl = `/api/stream/${fileId}`;
						const params = new URLSearchParams();
						if (password) params.append('password', password);
						if (params.toString()) streamUrl += `?${params.toString()}`;
						console.log('Using streaming URL:', streamUrl);
						setPreviewUrl(streamUrl);
					}
				}
			} else if (isLargeFile) {
				// For large non-media files, show progress and optimize loading
				const { blob } = await getFilePreviewWithProgress(
					fileId,
					(progress) => {
						setLoadingProgress(progress);
					},
					password,
					adminToken
				);
				const url = URL.createObjectURL(blob);
				setPreviewUrl(url);
			} else {
				// For smaller files, use normal loading
				const { blob } = await getFilePreview(fileId, password, adminToken);
				const url = URL.createObjectURL(blob);
				setPreviewUrl(url);
			}
		} catch (err: any) {
			console.error('Error loading preview:', err);
			if (err.message && err.message.includes('Password required')) {
				setError('Password required. Please enter the password to view the preview.');
			} else if (err.message && err.message.includes('415')) {
				setError(
					'This file type cannot be previewed in the browser. Please download the file to view it.'
				);
			} else if (err.message && err.message.includes('404')) {
				setError('File not found or expired');
			} else {
				setError('Failed to load preview');
			}
		} finally {
			setIsLoading(false);
		}
	};

	const loadZipContents = async () => {
		try {
			const contents = await getZipContents(fileId);
			setZipContents(contents);
		} catch (err) {
			console.error('Failed to load ZIP contents:', err);
		}
	};

	const getPreviewableFiles = () => {
		if (!zipContents) return [];
		return zipContents.files.filter((file) => !file.is_dir);
	};

	const handleZipFileDoubleClick = async (file: ZipFile) => {
		if (file.is_dir) return;

		const previewableFiles = getPreviewableFiles();
		const currentIndex = previewableFiles.findIndex((f) => f.name === file.name);

		try {
			const { blob, contentType } = await getZipFilePreview(fileId, file.name);

			if (zipFilePreview?.previewUrl) {
				URL.revokeObjectURL(zipFilePreview.previewUrl);
			}

			const previewUrl = URL.createObjectURL(blob);
			setZipFilePreview({
				file,
				previewUrl,
				contentType,
				currentIndex,
			});
			setIsZipFilePreviewOpen(true);
		} catch (err: any) {
			console.error('Failed to load file from ZIP:', err);
			alert(`Failed to preview file: ${err.message}`);
		}
	};

	const closeZipFilePreview = () => {
		if (zipFilePreview?.previewUrl) {
			URL.revokeObjectURL(zipFilePreview.previewUrl);
		}
		setZipFilePreview(null);
		setIsZipFilePreviewOpen(false);
	};

	const navigateToFile = async (direction: 'next' | 'prev') => {
		if (!zipFilePreview || !zipContents) return;

		const previewableFiles = getPreviewableFiles();
		const currentIndex = zipFilePreview.currentIndex;
		let nextIndex;

		if (direction === 'next') {
			nextIndex = (currentIndex + 1) % previewableFiles.length;
		} else {
			nextIndex = currentIndex === 0 ? previewableFiles.length - 1 : currentIndex - 1;
		}

		const nextFile = previewableFiles[nextIndex];
		if (!nextFile) return;

		try {
			const { blob, contentType } = await getZipFilePreview(fileId, nextFile.name);

			URL.revokeObjectURL(zipFilePreview.previewUrl);

			const previewUrl = URL.createObjectURL(blob);
			setZipFilePreview({
				file: nextFile,
				previewUrl,
				contentType,
				currentIndex: nextIndex,
			});
		} catch (err: any) {
			console.error('Failed to load file from ZIP:', err);
			alert(`Failed to preview file: ${err.message}`);
		}
	};

	const renderPreview = () => {
		if (error) {
			return (
				<div className='text-center p-10 text-red-600 bg-red-50 border border-red-200'>
					<div className='w-12 h-12 bg-red-100 mx-auto mb-4 flex items-center justify-center'>
						<AlertTriangle className='w-6 h-6 text-red-600' />
					</div>
					<div>{error}</div>
				</div>
			);
		}

		if (!previewUrl) {
			const isLargeFile = metadata.size > 50 * 1024 * 1024;

			return (
				<div className='text-center py-16 text-gray-500'>
					<div className='animate-spin w-8 h-8 border-2 border-gray-200 border-t-primary-500 rounded-full mx-auto mb-4'></div>
					<div>Loading preview...</div>
					{isLargeFile && (
						<div className='mt-4 max-w-xs mx-auto'>
							<div className='text-sm text-gray-600 mb-2'>
								Large file detected ({formatSize(metadata.size)})
							</div>
							{isLoading && loadingProgress > 0 && (
								<div className='w-full bg-gray-200 h-2'>
									<div
										className='bg-primary-500 h-2 transition-all duration-300'
										style={{ width: `${loadingProgress}%` }}
									></div>
								</div>
							)}
						</div>
					)}
				</div>
			);
		}

		const { mime_type } = metadata;

		if (mime_type.startsWith('image/')) {
			const isLargeImage = metadata.size > 1 * 1024 * 1024; // 1MB threshold

			return (
				<img
					src={previewUrl}
					alt={metadata.filename}
					className='max-w-full max-h-[80vh] object-contain mx-auto'
					loading={isLargeImage ? 'lazy' : 'eager'}
					onLoad={() => setIsLoading(false)}
					onError={() => setError('Failed to load image')}
					style={{
						transition: 'opacity 0.3s ease',
						opacity: isLoading ? 0.7 : 1,
					}}
				/>
			);
		}

		if (mime_type.startsWith('video/')) {
			// For large video files, use optimized streaming endpoint
			const isLargeVideo = metadata.size > 5 * 1024 * 1024; // 5MB threshold
			let streamUrl = previewUrl;
			
			if (isLargeVideo) {
				streamUrl = `/api/stream/${fileId}`;
				const params = new URLSearchParams();
				if (password) params.append('password', password);
				if (adminToken) params.append('admin_token', adminToken);
				if (params.toString()) streamUrl += `?${params.toString()}`;
				console.log('Video streaming URL:', streamUrl);
				console.log('Admin token present for video:', !!adminToken);
			}

			return (
				<video
					controls
					className='max-w-full max-h-[80vh] mx-auto'
					src={streamUrl}
					preload={isLargeVideo ? 'metadata' : 'auto'}
					crossOrigin='anonymous'
				>
					Your browser does not support the video tag.
				</video>
			);
		}

		if (mime_type.startsWith('audio/')) {
			// For large audio files, use optimized streaming endpoint
			const isLargeAudio = metadata.size > 5 * 1024 * 1024; // 5MB threshold
			let streamUrl = previewUrl;
			
			if (isLargeAudio) {
				streamUrl = `/api/stream/${fileId}`;
				const params = new URLSearchParams();
				if (password) params.append('password', password);
				if (adminToken) params.append('admin_token', adminToken);
				if (params.toString()) streamUrl += `?${params.toString()}`;
				console.log('Audio streaming URL:', streamUrl);
				console.log('Admin token present for audio:', !!adminToken);
			}

			return (
				<div className='flex justify-center'>
					<audio
						controls
						className='w-full max-w-lg'
						preload={isLargeAudio ? 'metadata' : 'auto'}
						crossOrigin='anonymous'
					>
						<source src={streamUrl} type={mime_type} />
						Your browser does not support the audio tag.
					</audio>
				</div>
			);
		}

		if (mime_type === 'application/pdf') {
			return (
				<embed
					src={previewUrl}
					className='w-full h-96 border-0'
					title='PDF Preview'
					type='application/pdf'
				/>
			);
		}

		if (
			mime_type.startsWith('text/') ||
			mime_type === 'application/json' ||
			mime_type === 'application/xml'
		) {
			// For very large text files, show a warning and partial content
			const isVeryLargeText = metadata.size > 10 * 1024 * 1024; // 10MB threshold

			return (
				<div className='text-left'>
					{isVeryLargeText && (
						<div className='mb-4 p-3 bg-yellow-50 border border-yellow-200 rounded'>
							<div className='text-sm text-yellow-800'>
								Large text file ({formatSize(metadata.size)}) - content may be truncated for
								performance
							</div>
						</div>
					)}
					<iframe
						src={previewUrl}
						className='w-full h-96 border border-gray-300 bg-gray-50'
						title='Text Preview'
					/>
				</div>
			);
		}

		return (
			<div className='text-center py-16 text-gray-500 bg-gray-50 border border-gray-200'>
				<div className='w-12 h-12 bg-gray-100 mx-auto mb-4 flex items-center justify-center'>
					<File className='w-6 h-6 text-gray-500' />
				</div>
				<div>Preview not supported for this file type.</div>
				<div className='text-sm mt-2'>You can still download the file.</div>
			</div>
		);
	};

	const renderZipContents = () => {
		if (!zipContents) {
			return (
				<div className='text-center py-8 text-gray-500'>
					<div className='animate-spin w-6 h-6 border-2 border-gray-200 border-t-primary-500 rounded-full mx-auto mb-4'></div>
					<div>Loading ZIP contents...</div>
				</div>
			);
		}

		return (
			<div>
				<div className='bg-gray-50 px-4 py-3 border-b border-gray-200 font-medium'>
					{zipContents.filename}
				</div>
				<div className='max-h-96 overflow-y-auto'>
					{zipContents.files.map((file, index) => (
						<div
							key={index}
							className={`px-4 py-3 border-b border-gray-100 flex items-center gap-3 hover:bg-gray-50 ${
								!file.is_dir ? 'cursor-pointer' : ''
							}`}
							onDoubleClick={() => handleZipFileDoubleClick(file)}
							title={file.is_dir ? 'Directory' : 'Double-click to preview'}
						>
							<div className='w-5 text-center'>
								{file.is_dir ? (
									<Folder className='w-4 h-4 text-gray-500' />
								) : (
									<File className='w-4 h-4 text-gray-500' />
								)}
							</div>
							<div className='flex-1 font-mono text-sm select-none'>{file.name}</div>
							<div className='text-xs text-gray-500 select-none'>{formatSize(file.size)}</div>
							<div className='text-xs text-gray-400 select-none'>{formatDate(file.modified)}</div>
						</div>
					))}
				</div>
			</div>
		);
	};

	const isZipFile = metadata.filename.toLowerCase().endsWith('.zip');

	return (
		<div>
			{isZipFile && (
				<div className='mb-6'>
					<div className='flex border-b border-gray-200 bg-white'>
						<button
							className={`px-6 py-3 font-medium border-b-2 transition-colors ${
								activeTab === 'preview'
									? 'border-primary-500 text-primary-600'
									: 'border-transparent text-gray-500 hover:text-gray-700'
							}`}
							onClick={() => setActiveTab('preview')}
						>
							Preview
						</button>
						<button
							className={`px-6 py-3 font-medium border-b-2 transition-colors ${
								activeTab === 'zip'
									? 'border-primary-500 text-primary-600'
									: 'border-transparent text-gray-500 hover:text-gray-700'
							}`}
							onClick={() => setActiveTab('zip')}
						>
							ZIP Contents
						</button>
					</div>
				</div>
			)}

			<div className='card overflow-hidden'>
				{activeTab === 'preview' ? (
					<div className='p-6 min-h-96 flex items-center justify-center'>{renderPreview()}</div>
				) : (
					<div>{renderZipContents()}</div>
				)}
			</div>

			{/* ZIP File Preview Modal */}
			{isZipFilePreviewOpen && zipFilePreview && (
				<div
					className='fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4'
					onClick={closeZipFilePreview}
				>
					<div
						className='bg-white shadow-angular-xl max-w-4xl max-h-[90vh] w-full overflow-hidden'
						onClick={(e) => e.stopPropagation()}
					>
						<div className='flex items-center justify-between p-4 border-b border-gray-200 bg-gray-50'>
							<div className='flex-1'>
								<h3 className='text-lg font-medium text-gray-900'>{zipFilePreview.file.name}</h3>
								<p className='text-sm text-gray-500'>
									{formatSize(zipFilePreview.file.size)} •{' '}
									{formatDate(zipFilePreview.file.modified)}
								</p>
								{zipContents && (
									<p className='text-xs text-gray-400 mt-1'>
										{zipFilePreview.currentIndex + 1} of {getPreviewableFiles().length} files
									</p>
								)}
							</div>
							<div className='flex items-center gap-4'>
								{zipContents && getPreviewableFiles().length > 1 && (
									<div className='flex items-center gap-4 text-sm text-gray-500'>
										<div className='flex items-center gap-1'>
											<ChevronLeft className='w-4 h-4' />
											<ChevronRight className='w-4 h-4' />
											<span>Navigate</span>
										</div>
										<div className='flex items-center gap-1'>
											<span>ESC</span>
											<span>Close</span>
										</div>
									</div>
								)}
								<button onClick={closeZipFilePreview} className='text-gray-400 hover:text-gray-600'>
									<X className='w-6 h-6' />
								</button>
							</div>
						</div>
						<div className='p-6 overflow-auto max-h-[calc(90vh-8rem)]'>
							{renderZipFilePreview()}
						</div>

						{/* Navigation arrows */}
						{zipContents && getPreviewableFiles().length > 1 && (
							<>
								<button
									onClick={() => navigateToFile('prev')}
									className='absolute left-4 top-1/2 transform -translate-y-1/2 bg-black/50 hover:bg-black/70 text-white p-3 transition-all'
								>
									<ChevronLeft className='w-6 h-6' />
								</button>
								<button
									onClick={() => navigateToFile('next')}
									className='absolute right-4 top-1/2 transform -translate-y-1/2 bg-black/50 hover:bg-black/70 text-white p-3 transition-all'
								>
									<ChevronRight className='w-6 h-6' />
								</button>
							</>
						)}
					</div>
				</div>
			)}
		</div>
	);

	function renderZipFilePreview() {
		if (!zipFilePreview) return null;

		const { previewUrl, contentType } = zipFilePreview;

		if (contentType.startsWith('image/')) {
			return (
				<img
					src={previewUrl}
					alt={zipFilePreview.file.name}
					className='max-w-full max-h-[60vh] object-contain mx-auto'
				/>
			);
		}

		if (contentType.startsWith('video/')) {
			// For large video files, show loading optimization
			const isLargeVideo = zipFilePreview.file.size > 5 * 1024 * 1024;

			return (
				<video
					controls
					className='max-w-full max-h-[60vh] mx-auto'
					src={previewUrl}
					preload={isLargeVideo ? 'metadata' : 'auto'}
				>
					Your browser does not support the video tag.
				</video>
			);
		}

		if (contentType.startsWith('audio/')) {
			// For large audio files, show loading optimization
			const isLargeAudio = zipFilePreview.file.size > 5 * 1024 * 1024;

			return (
				<div className='flex justify-center'>
					<audio controls className='w-full max-w-lg' preload={isLargeAudio ? 'metadata' : 'auto'}>
						<source src={previewUrl} type={contentType} />
						Your browser does not support the audio tag.
					</audio>
				</div>
			);
		}

		if (contentType === 'application/pdf') {
			return <iframe src={previewUrl} className='w-full h-96 border-0' title='PDF Preview' />;
		}

		if (
			contentType.startsWith('text/') ||
			contentType === 'application/json' ||
			contentType === 'application/xml'
		) {
			return (
				<div className='text-left'>
					<iframe
						src={previewUrl}
						className='w-full h-96 border border-gray-300 bg-gray-50'
						title='Text Preview'
					/>
				</div>
			);
		}

		return (
			<div className='text-center py-16 text-gray-500 bg-gray-50 border border-gray-200'>
				<div className='w-12 h-12 bg-gray-100 mx-auto mb-4 flex items-center justify-center'>
					<File className='w-6 h-6 text-gray-500' />
				</div>
				<div>Preview not supported for this file type.</div>
				<div className='text-sm mt-2'>Content type: {contentType}</div>
			</div>
		);
	}
};

export default FilePreview;
