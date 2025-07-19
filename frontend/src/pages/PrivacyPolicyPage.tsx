import React from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, UserCheck, Lock } from 'lucide-react';
import Footer from '../components/Footer';
import Button from '../components/Button';

const PrivacyPolicyPage: React.FC = () => {
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
						<h1 className='text-xl font-medium text-gray-900'>Privacy Policy</h1>
					</div>
				</div>
			</header>

			{/* Main Content */}
			<main className='max-w-4xl mx-auto px-6 py-12'>
				{/* Hero Section */}
				<div className='text-center mb-12'>
					<div className='w-16 h-16 bg-primary-500 mx-auto mb-6 flex items-center justify-center'>
						<UserCheck className='w-8 h-8 text-white' />
					</div>
					<h1 className='text-4xl font-light text-gray-900 mb-4 tracking-tight'>Privacy Policy</h1>
					<p className='text-lg text-gray-600 font-light'>Last updated: July 2025</p>
				</div>

				{/* Content */}
				<div className='card'>
					<div className='p-8 space-y-8'>
						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>1. Introduction</h2>
							<p className='text-gray-700 leading-relaxed'>
								This Privacy Policy describes how we collect, use, and protect your information when
								you use our file storage service. We are committed to protecting your privacy and
								handling your data transparently.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>2. Information We Collect</h2>
							<div className='space-y-4'>
								<div>
									<h3 className='text-lg font-medium text-gray-900 mb-2'>2.1 File Information</h3>
									<p className='text-gray-700 leading-relaxed mb-2'>
										When you upload files to our service, we collect:
									</p>
									<ul className='list-disc list-inside text-gray-700 space-y-1 ml-4'>
										<li>File name and file size</li>
										<li>File type (MIME type)</li>
										<li>Upload timestamp</li>
										<li>Compression algorithm used</li>
										<li>Optional passwords you set for download/deletion protection</li>
									</ul>
								</div>

								<div>
									<h3 className='text-lg font-medium text-gray-900 mb-2'>
										2.2 Technical Information
									</h3>
									<p className='text-gray-700 leading-relaxed mb-2'>
										We automatically collect certain technical information:
									</p>
									<ul className='list-disc list-inside text-gray-700 space-y-1 ml-4'>
										<li>IP address (for security and abuse prevention)</li>
										<li>Browser type and version</li>
										<li>Operating system</li>
										<li>Access times and dates</li>
										<li>Referring website addresses</li>
									</ul>
								</div>
							</div>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>
								3. How We Use Your Information
							</h2>
							<p className='text-gray-700 leading-relaxed mb-4'>
								We use the information we collect for the following purposes:
							</p>
							<ul className='list-disc list-inside text-gray-700 space-y-2 ml-4'>
								<li>To provide and maintain our file storage service</li>
								<li>To process file uploads, compression, and downloads</li>
								<li>To enforce our automatic 24-hour file expiry policy</li>
								<li>To prevent abuse and ensure service security</li>
								<li>To improve our service and user experience</li>
								<li>To comply with legal requirements</li>
							</ul>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>
								4. Data Storage and Security
							</h2>
							<div className='space-y-4'>
								<div className='bg-blue-50 border border-blue-200 p-4'>
									<div className='flex items-center gap-3 mb-2'>
										<Lock className='w-5 h-5 text-blue-500' />
										<p className='text-blue-800 font-medium'>Security Measures</p>
									</div>
									<ul className='list-disc list-inside text-blue-700 space-y-1 ml-4'>
										<li>All files are automatically deleted after 24 hours</li>
										<li>UUID-based file URLs for security</li>
										<li>Optional password protection</li>
										<li>Secure server infrastructure</li>
										<li>Regular security monitoring</li>
									</ul>
								</div>
								<p className='text-gray-700 leading-relaxed'>
									While we implement reasonable security measures, no method of transmission over
									the internet or electronic storage is 100% secure. We cannot guarantee absolute
									security of your information.
								</p>
							</div>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>5. Data Retention</h2>
							<div className='space-y-4'>
								<div>
									<h3 className='text-lg font-medium text-gray-900 mb-2'>5.1 File Data</h3>
									<p className='text-gray-700 leading-relaxed'>
										All uploaded files are automatically and permanently deleted after 24 hours.
										This is a core feature of our service designed to protect your privacy.
									</p>
								</div>

								<div>
									<h3 className='text-lg font-medium text-gray-900 mb-2'>5.2 Technical Logs</h3>
									<p className='text-gray-700 leading-relaxed'>
										Technical logs (IP addresses, access times) are retained for up to 30 days for
										security and abuse prevention purposes, then automatically deleted.
									</p>
								</div>
							</div>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>6. Information Sharing</h2>
							<p className='text-gray-700 leading-relaxed mb-4'>
								We do not sell, trade, or rent your personal information to third parties. We may
								share information only in the following circumstances:
							</p>
							<ul className='list-disc list-inside text-gray-700 space-y-2 ml-4'>
								<li>When required by law or court order</li>
								<li>To protect our rights, property, or safety</li>
								<li>To prevent illegal activities or policy violations</li>
								<li>In connection with a business transfer (merger, acquisition, etc.)</li>
							</ul>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>7. Cookies and Tracking</h2>
							<p className='text-gray-700 leading-relaxed'>
								Our service uses minimal cookies and tracking technologies. We use session cookies
								to maintain service functionality and do not use tracking cookies for advertising
								purposes.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>8. Your Rights</h2>
							<p className='text-gray-700 leading-relaxed mb-4'>
								You have the following rights regarding your information:
							</p>
							<ul className='list-disc list-inside text-gray-700 space-y-2 ml-4'>
								<li>Right to delete your files at any time before the 24-hour expiry</li>
								<li>Right to access information about files you've uploaded</li>
								<li>Right to request deletion of technical logs</li>
								<li>Right to contact us about privacy concerns</li>
							</ul>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>9. International Users</h2>
							<p className='text-gray-700 leading-relaxed'>
								Our service may be accessed from around the world. By using our service, you consent
								to the processing of your information in the country where our servers are located,
								which may have different privacy laws than your country.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>10. Children's Privacy</h2>
							<p className='text-gray-700 leading-relaxed'>
								Our service is not intended for children under 13 years of age. We do not knowingly
								collect personal information from children under 13. If you believe we have
								collected information from a child under 13, please contact us immediately.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>
								11. Changes to This Policy
							</h2>
							<p className='text-gray-700 leading-relaxed'>
								We may update this Privacy Policy from time to time. We will notify users of any
								material changes by posting the new Privacy Policy on this page and updating the
								"Last updated" date.
							</p>
						</section>

						<section>
							<h2 className='text-2xl font-medium text-gray-900 mb-4'>12. Contact Us</h2>
							<p className='text-gray-700 leading-relaxed'>
								If you have any questions about this Privacy Policy or our privacy practices, please
								contact us on X (Twitter)
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

export default PrivacyPolicyPage;
