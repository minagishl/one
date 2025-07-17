import React, { useState, useEffect } from 'react';
import { AlertTriangle, File, Folder } from 'lucide-react';
import { FileMetadata, ZipContents, ZipFile } from '../types';
import { getFilePreview, getZipContents, getZipFilePreview } from '../utils/api';
import { formatSize, formatDate } from '../utils/format';

interface FilePreviewProps {
	fileId: string;
	metadata: FileMetadata;
}

const FilePreview: React.FC<FilePreviewProps> = ({ fileId, metadata }) => {
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

	useEffect(() => {
		loadPreview();
		if (metadata.filename.toLowerCase().endsWith('.zip')) {
			loadZipContents();
		}

		// Cleanup function to revoke object URL and prevent memory leaks
		return () => {
			if (previewUrl) {
				URL.revokeObjectURL(previewUrl);
			}
			if (zipFilePreview?.previewUrl) {
				URL.revokeObjectURL(zipFilePreview.previewUrl);
			}
		};
	}, [fileId, metadata]);

	// Keyboard navigation for ZIP file preview
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
			// Revoke previous URL to prevent memory leaks
			if (previewUrl) {
				URL.revokeObjectURL(previewUrl);
			}

			const { blob } = await getFilePreview(fileId);
			const url = URL.createObjectURL(blob);
			setPreviewUrl(url);
			setError('');
		} catch (err: any) {
			console.error('Error loading preview:', err);
			if (err.message && err.message.includes('Password required')) {
				setError('This file is password protected. Use the download button to enter the password.');
			} else if (err.message && err.message.includes('415')) {
				setError(
					'This file type cannot be previewed in the browser. Please download the file to view it.'
				);
			} else if (err.message && err.message.includes('404')) {
				setError('File not found or expired');
			} else {
				setError('Failed to load preview');
			}
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

	// Get previewable files (non-directories)
	const getPreviewableFiles = () => {
		if (!zipContents) return [];
		return zipContents.files.filter((file) => !file.is_dir);
	};

	const handleZipFileDoubleClick = async (file: ZipFile) => {
		if (file.is_dir) {
			return; // Don't preview directories
		}

		const previewableFiles = getPreviewableFiles();
		const currentIndex = previewableFiles.findIndex((f) => f.name === file.name);

		try {
			const { blob, contentType } = await getZipFilePreview(fileId, file.name);

			// Revoke previous preview URL if exists
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

			// Revoke previous preview URL
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
				<div className='text-center py-16 text-red-600 bg-red-50 rounded-lg'>
					<div className='flex justify-center mb-4'>
						<AlertTriangle className='w-12 h-12' />
					</div>
					<div>{error}</div>
				</div>
			);
		}

		if (!previewUrl) {
			return (
				<div className='text-center py-16 text-gray-500'>
					<div className='animate-spin w-8 h-8 border-3 border-gray-300 border-t-primary-500 rounded-full mx-auto mb-4'></div>
					<div>Loading preview...</div>
				</div>
			);
		}

		const { mime_type } = metadata;

		if (mime_type.startsWith('image/')) {
			return (
				<img
					src={previewUrl}
					alt={metadata.filename}
					className='max-w-full max-h-[80vh] object-contain mx-auto'
				/>
			);
		}

		if (mime_type.startsWith('video/')) {
			return (
				<video controls className='max-w-full max-h-[80vh] mx-auto' src={previewUrl}>
					Your browser does not support the video tag.
				</video>
			);
		}

		if (mime_type.startsWith('audio/')) {
			return (
				<div className='flex justify-center'>
					<audio controls className='w-full max-w-lg'>
						<source src={previewUrl} type={mime_type} />
						Your browser does not support the audio tag.
					</audio>
				</div>
			);
		}

		if (mime_type === 'application/pdf') {
			return <iframe src={previewUrl} className='w-full h-96 border-0' title='PDF Preview' />;
		}

		if (
			mime_type.startsWith('text/') ||
			mime_type === 'application/json' ||
			mime_type === 'application/xml'
		) {
			return (
				<div className='text-left'>
					<iframe
						src={previewUrl}
						className='w-full h-96 border border-gray-300 rounded-lg bg-gray-50'
						title='Text Preview'
					/>
				</div>
			);
		}

		return (
			<div className='text-center py-16 text-gray-500 bg-gray-50 rounded-lg'>
				<div className='flex justify-center mb-4'>
					<File className='w-12 h-12' />
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
					<div className='animate-spin w-6 h-6 border-2 border-gray-300 border-t-primary-500 rounded-full mx-auto mb-4'></div>
					<div>Loading ZIP contents...</div>
				</div>
			);
		}

		return (
			<div>
				<div className='bg-gray-50 px-4 py-3 border-b font-medium'>{zipContents.filename}</div>
				<div className='max-h-96 overflow-y-auto'>
					{zipContents.files.map((file, index) => (
						<div
							key={index}
							className={`px-4 py-2 border-b border-gray-100 flex items-center gap-3 hover:bg-gray-50 ${
								!file.is_dir ? 'cursor-pointer' : ''
							}`}
							onDoubleClick={() => handleZipFileDoubleClick(file)}
							title={file.is_dir ? 'Directory' : 'Double-click to preview'}
						>
							<div className='w-5 text-center'>
								{file.is_dir ? <Folder className='w-4 h-4' /> : <File className='w-4 h-4' />}
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
					<div className='flex border-b border-gray-200 bg-white rounded-t-lg'>
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

			<div className='bg-white rounded-lg shadow-sm border overflow-hidden'>
				{activeTab === 'preview' ? (
					<div className='p-6 min-h-96 flex items-center justify-center'>{renderPreview()}</div>
				) : (
					<div>{renderZipContents()}</div>
				)}
			</div>

			{/* ZIP File Preview Modal */}
			{isZipFilePreviewOpen && zipFilePreview && (
				<div
					className='fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4'
					onClick={closeZipFilePreview}
				>
					<div
						className='bg-white rounded-lg shadow-xl max-w-4xl max-h-[90vh] w-full overflow-hidden'
						onClick={(e) => e.stopPropagation()}
					>
						<div className='flex items-center justify-between p-4 border-b bg-gray-50'>
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
							<div className='flex items-center gap-2'>
								{zipContents && getPreviewableFiles().length > 1 && (
									<div className='flex items-center gap-1 text-sm text-gray-500 mr-4'>
										<span>← →</span>
										<span>Navigate</span>
										<span className='mx-2'>•</span>
										<span>ESC</span>
										<span>Close</span>
									</div>
								)}
								<button
									onClick={closeZipFilePreview}
									className='text-gray-400 hover:text-gray-600 text-2xl font-bold'
								>
									×
								</button>
							</div>
						</div>
						<div className='p-6 overflow-auto max-h-[calc(90vh-8rem)]'>
							{renderZipFilePreview()}
						</div>

						{/* Navigation arrows (visible on hover) */}
						{zipContents && getPreviewableFiles().length > 1 && (
							<>
								<button
									onClick={() => navigateToFile('prev')}
									className='absolute left-4 top-1/2 transform -translate-y-1/2 bg-black bg-opacity-50 hover:bg-opacity-70 text-white p-2 rounded-full opacity-0 hover:opacity-100 transition-opacity'
								>
									<svg className='w-6 h-6' fill='none' stroke='currentColor' viewBox='0 0 24 24'>
										<path
											strokeLinecap='round'
											strokeLinejoin='round'
											strokeWidth={2}
											d='M15 19l-7-7 7-7'
										/>
									</svg>
								</button>
								<button
									onClick={() => navigateToFile('next')}
									className='absolute right-4 top-1/2 transform -translate-y-1/2 bg-black bg-opacity-50 hover:bg-opacity-70 text-white p-2 rounded-full opacity-0 hover:opacity-100 transition-opacity'
								>
									<svg className='w-6 h-6' fill='none' stroke='currentColor' viewBox='0 0 24 24'>
										<path
											strokeLinecap='round'
											strokeLinejoin='round'
											strokeWidth={2}
											d='M9 5l7 7-7 7'
										/>
									</svg>
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
			return (
				<video controls className='max-w-full max-h-[60vh] mx-auto' src={previewUrl}>
					Your browser does not support the video tag.
				</video>
			);
		}

		if (contentType.startsWith('audio/')) {
			return (
				<div className='flex justify-center'>
					<audio controls className='w-full max-w-lg'>
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
						className='w-full h-96 border border-gray-300 rounded-lg bg-gray-50'
						title='Text Preview'
					/>
				</div>
			);
		}

		return (
			<div className='text-center py-16 text-gray-500 bg-gray-50 rounded-lg'>
				<div className='flex justify-center mb-4'>
					<File className='w-12 h-12' />
				</div>
				<div>Preview not supported for this file type.</div>
				<div className='text-sm mt-2'>Content type: {contentType}</div>
			</div>
		);
	}
};

export default FilePreview;
