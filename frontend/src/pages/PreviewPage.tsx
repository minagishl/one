import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { AlertTriangle } from 'lucide-react';
import FilePreview from '../components/FilePreview';
import { FileMetadata } from '../types';
import { downloadFile, deleteFile, getFileMetadata } from '../utils/api';
import { formatSize, formatDate, formatCountdown } from '../utils/format';

const PreviewPage: React.FC = () => {
	const { fileId } = useParams<{ fileId: string }>();
	const navigate = useNavigate();
	const [metadata, setMetadata] = useState<FileMetadata | null>(null);
	const [error, setError] = useState<string>('');
	const [countdown, setCountdown] = useState<string>('');
	const [showPasswordDialog, setShowPasswordDialog] = useState(false);
	const [passwordDialogType, setPasswordDialogType] = useState<'download' | 'delete'>('download');
	const [passwordInput, setPasswordInput] = useState('');

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
		}
	};

	if (error) {
		return (
			<div className='min-h-screen bg-gray-50 flex items-center justify-center'>
				<div className='text-center'>
					<div className='flex justify-center mb-4'>
						<AlertTriangle className='w-16 h-16 text-red-500' />
					</div>
					<div className='text-2xl font-medium text-gray-800 mb-2'>File Not Found</div>
					<div className='text-gray-600 mb-6'>{error}</div>
					<button
						onClick={() => navigate('/')}
						className='bg-primary-500 text-white px-6 py-2 rounded-lg hover:bg-primary-600 transition-colors'
					>
						Go Back
					</button>
				</div>
			</div>
		);
	}

	if (!metadata) {
		return (
			<div className='min-h-screen bg-gray-50 flex items-center justify-center'>
				<div className='text-center'>
					<div className='animate-spin w-12 h-12 border-4 border-gray-300 border-t-primary-500 rounded-full mx-auto mb-4'></div>
					<div className='text-gray-600'>Loading file...</div>
				</div>
			</div>
		);
	}

	const compressionRatio =
		metadata.size > 0 ? Math.round((1 - metadata.compressed_size / metadata.size) * 100) : 0;

	return (
		<div className='min-h-screen bg-gray-50'>
			<div className='max-w-6xl mx-auto p-6'>
				<div className='mb-6'>
					<button
						onClick={() => navigate('/')}
						className='text-primary-600 hover:text-primary-700 font-medium'
					>
						← Back to Upload
					</button>
				</div>

				<div className='bg-white rounded-lg shadow-sm border mb-6 overflow-hidden'>
					<div className='px-6 py-4 border-b flex items-center justify-between'>
						<h1 className='text-2xl font-bold text-gray-800'>File Preview</h1>
						<div className='flex gap-3'>
							<button
								onClick={handleDownload}
								className='bg-primary-500 text-white px-4 py-2 rounded-lg hover:bg-primary-600 transition-colors'
							>
								Download
							</button>
							<button
								onClick={handleDelete}
								className='bg-red-500 text-white px-4 py-2 rounded-lg hover:bg-red-600 transition-colors'
							>
								Delete
							</button>
						</div>
					</div>

					<div className='p-6 bg-gray-50'>
						<div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 text-sm'>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Filename</div>
								<div className='text-gray-800'>{metadata.filename}</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Size</div>
								<div className='text-gray-800'>{formatSize(metadata.size)}</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Compressed Size</div>
								<div className='text-gray-800'>
									{formatSize(metadata.compressed_size)} ({compressionRatio}% saved)
								</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Type</div>
								<div className='text-gray-800'>{metadata.mime_type}</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Compression</div>
								<div className='text-gray-800'>{metadata.compression}</div>
							</div>
							<div>
								<div className='font-medium text-gray-600 mb-1'>Uploaded</div>
								<div className='text-gray-800'>{formatDate(metadata.upload_time)}</div>
							</div>
						</div>

						<div className='mt-4 bg-red-50 border border-red-200 rounded-lg p-3 text-center'>
							<div className='font-medium text-red-800'>Expires in: {countdown}</div>
						</div>
					</div>
				</div>

				<FilePreview fileId={metadata.id} metadata={metadata} />
			</div>

			{/* Password Dialog */}
			{showPasswordDialog && (
				<div className='fixed inset-0 bg-black/50 flex items-center justify-center z-50'>
					<div className='bg-white rounded-2xl p-6 w-full max-w-md mx-4'>
						<h3 className='text-xl font-semibold text-gray-800 mb-4'>
							{passwordDialogType === 'download'
								? 'Download Password Required'
								: 'Delete Password Required'}
						</h3>
						<p className='text-gray-600 mb-6'>
							{passwordDialogType === 'download'
								? 'This file is password protected. Enter the password to download it.'
								: 'Enter the delete password to permanently remove this file.'}
						</p>
						<input
							type='password'
							value={passwordInput}
							onChange={(e) => setPasswordInput(e.target.value)}
							placeholder={
								passwordDialogType === 'download'
									? 'Enter download password'
									: 'Enter delete password'
							}
							className='w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 mb-4'
							onKeyDown={(e) => e.key === 'Enter' && handlePasswordSubmit()}
						/>
						<div className='flex gap-3'>
							<button
								onClick={() => {
									setShowPasswordDialog(false);
									setPasswordInput('');
								}}
								className='flex-1 px-4 py-2 bg-gray-200 text-gray-800 rounded-lg hover:bg-gray-300 transition-colors'
							>
								Cancel
							</button>
							<button
								onClick={handlePasswordSubmit}
								disabled={!passwordInput}
								className='flex-1 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors'
							>
								{passwordDialogType === 'download' ? 'Download' : 'Delete'}
							</button>
						</div>
					</div>
				</div>
			)}
		</div>
	);
};

export default PreviewPage;
