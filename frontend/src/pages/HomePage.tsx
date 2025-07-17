import React from 'react';
import UploadArea from '../components/UploadArea';

const HomePage: React.FC = () => {
	return (
		<div className='min-h-screen bg-gradient-to-br from-slate-900 via-purple-900 to-slate-900 text-white'>
			{/* Background Pattern */}
			<div className='absolute inset-0 bg-[radial-gradient(circle_500px_at_50%_200px,#3b82f6,transparent)]' />

			<div className='relative z-10 flex items-center justify-center min-h-screen p-4'>
				<div className='w-full max-w-6xl mx-auto'>
					{/* Hero Section */}
					<div className='text-center mb-16'>
						<div className='inline-flex items-center justify-center w-20 h-20 bg-gradient-to-r from-blue-500 to-purple-600 rounded-2xl mb-8 shadow-2xl'>
							<svg
								className='w-10 h-10 text-white'
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

						<h1 className='text-6xl md:text-7xl font-bold mb-6 bg-gradient-to-r from-white via-blue-100 to-purple-100 bg-clip-text text-transparent'>
							ONE
						</h1>

						<p className='text-xl md:text-2xl text-gray-300 mb-4 max-w-2xl mx-auto'>
							Blazing-fast file storage with advanced compression
						</p>

						<div className='flex items-center justify-center gap-6 text-sm text-gray-400 mb-12'>
							<div className='flex items-center gap-2'>
								<div className='w-2 h-2 bg-green-500 rounded-full animate-pulse'></div>
								<span>100MB Max</span>
							</div>
							<div className='flex items-center gap-2'>
								<div className='w-2 h-2 bg-blue-500 rounded-full animate-pulse'></div>
								<span>24h Expiry</span>
							</div>
							<div className='flex items-center gap-2'>
								<div className='w-2 h-2 bg-purple-500 rounded-full animate-pulse'></div>
								<span>Secure URLs</span>
							</div>
						</div>
					</div>

					{/* Upload Area */}
					<div className='mb-16'>
						<UploadArea />
					</div>

					{/* Features Grid */}
					<div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6'>
						<div className='group bg-white/5 backdrop-blur-sm border border-white/10 rounded-2xl p-6 text-center hover:bg-white/10 transition-all duration-300 hover:scale-105'>
							<div className='w-12 h-12 bg-gradient-to-r from-blue-500 to-cyan-500 rounded-xl flex items-center justify-center mx-auto mb-4 group-hover:rotate-12 transition-transform duration-300'>
								<svg
									className='w-6 h-6 text-white'
									fill='none'
									stroke='currentColor'
									viewBox='0 0 24 24'
								>
									<path
										strokeLinecap='round'
										strokeLinejoin='round'
										strokeWidth={2}
										d='M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10'
									/>
								</svg>
							</div>
							<h3 className='font-semibold text-lg mb-2'>Smart Compression</h3>
							<p className='text-gray-400 text-sm'>
								Automatic algorithm selection for optimal compression
							</p>
						</div>

						<div className='group bg-white/5 backdrop-blur-sm border border-white/10 rounded-2xl p-6 text-center hover:bg-white/10 transition-all duration-300 hover:scale-105'>
							<div className='w-12 h-12 bg-gradient-to-r from-purple-500 to-pink-500 rounded-xl flex items-center justify-center mx-auto mb-4 group-hover:rotate-12 transition-transform duration-300'>
								<svg
									className='w-6 h-6 text-white'
									fill='none'
									stroke='currentColor'
									viewBox='0 0 24 24'
								>
									<path
										strokeLinecap='round'
										strokeLinejoin='round'
										strokeWidth={2}
										d='M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z'
									/>
								</svg>
							</div>
							<h3 className='font-semibold text-lg mb-2'>Auto Expiry</h3>
							<p className='text-gray-400 text-sm'>Files automatically deleted after 24 hours</p>
						</div>

						<div className='group bg-white/5 backdrop-blur-sm border border-white/10 rounded-2xl p-6 text-center hover:bg-white/10 transition-all duration-300 hover:scale-105'>
							<div className='w-12 h-12 bg-gradient-to-r from-green-500 to-emerald-500 rounded-xl flex items-center justify-center mx-auto mb-4 group-hover:rotate-12 transition-transform duration-300'>
								<svg
									className='w-6 h-6 text-white'
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
							</div>
							<h3 className='font-semibold text-lg mb-2'>Secure Sharing</h3>
							<p className='text-gray-400 text-sm'>UUID-based URLs for private file sharing</p>
						</div>

						<div className='group bg-white/5 backdrop-blur-sm border border-white/10 rounded-2xl p-6 text-center hover:bg-white/10 transition-all duration-300 hover:scale-105'>
							<div className='w-12 h-12 bg-gradient-to-r from-orange-500 to-red-500 rounded-xl flex items-center justify-center mx-auto mb-4 group-hover:rotate-12 transition-transform duration-300'>
								<svg
									className='w-6 h-6 text-white'
									fill='none'
									stroke='currentColor'
									viewBox='0 0 24 24'
								>
									<path
										strokeLinecap='round'
										strokeLinejoin='round'
										strokeWidth={2}
										d='M15 12a3 3 0 11-6 0 3 3 0 016 0z'
									/>
									<path
										strokeLinecap='round'
										strokeLinejoin='round'
										strokeWidth={2}
										d='M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z'
									/>
								</svg>
							</div>
							<h3 className='font-semibold text-lg mb-2'>Preview Files</h3>
							<p className='text-gray-400 text-sm'>View images, videos, and documents in browser</p>
						</div>
					</div>
				</div>
			</div>
		</div>
	);
};

export default HomePage;
