import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { AlertTriangle, ArrowLeft, Download, Trash2 } from 'lucide-react';
import FilePreview from '../components/FilePreview';
import { FileMetadata } from '../types';
import { downloadFile, deleteFile, getFileMetadata, getFilePreview } from '../utils/api';
import { formatSize, formatDate, formatCountdown } from '../utils/format';

const PreviewPage: React.FC = () => {
	const { fileId } = useParams<{ fileId: string }>();
	const navigate = useNavigate();
	const [metadata, setMetadata] = useState<FileMetadata | null>(null);
	const [error, setError] = useState<string>('');
	const [countdown, setCountdown] = useState<string>('');
	const [showPasswordDialog, setShowPasswordDialog] = useState(false);
	const [passwordDialogType, setPasswordDialogType] = useState<'download' | 'delete' | 'preview'>('download');
	const [passwordInput, setPasswordInput] = useState('');
	const [previewPassword, setPreviewPassword] = useState<string>('');
	const [isPreviewAuthenticated, setIsPreviewAuthenticated] = useState(false);

	useEffect(() => {
		if (fileId) {
			loadFileMetadata();
		}
	}, [fileId]);

	useEffect(() => {
		if (metadata) {
			const interval = setInterval(() => {
				setCountdown(formatCountdown(metadata.expires_at));
			}, 1000);
			return () => clearInterval(interval);
		}
	}, [metadata]);

	const loadFileMetadata = async () => {
		if (!fileId) return;

		try {
			const metadata = await getFileMetadata(fileId);
			setMetadata(metadata);
			setError('');
			
			// Reset preview authentication when metadata changes
			setIsPreviewAuthenticated(false);
			setPreviewPassword('');
		} catch (err) {
			setError(err instanceof Error ? err.message : 'File not found or expired');
		}
	};

	const handleDownload = async () => {
		if (!metadata) return;

		if (metadata.has_download_password) {
			setPasswordDialogType('download');
			setShowPasswordDialog(true);
		} else {
			try {
				await downloadFile(metadata.id, metadata.filename);
			} catch (error) {
				alert('Download failed: ' + (error instanceof Error ? error.message : 'Unknown error'));
			}
		}
	};

	const handleDelete = async () => {
		if (!metadata) return;

		if (window.confirm(`Are you sure you want to delete "${metadata.filename}"?`)) {
			setPasswordDialogType('delete');
			setShowPasswordDialog(true);
		}
	};

	const handlePasswordSubmit = async () => {
		if (!metadata || !passwordInput) return;

		if (passwordDialogType === 'download') {
			try {
				await downloadFile(metadata.id, metadata.filename, passwordInput);
				setShowPasswordDialog(false);
				setPasswordInput('');
			} catch (error) {
				alert('Download failed: ' + (error instanceof Error ? error.message : 'Unknown error'));
			}
		} else if (passwordDialogType === 'delete') {
			const result = await deleteFile(metadata.id, passwordInput);
			if (result.success) {
				alert('File deleted successfully');
				navigate('/');
			} else {
				alert('Error: ' + result.error);
			}
			setShowPasswordDialog(false);
			setPasswordInput('');
		} else if (passwordDialogType === 'preview') {
			try {
				// Test the password by making a preview request
				await getFilePreview(metadata.id, passwordInput);
				// If successful, store password and show preview
				setPreviewPassword(passwordInput);
				setIsPreviewAuthenticated(true);
				setShowPasswordDialog(false);
				setPasswordInput('');
			} catch (error) {
				// If password is wrong, show error
				if (error instanceof Error && error.message.includes('Password required')) {
					alert('Incorrect password. Please try again.');
				} else {
					alert('Failed to load preview.');
				}
			}
		}
	};

	const handleShowPreview = () => {
		if (!metadata) return;

		if (metadata.has_download_password && !isPreviewAuthenticated) {
			setPasswordDialogType('preview');
			setShowPasswordDialog(true);
		} else {
			setIsPreviewAuthenticated(true);
		}
	};

	if (error) {
		return (
			<div className='min-h-screen bg-gray-25 flex items-center justify-center'>
				<div className='text-center max-w-md mx-auto px-6'>
					<div className='w-16 h-16 bg-red-100 mx-auto mb-6 flex items-center justify-center'>
						<AlertTriangle className='w-8 h-8 text-red-600' />
					</div>
					<h1 className='text-2xl font-medium text-gray-900 mb-4'>File Not Found</h1>
					<p className='text-gray-600 mb-8'>{error}</p>
					<button
						onClick={() => navigate('/')}
						className='btn-primary inline-flex items-center gap-2'
					>
						<ArrowLeft className='w-4 h-4' />
						Go Back
					</button>
				</div>
			</div>
		);
	}

	if (!metadata) {
		return (
			<div className='min-h-screen bg-gray-25 flex items-center justify-center'>
				<div className='text-center'>
					<div className='animate-spin w-8 h-8 border-2 border-gray-200 border-t-primary-500 rounded-full mx-auto mb-4'></div>
					<p className='text-gray-600'>Loading file...</p>
				</div>
			</div>
		);
	}

	const compressionRatio =
		metadata.size > 0 ? Math.round((1 - metadata.compressed_size / metadata.size) * 100) : 0;

	return (
		<div className='min-h-screen bg-gray-25'>
			{/* Header */}
			<header className='bg-white border-b border-gray-200'>
				<div className='max-w-6xl mx-auto px-6 py-4'>
					<div className='flex items-center justify-between'>
						<div className='flex items-center gap-4'>
							<button
								onClick={() => navigate('/')}
								className='text-gray-600 hover:text-gray-900 inline-flex items-center gap-2 font-medium'
							>
								<ArrowLeft className='w-4 h-4' />
								<span className='hidden sm:inline'>Back to Upload</span>
							</button>
							<div className='h-6 w-px bg-gray-300 hidden sm:block'></div>
							<h1 className='text-xl font-medium text-gray-900 hidden sm:block'>File Preview</h1>
							<h1 className='text-lg font-medium text-gray-900 sm:hidden'>Preview</h1>
						</div>
						<div className='flex gap-2 sm:gap-3'>
							<button
								onClick={handleDownload}
								className='btn-primary inline-flex items-center gap-2 px-4 py-2 sm:px-6 sm:py-3'
								title='Download'
							>
								<Download className='w-4 h-4' />
								<span className='hidden sm:inline'>Download</span>
							</button>
							<button
								onClick={handleDelete}
								className='bg-red-500 text-white px-4 py-2 sm:px-6 sm:py-3 hover:bg-red-600 transition-colors inline-flex items-center gap-2'
								title='Delete'
							>
								<Trash2 className='w-4 h-4' />
								<span className='hidden sm:inline'>Delete</span>
							</button>
						</div>
					</div>
				</div>
			</header>

			{/* Main Content */}
			<main className='max-w-6xl mx-auto px-6 py-8'>
				{/* File Information */}
				<div className='card mb-8 overflow-hidden'>
					<div className='px-6 py-4 border-b border-gray-200 bg-gray-50'>
						<h2 className='text-lg font-medium text-gray-900'>{metadata.filename}</h2>
					</div>

					<div className='p-6'>
						<div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 text-sm'>
							<div>
								<div className='font-medium text-gray-600 mb-1'>File Size</div>
								<div className='text-gray-900'>{formatSize(metadata.size)}</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Compressed Size</div>
								<div className='text-gray-900'>
									{formatSize(metadata.compressed_size)}
									<span className='text-green-600 ml-1'>({compressionRatio}% saved)</span>
								</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>File Type</div>
								<div className='text-gray-900'>{metadata.mime_type}</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Compression</div>
								<div className='text-gray-900'>{metadata.compression}</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Uploaded</div>
								<div className='text-gray-900'>{formatDate(metadata.upload_time)}</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Expires In</div>
								<div className='text-red-600 font-medium'>{countdown}</div>
							</div>
						</div>
					</div>
				</div>

				{/* File Preview */}
				{metadata.has_download_password && !isPreviewAuthenticated ? (
					<div className='card text-center py-12'>
						<div className='w-16 h-16 bg-primary-100 mx-auto mb-6 flex items-center justify-center'>
							<AlertTriangle className='w-8 h-8 text-primary-600' />
						</div>
						<h3 className='text-xl font-medium text-gray-900 mb-4'>This file is password protected</h3>
						<p className='text-gray-600 mb-8 max-w-md mx-auto'>
							A password is required to preview this file. Click the button below to enter the password.
						</p>
						<button
							onClick={handleShowPreview}
							className='btn-primary inline-flex items-center gap-2'
						>
							Show Preview
						</button>
					</div>
				) : (
					<FilePreview 
						fileId={metadata.id} 
						metadata={metadata} 
						password={metadata.has_download_password ? previewPassword : undefined}
					/>
				)}
			</main>

			{/* Password Dialog */}
			{showPasswordDialog && (
				<div className='fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4'>
					<div className='bg-white shadow-angular-xl w-full max-w-md'>
						<div className='p-6'>
							<h3 className='text-xl font-medium text-gray-900 mb-4'>
								{passwordDialogType === 'download'
									? 'Download Password Required'
									: passwordDialogType === 'delete'
									? 'Delete Password Required'
									: 'Preview Password Required'}
							</h3>
							<p className='text-gray-600 mb-6'>
								{passwordDialogType === 'download'
									? 'This file is password protected. Enter the password to download it.'
									: passwordDialogType === 'delete'
									? 'Enter the delete password to permanently remove this file.'
									: 'This file is password protected. Enter the password to preview it.'}
							</p>
							<input
								type='password'
								value={passwordInput}
								onChange={(e) => setPasswordInput(e.target.value)}
								placeholder={
									passwordDialogType === 'download'
										? 'Enter download password'
										: passwordDialogType === 'delete'
										? 'Enter delete password'
										: 'Enter password'
								}
								className='input-field w-full mb-6'
								onKeyDown={(e) => e.key === 'Enter' && handlePasswordSubmit()}
							/>
							<div className='flex gap-3'>
								<button
									onClick={() => {
										setShowPasswordDialog(false);
										setPasswordInput('');
									}}
									className='btn-secondary flex-1'
								>
									Cancel
								</button>
								<button
									onClick={handlePasswordSubmit}
									disabled={!passwordInput}
									className='btn-primary flex-1 disabled:bg-gray-300 disabled:cursor-not-allowed'
								>
									{passwordDialogType === 'download' 
										? 'Download' 
										: passwordDialogType === 'delete' 
										? 'Delete' 
										: 'Show Preview'}
								</button>
							</div>
						</div>
					</div>
				</div>
			)}
		</div>
	);
};

export default PreviewPage;
