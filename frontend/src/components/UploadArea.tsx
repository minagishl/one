import React, { useState, useCallback } from 'react';
import {
	Lock,
	Upload,
	Check,
	AlertTriangle,
	FileText,
	RefreshCw,
	Archive,
	QrCode,
	X,
} from 'lucide-react';
import { uploadFile } from '../utils/api';
import { ChunkUploader, shouldUseChunkUpload, formatUploadProgress } from '../utils/chunkUpload';
import { formatSize } from '../utils/format';
import { UploadResult } from '../types';

interface UploadAreaProps {
	onUploadComplete?: (results: UploadResult[]) => void;
}

interface UploadProgress {
	filename: string;
	progress: number;
	chunkProgress?: string;
	isChunkUpload: boolean;
	retryCount?: number;
	error?: string;
}

const UploadArea: React.FC<UploadAreaProps> = ({ onUploadComplete }) => {
	const [isDragOver, setIsDragOver] = useState(false);
	const [isUploading, setIsUploading] = useState(false);
	const [uploadResults, setUploadResults] = useState<UploadResult[]>([]);
	const [downloadPassword, setDownloadPassword] = useState('');
	const [enablePasswordProtection, setEnablePasswordProtection] = useState(false);
	const [uploadProgress, setUploadProgress] = useState<UploadProgress[]>([]);
	const [enableBatchUpload, setEnableBatchUpload] = useState(false);
	const [showQrModal, setShowQrModal] = useState(false);
	const [selectedFileForQr, setSelectedFileForQr] = useState<UploadResult | null>(null);
	const [qrCodeUrl, setQrCodeUrl] = useState<string>('');

	const createZipFile = async (files: File[]): Promise<File> => {
		const JSZip = (await import('jszip')).default;
		const zip = new JSZip();

		files.forEach((file) => {
			zip.file(file.name, file);
		});

		const zipBlob = await zip.generateAsync({ type: 'blob' });
		const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
		const zipFileName = `files-${timestamp}.zip`;

		return new File([zipBlob], zipFileName, { type: 'application/zip' });
	};

	const generateQrCode = async (uploadResult: UploadResult) => {
		if (!uploadResult.fileId) return;

		try {
			const QRCode = (await import('qrcode')).default;
			const fileUrl = `${window.location.origin}/f/${uploadResult.fileId}`;
			const qrDataURL = await QRCode.toDataURL(fileUrl, {
				width: 256,
				margin: 2,
				color: {
					dark: '#000000',
					light: '#FFFFFF',
				},
			});
			setQrCodeUrl(qrDataURL);
			setSelectedFileForQr(uploadResult);
			setShowQrModal(true);
		} catch (error) {
			alert(
				'Failed to generate QR code: ' + (error instanceof Error ? error.message : 'Unknown error')
			);
		}
	};

	const handleFiles = useCallback(
		async (files: File[]) => {
			if (files.length === 0) return;

			if (enablePasswordProtection && !downloadPassword.trim()) {
				alert('Please enter a password to protect your files');
				return;
			}

			setIsUploading(true);
			setUploadResults([]);
			setUploadProgress([]);

			// If batch upload is enabled and multiple files are selected, create a zip
			if (enableBatchUpload && files.length > 1) {
				try {
					const zipFile = await createZipFile(files);

					// Initialize progress for the zip file
					const initialProgress: UploadProgress[] = [
						{
							filename: zipFile.name,
							progress: 0,
							isChunkUpload: shouldUseChunkUpload(zipFile),
							retryCount: 0,
						},
					];
					setUploadProgress(initialProgress);

					const updateProgress = (progress: number) => {
						setUploadProgress((prev) => prev.map((p) => ({ ...p, progress })));
					};

					const updateChunkProgress = (chunkIndex: number, totalChunks: number) => {
						setUploadProgress((prev) =>
							prev.map((p) => ({
								...p,
								chunkProgress: formatUploadProgress(chunkIndex, totalChunks, p.progress),
							}))
						);
					};

					const updateRetryCount = (chunkIndex: number, attempt: number) => {
						setUploadProgress((prev) =>
							prev.map((p) => ({
								...p,
								retryCount: attempt,
								chunkProgress: `Retrying chunk ${chunkIndex} (attempt ${attempt})`,
							}))
						);
					};

					const updateError = (error: string) => {
						setUploadProgress((prev) => prev.map((p) => ({ ...p, error })));
					};

					let result;
					if (shouldUseChunkUpload(zipFile)) {
						const chunkResult = await ChunkUploader.uploadFile({
							file: zipFile,
							downloadPassword: enablePasswordProtection ? downloadPassword : undefined,
							onProgress: updateProgress,
							onChunkProgress: updateChunkProgress,
							onRetry: updateRetryCount,
							onError: updateError,
						});

						result = {
							success: chunkResult.success,
							filename: zipFile.name,
							fileId: chunkResult.fileId,
							metadata: chunkResult.metadata,
							delete_password: chunkResult.delete_password,
							error: chunkResult.error,
						};
					} else {
						updateProgress(50);
						result = await uploadFile(
							zipFile,
							enablePasswordProtection ? downloadPassword : undefined
						);
						updateProgress(100);
					}

					setUploadResults([result]);
					setIsUploading(false);

					if (onUploadComplete) {
						onUploadComplete([result]);
					}
					return;
				} catch (error) {
					console.error('Failed to create zip file:', error);
					alert('Failed to create zip file. Please try uploading files individually.');
					setIsUploading(false);
					return;
				}
			}

			// Normal individual file upload
			// Initialize progress for all files
			const initialProgress: UploadProgress[] = files.map((file) => ({
				filename: file.name,
				progress: 0,
				isChunkUpload: shouldUseChunkUpload(file),
				retryCount: 0,
			}));
			setUploadProgress(initialProgress);

			const uploadPromises = files.map(async (file, index) => {
				const updateProgress = (progress: number) => {
					setUploadProgress((prev) => prev.map((p, i) => (i === index ? { ...p, progress } : p)));
				};

				const updateChunkProgress = (chunkIndex: number, totalChunks: number) => {
					setUploadProgress((prev) =>
						prev.map((p, i) =>
							i === index
								? {
										...p,
										chunkProgress: formatUploadProgress(chunkIndex, totalChunks, p.progress),
								  }
								: p
						)
					);
				};

				const updateRetryCount = (chunkIndex: number, attempt: number) => {
					setUploadProgress((prev) =>
						prev.map((p, i) =>
							i === index
								? {
										...p,
										retryCount: attempt,
										chunkProgress: `Retrying chunk ${chunkIndex} (attempt ${attempt})`,
								  }
								: p
						)
					);
				};

				const updateError = (error: string) => {
					setUploadProgress((prev) => prev.map((p, i) => (i === index ? { ...p, error } : p)));
				};

				// Use chunk upload for large files
				if (shouldUseChunkUpload(file)) {
					const chunkResult = await ChunkUploader.uploadFile({
						file,
						downloadPassword: enablePasswordProtection ? downloadPassword : undefined,
						onProgress: updateProgress,
						onChunkProgress: updateChunkProgress,
						onRetry: updateRetryCount,
						onError: updateError,
					});

					// Convert ChunkUploadResult to UploadResult
					return {
						success: chunkResult.success,
						filename: file.name,
						fileId: chunkResult.fileId,
						metadata: chunkResult.metadata,
						delete_password: chunkResult.delete_password,
						error: chunkResult.error,
					};
				} else {
					// Use regular upload for small files
					updateProgress(50); // Show some progress
					const result = await uploadFile(
						file,
						enablePasswordProtection ? downloadPassword : undefined
					);
					updateProgress(100);
					return result;
				}
			});

			const results = await Promise.all(uploadPromises);
			setUploadResults(results);
			setIsUploading(false);

			if (onUploadComplete) {
				onUploadComplete(results);
			}
		},
		[enablePasswordProtection, downloadPassword, enableBatchUpload, onUploadComplete]
	);

	const handleDragOver = useCallback((e: React.DragEvent) => {
		e.preventDefault();
		setIsDragOver(true);
	}, []);

	const handleDragLeave = useCallback((e: React.DragEvent) => {
		e.preventDefault();
		setIsDragOver(false);
	}, []);

	const handleDrop = useCallback(
		(e: React.DragEvent) => {
			e.preventDefault();
			setIsDragOver(false);
			const files = Array.from(e.dataTransfer.files);
			handleFiles(files);
		},
		[handleFiles]
	);

	const handleFileInput = useCallback(
		(e: React.ChangeEvent<HTMLInputElement>) => {
			const files = Array.from(e.target.files || []);
			handleFiles(files);
		},
		[handleFiles]
	);

	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text);
	};

	const handlePasswordProtectionToggle = (enabled: boolean) => {
		setEnablePasswordProtection(enabled);
		if (!enabled) {
			setDownloadPassword('');
		}
	};

	return (
		<div className='w-full max-w-4xl mx-auto'>
			{/* Upload Settings */}
			<div className='card p-6 mb-8'>
				{/* Batch Upload Toggle */}
				<div className='flex flex-col sm:flex-row sm:items-start gap-4 mb-6'>
					<div className='flex items-center gap-3'>
						<label className='relative inline-flex items-center cursor-pointer'>
							<input
								type='checkbox'
								checked={enableBatchUpload}
								onChange={(e) => setEnableBatchUpload(e.target.checked)}
								className='sr-only'
							/>
							<div
								className={`w-11 h-6 min-w-11 min-h-6 transition-colors ${
									enableBatchUpload ? 'bg-primary-500' : 'bg-gray-300'
								}`}
							>
								<div
									className={`w-5 h-5 bg-white shadow-sm transform transition-transform ${
										enableBatchUpload ? 'translate-x-5 ml-0.5' : 'translate-x-0.5'
									} mt-0.5`}
								></div>
							</div>
							<span className='ml-3 text-sm font-medium text-gray-900'>
								Batch Upload (Combine Files)
							</span>
						</label>
					</div>
					<div className='flex items-center gap-2 text-sm text-gray-600'>
						<Archive className='w-4 h-4' />
						<span>Combine multiple files into a single ZIP archive</span>
					</div>
				</div>

				{/* Password Protection Toggle */}
				<div className='flex flex-col sm:flex-row sm:items-start gap-4 border-t border-gray-200 pt-6'>
					<div className='flex items-center gap-3'>
						<label className='relative inline-flex items-center cursor-pointer'>
							<input
								type='checkbox'
								checked={enablePasswordProtection}
								onChange={(e) => handlePasswordProtectionToggle(e.target.checked)}
								className='sr-only'
							/>
							<div
								className={`w-11 h-6 min-w-11 min-h-6 transition-colors ${
									enablePasswordProtection ? 'bg-primary-500' : 'bg-gray-300'
								}`}
							>
								<div
									className={`w-5 h-5 bg-white shadow-sm transform transition-transform ${
										enablePasswordProtection ? 'translate-x-5 ml-0.5' : 'translate-x-0.5'
									} mt-0.5`}
								></div>
							</div>
							<span className='ml-3 text-sm font-medium text-gray-900'>
								Enable Password Protection
							</span>
						</label>
					</div>
					<div className='flex items-center gap-2 text-sm text-gray-600'>
						<Lock className='w-4 h-4' />
						<span>Require password for file downloads</span>
					</div>
				</div>

				{enablePasswordProtection && (
					<div className='mt-6 pt-6 border-t border-gray-200'>
						<label className='block text-sm font-medium text-gray-900 mb-2'>
							Download Password
						</label>
						<input
							type='password'
							value={downloadPassword}
							onChange={(e) => setDownloadPassword(e.target.value)}
							placeholder='Enter password to protect downloads'
							className='input-field w-full'
							required
						/>
						<p className='text-xs text-gray-500 mt-2'>
							This password will be required to download the file
						</p>
					</div>
				)}
			</div>

			{/* Upload Area */}
			<div
				className={`border-2 border-dashed p-12 text-center transition-all duration-200 cursor-pointer ${
					isDragOver
						? 'border-primary-500 bg-primary-50'
						: 'border-gray-300 bg-gray-50 hover:bg-gray-100 hover:border-gray-400'
				}`}
				onDragOver={handleDragOver}
				onDragLeave={handleDragLeave}
				onDrop={handleDrop}
				onClick={() => document.getElementById('fileInput')?.click()}
			>
				<div className='w-16 h-16 bg-primary-500 mx-auto mb-6 flex items-center justify-center'>
					<Upload className='w-8 h-8 text-white' />
				</div>

				<h3 className='text-2xl font-medium mb-4 text-gray-900'>
					Drop files here or click to upload
				</h3>
				<p className='text-gray-600 text-lg mb-2'>Files expire automatically after 24 hours</p>
				<p className='text-gray-500 text-sm mb-8'>
					Files larger than 100MB will use chunked upload for better reliability
				</p>

				<button className='btn-primary'>Choose Files</button>

				<input id='fileInput' type='file' multiple className='hidden' onChange={handleFileInput} />
			</div>

			{/* Upload Progress */}
			{isUploading && (
				<div className='card p-8 mt-8'>
					<div className='text-center mb-6'>
						<div className='flex items-center justify-center mb-4'>
							<div className='animate-spin w-8 h-8 border-2 border-gray-200 border-t-primary-500 rounded-full' />
						</div>
						<p className='text-lg font-medium text-gray-900'>Uploading files...</p>
						<p className='text-gray-600 mt-2'>Please wait while we process your files</p>
					</div>

					{/* Individual file progress */}
					<div className='space-y-4'>
						{uploadProgress.map((progress, index) => (
							<div key={index} className='card p-6'>
								<div className='flex items-center justify-between mb-3'>
									<div className='flex items-center gap-3'>
										<div
											className={`w-10 h-10 ${
												progress.isChunkUpload ? 'bg-orange-500' : 'bg-primary-500'
											} flex items-center justify-center`}
										>
											<FileText className='w-5 h-5 text-white' />
										</div>
										<div className='flex flex-col'>
											<span className='font-medium text-gray-900'>{progress.filename}</span>
											<div className='flex items-center gap-2'>
												<span className='text-xs text-gray-600'>
													{progress.isChunkUpload
														? progress.chunkProgress ?? 'Preparing chunked upload...'
														: 'Uploading...'}
												</span>
												{progress.isChunkUpload && (
													<span className='text-xs text-orange-600 font-medium'>
														Large file - Using chunked upload
													</span>
												)}
											</div>
										</div>
									</div>
									<span className='text-sm text-gray-600'>{progress.progress}%</span>
								</div>

								{/* Progress bar */}
								<div className='w-full bg-gray-200 h-2'>
									<div
										className='bg-primary-500 h-2 transition-all duration-300'
										style={{ width: `${progress.progress}%` }}
									></div>
								</div>

								{/* Retry indicator */}
								{progress.retryCount && progress.retryCount > 0 ? (
									<div className='flex items-center gap-1 text-xs text-orange-600'>
										<RefreshCw className='w-3 h-3' />
										Retrying... (attempt {progress.retryCount})
									</div>
								) : null}

								{/* Error indicator */}
								{progress.error && (
									<div className='flex items-center gap-1 text-xs text-red-600 mt-1'>
										<AlertTriangle className='w-3 h-3' />
										{progress.error}
									</div>
								)}
							</div>
						))}
					</div>
				</div>
			)}

			{/* Upload Results */}
			{uploadResults.length > 0 && (
				<div className='card p-8 mt-8'>
					<div className='text-center mb-8'>
						<div className='w-16 h-16 bg-green-500 mx-auto mb-4 flex items-center justify-center'>
							<Check className='w-8 h-8 text-white' />
						</div>
						<h3 className='text-2xl font-medium text-gray-900 mb-2'>
							Files Uploaded Successfully!
						</h3>
						<p className='text-gray-600'>Your files are ready to share</p>
					</div>

					<div className='space-y-6'>
						{uploadResults.map((result, index) => (
							<div key={index} className='card p-6'>
								{result.success ? (
									<>
										<div className='flex items-center gap-3 mb-4'>
											<div className='w-10 h-10 bg-primary-500 flex items-center justify-center'>
												<FileText className='w-5 h-5 text-white' />
											</div>
											<div>
												<div className='font-medium text-gray-900'>{result.filename}</div>
												{result.metadata && (
													<div className='text-gray-600 text-xs'>
														{formatSize(result.metadata.size)}
														{result.metadata.compressed_size !== undefined &&
															result.metadata.compressed_size !== result.metadata.size && (
																<>
																	{' → '}
																	{formatSize(result.metadata.compressed_size)}
																	<span className='text-green-600 font-medium ml-2'>
																		(
																		{result.metadata.size > 0 &&
																		result.metadata.compressed_size >= 0
																			? Math.round(
																					(1 -
																						result.metadata.compressed_size /
																							result.metadata.size) *
																						100
																			  )
																			: 0}
																		% saved)
																	</span>
																</>
															)}
														{result.metadata.has_download_password && (
															<span className='text-primary-500 font-medium ml-2 inline-flex items-center gap-1'>
																<Lock className='w-4 h-4' />
																Password Protected
															</span>
														)}
													</div>
												)}
											</div>
										</div>

										<div className='space-y-4'>
											{/* Share URL */}
											<div className='bg-gray-50 p-4 border border-gray-200'>
												<div className='flex flex-col sm:flex-row items-center justify-between'>
													<div className='font-mono text-sm text-gray-700 break-all flex-1 sm:mr-4'>
														{window.location.origin}/f/{result.fileId}
													</div>
													<div className='flex w-full sm:w-auto gap-4 mt-4 sm:mt-0'>
														<button
															onClick={() => generateQrCode(result)}
															className='btn-secondary flex items-center gap-2 shrink-0 w-[calc(50%-0.5rem)] justify-center'
															title='Generate QR Code'
														>
															<QrCode className='w-4 h-4' />
															QR
														</button>
														<button
															onClick={() =>
																copyToClipboard(`${window.location.origin}/f/${result.fileId}`)
															}
															className='btn-secondary flex items-center gap-2 shrink-0 w-[calc(50%-0.5rem)] justify-center'
														>
															COPY
														</button>
													</div>
												</div>
											</div>

											{/* Delete Password */}
											{result.delete_password && (
												<div className='bg-red-50 border border-red-200 p-4'>
													<div className='flex items-center gap-3 mb-2'>
														<AlertTriangle className='w-5 h-5 text-red-600' />
														<span className='font-medium text-red-900'>Delete Password</span>
													</div>
													<div className='flex items-center justify-between'>
														<div className='font-mono text-sm text-red-800 break-all flex-1 mr-4'>
															{result.delete_password}
														</div>
														<button
															onClick={() => copyToClipboard(result.delete_password!)}
															className='bg-red-500 text-white px-3 py-2 hover:bg-red-600 transition-colors flex items-center gap-2 shrink-0'
														>
															COPY
														</button>
													</div>
													<p className='text-xs text-red-700 mt-2'>
														Save this password! You'll need it to delete the file.
													</p>
												</div>
											)}
										</div>
									</>
								) : (
									<div className='flex items-center gap-3 bg-red-50 border border-red-200 p-4'>
										<div className='w-10 h-10 bg-red-500 flex items-center justify-center'>
											<AlertTriangle className='w-5 h-5 text-white' />
										</div>
										<div>
											<div className='font-medium text-red-900'>{result.filename}</div>
											<div className='text-red-700 text-sm'>{result.error}</div>
										</div>
									</div>
								)}
							</div>
						))}
					</div>

					<div className='text-center mt-8 p-4 bg-primary-50 border border-primary-200'>
						<p className='text-primary-700 font-medium'>
							Share these URLs with anyone. Files will expire in 24 hours.
						</p>
					</div>
				</div>
			)}

			{/* QR Code Modal */}
			{showQrModal && selectedFileForQr && (
				<div className='fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4'>
					<div className='bg-white shadow-angular-xl w-full max-w-md'>
						<div className='p-6'>
							<div className='flex items-center justify-between mb-4'>
								<h3 className='text-xl font-medium text-gray-900'>QR Code</h3>
								<button
									onClick={() => setShowQrModal(false)}
									className='text-gray-400 hover:text-gray-600'
								>
									<X className='w-6 h-6' />
								</button>
							</div>
							<p className='text-gray-600 mb-4'>
								Scan this QR code to access "{selectedFileForQr.filename}"
							</p>
							<div className='flex justify-center mb-6'>
								{qrCodeUrl && (
									<img src={qrCodeUrl} alt='QR Code' className='w-64 h-64 border border-gray-200' />
								)}
							</div>
							<div className='flex gap-3'>
								<button onClick={() => setShowQrModal(false)} className='btn-secondary flex-1'>
									Close
								</button>
								<button
									onClick={() => {
										if (qrCodeUrl) {
											const link = document.createElement('a');
											link.download = `${selectedFileForQr.filename}_qr.png`;
											link.href = qrCodeUrl;
											link.click();
										}
									}}
									className='btn-primary flex-1'
								>
									Download QR
								</button>
							</div>
						</div>
					</div>
				</div>
			)}
		</div>
	);
};

export default UploadArea;
