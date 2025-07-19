import React from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, Shield, TriangleAlert } from 'lucide-react';
import Footer from '../components/Footer';
import Button from '../components/Button';

const TermsOfServicePage: React.FC = () => {
	const navigate = useNavigate();

	return (
		<div className='min-h-screen bg-gray-25'>
			{/* Header */}
			<header className='bg-white border-b border-gray-200'>
				<div className='max-w-6xl mx-auto px-6 py-4'>
					<div className='flex items-center gap-4'>
						<Button
							onClick={() => navigate('/')}
							variant='secondary'
							size='md'
							icon={ArrowLeft}
							className='text-gray-600 hover:text-gray-900 border-none bg-transparent'
						>
							Back to Home
						</Button>
						<div className='h-6 w-px bg-gray-300'></div>
						<h1 className='text-xl font-medium text-gray-900'>Terms of Service</h1>
					</div>
				</div>
			</header>

			{/* Main Content */}
			<main className='max-w-4xl mx-auto px-6 py-12'>
				{/* Hero Section */}
				<div className='text-center mb-12'>
					<div className='w-16 h-16 bg-primary-500 mx-auto mb-6 flex items-center justify-center'>
						<Shield className='w-8 h-8 text-white' />
					</div>
					<h1 className='text-4xl font-light text-gray-900 mb-4 tracking-tight'>
						Terms of Service
					</h1>
					<p className='text-lg text-gray-600 font-light'>Last updated: July 2025</p>
				</div>

				{/* Content */}
				<div className='card'>
					<div className='p-8 space-y-8'>
						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>1. Acceptance of Terms</h2>
							<p className='text-gray-700 leading-relaxed'>
								By accessing and using this file storage service, you accept and agree to be bound
								by the terms and provision of this agreement. If you do not agree to abide by the
								above, please do not use this service.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>2. Service Description</h2>
							<p className='text-gray-700 leading-relaxed mb-4'>
								Our service provides secure file storage with automatic compression and expiry
								features. We offer:
							</p>
							<ul className='list-disc list-inside text-gray-700 space-y-2 ml-4'>
								<li>File storage with up to 10GB capacity per file</li>
								<li>Automatic file compression to reduce storage size</li>
								<li>24-hour automatic file expiry for security</li>
								<li>Optional password protection for downloads and deletions</li>
								<li>UUID-based secure file URLs</li>
							</ul>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>3. Prohibited Uses</h2>
							<p className='text-gray-700 leading-relaxed mb-4'>
								You may not use our service for any unlawful purposes or to conduct any unlawful
								activity, including but not limited to:
							</p>
							<div className='bg-red-50 border border-red-200 p-4 mb-4'>
								<div className='flex items-center gap-3 mb-2'>
									<TriangleAlert className='w-5 h-5 text-red-500' />
									<p className='text-red-800 font-medium'>Strictly Prohibited</p>
								</div>
								<ul className='list-disc list-inside text-red-700 space-y-1 ml-4'>
									<li>Criminal activities or illegal content distribution</li>
									<li>Malware, viruses, or any malicious software</li>
									<li>Content that violates intellectual property rights</li>
									<li>Harassment, defamation, or threatening material</li>
									<li>Spam or unauthorized commercial content</li>
									<li>Content that violates privacy rights of others</li>
								</ul>
							</div>
							<p className='text-gray-700 leading-relaxed'>
								We reserve the right to terminate access to users who violate these terms
								immediately and without notice.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>
								4. File Retention and Security
							</h2>
							<p className='text-gray-700 leading-relaxed mb-4'>
								Files uploaded to our service are automatically deleted after 24 hours. We implement
								security measures to protect your files during storage, but you acknowledge that:
							</p>
							<ul className='list-disc list-inside text-gray-700 space-y-2 ml-4'>
								<li>No electronic storage system is 100% secure</li>
								<li>You are responsible for maintaining backup copies of important files</li>
								<li>We cannot guarantee file recovery after the 24-hour expiry period</li>
								<li>Password protection is optional and user-controlled</li>
							</ul>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>
								5. Privacy and Data Protection
							</h2>
							<p className='text-gray-700 leading-relaxed'>
								We respect your privacy and handle your data in accordance with our Privacy Policy.
								By using our service, you consent to the collection and use of information as
								described in our Privacy Policy.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>
								6. Limitation of Liability
							</h2>
							<p className='text-gray-700 leading-relaxed'>
								In no event shall we be liable for any indirect, incidental, special, consequential,
								or punitive damages, including without limitation, loss of profits, data, use,
								goodwill, or other intangible losses, resulting from your use of the service.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>7. Service Availability</h2>
							<p className='text-gray-700 leading-relaxed'>
								We strive to maintain service availability but do not guarantee uninterrupted
								access. The service may be temporarily unavailable due to maintenance, updates, or
								technical issues.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>8. Modifications to Terms</h2>
							<p className='text-gray-700 leading-relaxed'>
								We reserve the right to modify these terms at any time. Changes will be effective
								immediately upon posting. Your continued use of the service constitutes acceptance
								of the modified terms.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>9. Termination</h2>
							<p className='text-gray-700 leading-relaxed'>
								We may terminate or suspend access to our service immediately, without prior notice
								or liability, for any reason whatsoever, including without limitation if you breach
								the Terms.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>10. Contact Information</h2>
							<p className='text-gray-700 leading-relaxed'>
								If you have any questions about these Terms of Service, please contact us on X
								(Twitter)
								<a
									href='https://x.com/minagishl'
									className='text-primary-600 hover:text-primary-700 ml-1'
									target='_blank'
									rel='noopener noreferrer'
								>
									@minagishl
								</a>
								.
							</p>
						</section>
					</div>
				</div>
			</main>

			{/* Footer */}
			<Footer />
		</div>
	);
};

export default TermsOfServicePage;
