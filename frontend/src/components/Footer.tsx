export default function Footer() {
	return (
		<footer className='bg-white border-t border-gray-200 mt-20'>
			<div className='max-w-6xl mx-auto px-6 py-8'>
				<div className='text-center text-sm text-gray-500 space-y-4'>
					<p className='leading-relaxed'>
						Secure file storage with automatic compression and expiry
						<br />
						&copy; {new Date().getFullYear()} All rights reserved by{' '}
						<a
							href='https://x.com/minagishl'
							className='text-gray-500 hover:text-gray-700 transition-colors'
							target='_blank'
							rel='noopener noreferrer'
						>
							@minagishl
						</a>
					</p>

					<div className='flex items-center justify-center gap-6'>
						<a href='/terms' className='text-gray-500 hover:text-gray-700 transition-colors'>
							Terms of Service
						</a>
						<div className='w-px h-4 bg-gray-300'></div>
						<a href='/privacy' className='text-gray-500 hover:text-gray-700 transition-colors'>
							Privacy Policy
						</a>
						<div className='w-px h-4 bg-gray-300'></div>
						<a
							href='https://x.com/minagishl'
							className='text-gray-500 hover:text-gray-700 transition-colors'
							target='_blank'
							rel='noopener noreferrer'
						>
							Contact
						</a>
					</div>
				</div>
			</div>
		</footer>
	);
}
