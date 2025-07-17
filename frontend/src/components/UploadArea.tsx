import React, { useState, useCallback } from 'react';
import { Lock } from 'lucide-react';
import { uploadFile } from '../utils/api';
import { formatSize } from '../utils/format';
import { UploadResult } from '../types';

interface UploadAreaProps {
	onUploadComplete?: (results: UploadResult[]) => void;
}

const UploadArea: React.FC<UploadAreaProps> = ({ onUploadComplete }) => {
	const [isDragOver, setIsDragOver] = useState(false);
	const [isUploading, setIsUploading] = useState(false);
	const [uploadResults, setUploadResults] = useState<UploadResult[]>([]);
	const [downloadPassword, setDownloadPassword] = useState('');
	const [enablePasswordProtection, setEnablePasswordProtection] = useState(false);

	const handleFiles = useCallback(async (files: File[]) => {
		if (files.length === 0) return;

		// Validate password if protection is enabled
		if (enablePasswordProtection && !downloadPassword.trim()) {
			alert('Please enter a password to protect your files');
			return;
		}

		setIsUploading(true);
		setUploadResults([]);


		const uploadPromises = files.map((file) =>
			uploadFile(file, enablePasswordProtection ? downloadPassword : undefined)
		);
		const results = await Promise.all(uploadPromises);

		setUploadResults(results);
		setIsUploading(false);

		if (onUploadComplete) {
			onUploadComplete(results);
		}
	}, [enablePasswordProtection, downloadPassword, onUploadComplete]);

	const handleDragOver = useCallback((e: React.DragEvent) => {
		e.preventDefault();
		setIsDragOver(true);
	}, []);

	const handleDragLeave = useCallback((e: React.DragEvent) => {
		e.preventDefault();
		setIsDragOver(false);
	}, []);

	const handleDrop = useCallback((e: React.DragEvent) => {
		e.preventDefault();
		setIsDragOver(false);
		const files = Array.from(e.dataTransfer.files);
		handleFiles(files);
	}, [handleFiles]);

	const handleFileInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
		const files = Array.from(e.target.files || []);
		handleFiles(files);
	}, [handleFiles]);

	const copyToClipboard = (text: string) => {
		navigator.clipboard.writeText(text);
		// Add a visual feedback here later
	};

	const handlePasswordProtectionToggle = (enabled: boolean) => {
		setEnablePasswordProtection(enabled);
		if (!enabled) {
			setDownloadPassword('');
		}
	};

	return (
		<div className='w-full max-w-3xl mx-auto'>
			{/* Password Protection Settings */}
			<div className='mb-6 bg-white/10 backdrop-blur-sm border border-white/20 rounded-2xl p-6'>
				<div className='flex items-start gap-4'>
					<div className='flex items-center gap-3'>
						<label className='relative inline-flex items-center cursor-pointer'>
							<input
								type='checkbox'
								checked={enablePasswordProtection}
								onChange={(e) => handlePasswordProtectionToggle(e.target.checked)}
								className='sr-only'
							/>
							<div
								className={`w-11 h-6 rounded-full transition-colors ${
									enablePasswordProtection ? 'bg-blue-500' : 'bg-gray-600'
								}`}
							>
								<div
									className={`w-5 h-5 bg-white rounded-full shadow-md transform transition-transform ${
										enablePasswordProtection ? 'translate-x-5' : 'translate-x-0.5'
									} mt-0.5`}
								></div>
							</div>
							<span className='ml-3 text-sm font-medium text-white'>
								Enable Password Protection
							</span>
						</label>
					</div>
					<div className='flex items-center gap-2 text-sm text-gray-300'>
						<svg className='w-4 h-4' fill='none' stroke='currentColor' viewBox='0 0 24 24'>
							<path
								strokeLinecap='round'
								strokeLinejoin='round'
								strokeWidth={2}
								d='M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z'
							/>
						</svg>
						<span>Require password for file downloads</span>
					</div>
				</div>

				{enablePasswordProtection && (
					<div className='mt-4 pt-4 border-t border-white/20'>
						<label className='block text-sm font-medium text-white mb-2'>Download Password</label>
						<input
							type='password'
							value={downloadPassword}
							onChange={(e) => setDownloadPassword(e.target.value)}
							placeholder='Enter password to protect downloads'
							className='w-full px-4 py-2 bg-white/10 border border-white/20 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500'
							required
						/>
						<p className='text-xs text-gray-400 mt-1'>
							This password will be required to download the file
						</p>
					</div>
				)}
			</div>

			<div
				className={`
          relative border-2 border-dashed rounded-3xl p-12 text-center transition-all duration-300 cursor-pointer
          ${
						isDragOver
							? 'border-blue-400 bg-blue-500/20 scale-105'
							: 'border-white/20 bg-white/5 hover:bg-white/10'
					} backdrop-blur-sm group
        `}
				onDragOver={handleDragOver}
				onDragLeave={handleDragLeave}
				onDrop={handleDrop}
				onClick={() => document.getElementById('fileInput')?.click()}
			>
				<div className='absolute inset-0 bg-gradient-to-r from-blue-500/10 to-purple-500/10 rounded-3xl opacity-0 group-hover:opacity-100 transition-opacity duration-300'></div>

				<div className='relative z-10'>
					<div className='w-24 h-24 bg-gradient-to-r from-blue-500 to-purple-600 rounded-2xl flex items-center justify-center mx-auto mb-6 group-hover:scale-110 transition-transform duration-300'>
						<svg
							className='w-12 h-12 text-white'
							fill='none'
							stroke='currentColor'
							viewBox='0 0 24 24'
						>
							<path
								strokeLinecap='round'
								strokeLinejoin='round'
								strokeWidth={2}
								d='M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12'
							/>
						</svg>
					</div>

					<h3 className='text-3xl font-semibold mb-4 text-white'>
						Drop files here or click to upload
					</h3>
					<p className='text-gray-300 text-lg mb-8'>Files expire automatically after 24 hours</p>

					<button className='bg-gradient-to-r from-blue-500 to-purple-600 text-white px-8 py-4 rounded-2xl font-semibold hover:shadow-2xl transform hover:-translate-y-1 transition-all duration-300 text-lg'>
						Choose Files
					</button>
				</div>

				<input id='fileInput' type='file' multiple className='hidden' onChange={handleFileInput} />
			</div>

			{isUploading && (
				<div className='mt-8 bg-white/10 backdrop-blur-sm rounded-3xl p-8 text-center border border-white/20'>
					<div className='flex items-center justify-center mb-4'>
						<div className='animate-spin w-12 h-12 border-4 border-white/20 border-t-white rounded-full'></div>
					</div>
					<p className='text-xl font-medium text-white'>Uploading and compressing...</p>
					<p className='text-gray-300 mt-2'>Please wait while we process your files</p>
				</div>
			)}

			{uploadResults.length > 0 && (
				<div className='mt-8 bg-white/10 backdrop-blur-sm rounded-3xl p-8 border border-white/20'>
					<div className='text-center mb-6'>
						<div className='w-16 h-16 bg-gradient-to-r from-green-500 to-emerald-500 rounded-2xl flex items-center justify-center mx-auto mb-4'>
							<svg
								className='w-8 h-8 text-white'
								fill='none'
								stroke='currentColor'
								viewBox='0 0 24 24'
							>
								<path
									strokeLinecap='round'
									strokeLinejoin='round'
									strokeWidth={2}
									d='M5 13l4 4L19 7'
								/>
							</svg>
						</div>
						<h3 className='text-2xl font-semibold text-white mb-2'>Files Shared Successfully!</h3>
						<p className='text-gray-300'>Your files are ready to share</p>
					</div>

					<div className='space-y-4'>
						{uploadResults.map((result, index) => (
							<div key={index} className='bg-white/10 rounded-2xl p-6 border border-white/20'>
								{result.success ? (
									<>
										<div className='flex items-center gap-3 mb-4'>
											<div className='w-10 h-10 bg-gradient-to-r from-blue-500 to-purple-600 rounded-xl flex items-center justify-center'>
												<svg
													className='w-5 h-5 text-white'
													fill='none'
													stroke='currentColor'
													viewBox='0 0 24 24'
												>
													<path
														strokeLinecap='round'
														strokeLinejoin='round'
														strokeWidth={2}
														d='M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z'
													/>
												</svg>
											</div>
											<div>
												<div className='font-semibold text-white text-lg'>{result.filename}</div>
												{result.metadata && (
													<div className='text-gray-300 text-sm'>
														{formatSize(result.metadata.size)} →{' '}
														{formatSize(result.metadata.compressed_size)}
														<span className='text-green-400 font-medium'>
															(
															{Math.round(
																(1 - result.metadata.compressed_size / result.metadata.size) * 100
															)}
															% saved)
														</span>
														{result.metadata.has_download_password && (
															<span className='text-blue-400 font-medium ml-2 flex items-center gap-1'>
																<Lock className='w-4 h-4' />
																Password Protected
															</span>
														)}
													</div>
												)}
											</div>
										</div>

										<div className='space-y-3'>
											{/* Share URL */}
											<div className='bg-black/30 rounded-xl p-4 border border-white/10'>
												<div className='flex items-center justify-between'>
													<div className='font-mono text-sm text-gray-300 break-all flex-1 mr-4'>
														{window.location.origin}/f/{result.fileId}
													</div>
													<button
														onClick={() =>
															copyToClipboard(`${window.location.origin}/f/${result.fileId}`)
														}
														className='bg-gradient-to-r from-blue-500 to-purple-600 text-white px-4 py-2 rounded-lg font-medium hover:shadow-lg transition-all duration-300 flex items-center gap-2 shrink-0'
													>
														<svg
															className='w-4 h-4'
															fill='none'
															stroke='currentColor'
															viewBox='0 0 24 24'
														>
															<path
																strokeLinecap='round'
																strokeLinejoin='round'
																strokeWidth={2}
																d='M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3'
															/>
														</svg>
														Copy
													</button>
												</div>
											</div>

											{/* Delete Password */}
											{result.delete_password && (
												<div className='bg-red-500/20 border border-red-500/30 rounded-xl p-4'>
													<div className='flex items-center gap-3 mb-2'>
														<svg
															className='w-5 h-5 text-red-400'
															fill='none'
															stroke='currentColor'
															viewBox='0 0 24 24'
														>
															<path
																strokeLinecap='round'
																strokeLinejoin='round'
																strokeWidth={2}
																d='M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z'
															/>
														</svg>
														<span className='font-medium text-red-300'>Delete Password</span>
													</div>
													<div className='flex items-center justify-between'>
														<div className='font-mono text-sm text-red-200 break-all flex-1 mr-4'>
															{result.delete_password}
														</div>
														<button
															onClick={() => copyToClipboard(result.delete_password!)}
															className='bg-red-500 text-white px-3 py-1 rounded-lg font-medium hover:bg-red-600 transition-colors flex items-center gap-2 shrink-0'
														>
															<svg
																className='w-3 h-3'
																fill='none'
																stroke='currentColor'
																viewBox='0 0 24 24'
															>
																<path
																	strokeLinecap='round'
																	strokeLinejoin='round'
																	strokeWidth={2}
																	d='M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3'
																/>
															</svg>
															Copy
														</button>
													</div>
													<p className='text-xs text-red-400 mt-2'>
														Save this password! You'll need it to delete the file.
													</p>
												</div>
											)}
										</div>
									</>
								) : (
									<div className='flex items-center gap-3 bg-red-500/20 border border-red-500/30 rounded-xl p-4'>
										<div className='w-10 h-10 bg-red-500 rounded-xl flex items-center justify-center'>
											<svg
												className='w-5 h-5 text-white'
												fill='none'
												stroke='currentColor'
												viewBox='0 0 24 24'
											>
												<path
													strokeLinecap='round'
													strokeLinejoin='round'
													strokeWidth={2}
													d='M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z'
												/>
											</svg>
										</div>
										<div>
											<div className='font-semibold text-red-300'>{result.filename}</div>
											<div className='text-red-400 text-sm'>{result.error}</div>
										</div>
									</div>
								)}
							</div>
						))}
					</div>

					<div className='text-center mt-6 p-4 bg-blue-500/20 border border-blue-500/30 rounded-xl'>
						<p className='text-blue-300 font-medium'>
							Share these URLs with anyone. Files will expire in 24 hours.
						</p>
					</div>
				</div>
			)}
		</div>
	);
};

export default UploadArea;
