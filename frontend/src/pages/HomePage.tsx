import React from 'react';
import UploadArea from '../components/UploadArea';
import Footer from '../components/Footer';
import { CloudUpload, GalleryVerticalEnd, Clock4, LockKeyhole, Eye } from 'lucide-react';

const HomePage: React.FC = () => {
	return (
		<div className='min-h-screen bg-gray-25'>
			{/* Main Content */}
			<main className='max-w-6xl mx-auto px-6 py-12'>
				{/* Hero Section */}
				<div className='text-center mb-16'>
					<div className='w-16 h-16 bg-primary-500 mx-auto mb-8 flex items-center justify-center'>
						<CloudUpload className='w-8 h-8 text-white' />
					</div>

					<h1 className='text-5xl md:text-6xl font-light text-gray-900 mb-6 tracking-tight'>
						Store and share files
					</h1>

					<p className='text-xl text-gray-600 mb-8 max-w-2xl mx-auto font-light'>
						Secure file storage with advanced compression. Upload, share, and manage your files with
						ease.
					</p>

					<div className='flex items-center justify-center gap-8 text-sm text-gray-500 mb-12'>
						<div className='flex items-center gap-2'>
							<div className='w-2 h-2 bg-primary-500'></div>
							<span>10GB max size</span>
						</div>
						<div className='flex items-center gap-2'>
							<div className='w-2 h-2 bg-primary-500'></div>
							<span>24 hour expiry</span>
						</div>
						<div className='flex items-center gap-2'>
							<div className='w-2 h-2 bg-primary-500'></div>
							<span>Password protection</span>
						</div>
					</div>
				</div>

				{/* Upload Area */}
				<div className='mb-20'>
					<UploadArea />
				</div>

				{/* Features Grid */}
				<div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-8'>
					<div className='text-center'>
						<div className='w-12 h-12 bg-gray-100 mx-auto mb-4 flex items-center justify-center'>
							<GalleryVerticalEnd className='w-6 h-6 text-gray-700' />
						</div>
						<h3 className='font-medium text-lg mb-2 text-gray-900'>Smart Compression</h3>
						<p className='text-gray-600 text-sm leading-relaxed'>
							Automatic algorithm selection for optimal file size reduction
						</p>
					</div>

					<div className='text-center'>
						<div className='w-12 h-12 bg-gray-100 mx-auto mb-4 flex items-center justify-center'>
							<Clock4 className='w-6 h-6 text-gray-700' />
						</div>
						<h3 className='font-medium text-lg mb-2 text-gray-900'>Auto Expiry</h3>
						<p className='text-gray-600 text-sm leading-relaxed'>
							Files automatically deleted after 24 hours for security
						</p>
					</div>

					<div className='text-center'>
						<div className='w-12 h-12 bg-gray-100 mx-auto mb-4 flex items-center justify-center'>
							<LockKeyhole className='w-6 h-6 text-gray-700' />
						</div>
						<h3 className='font-medium text-lg mb-2 text-gray-900'>Secure Sharing</h3>
						<p className='text-gray-600 text-sm leading-relaxed'>
							UUID-based URLs with optional password protection
						</p>
					</div>

					<div className='text-center'>
						<div className='w-12 h-12 bg-gray-100 mx-auto mb-4 flex items-center justify-center'>
							<Eye className='w-6 h-6 text-gray-700' />
						</div>
						<h3 className='font-medium text-lg mb-2 text-gray-900'>File Preview</h3>
						<p className='text-gray-600 text-sm leading-relaxed'>
							View images, videos, and documents directly in browser
						</p>
					</div>
				</div>
			</main>

			{/* Footer */}
			<Footer />
		</div>
	);
};

export default HomePage;
