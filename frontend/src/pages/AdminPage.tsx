import React, { useState } from 'react';
import { formatSize, formatDate } from '../utils/format';
import {
	Shield,
	RefreshCw,
	LogOut,
	Trash2,
	Clock,
	Database,
	Archive,
	Copy,
	Eye,
	Lock,
} from 'lucide-react';
import Footer from '../components/Footer';
import Button from '../components/Button';
import Input from '../components/Input';

interface FileData {
	file_id: string;
	filename: string;
	size: number;
	original_size: number;
	uploaded_at: string;
	expires_at: string;
	storage_type: string; // "postgresql" or "disk"
	storage_path?: string; // disk path if applicable
	compressed: boolean;
	compression?: string; // compression algorithm
	mime_type?: string;
	has_password: boolean;
}

interface AdminResponse {
	message: string;
	count: number;
	files: FileData[];
}

const AdminPage: React.FC = () => {
	const [isAuthenticated, setIsAuthenticated] = useState(false);
	const [password, setPassword] = useState('');
	const [adminPassword, setAdminPassword] = useState('');
	const [files, setFiles] = useState<FileData[]>([]);
	const [loading, setLoading] = useState(false);
	const [error, setError] = useState('');
	const [message, setMessage] = useState('');
	const [adminToken, setAdminToken] = useState('');

	const authenticate = async (e: React.FormEvent) => {
		e.preventDefault();
		setLoading(true);
		setError('');

		try {
			// First, get admin token
			const authResponse = await fetch('/api/admin/auth', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({
					admin_password: password,
				}),
			});

			if (authResponse.ok) {
				const authData = await authResponse.json();
				setAdminToken(authData.token);
				setAdminPassword(password);

				// Then get files list
				const filesResponse = await fetch('/api/admin/files', {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
					},
					body: JSON.stringify({
						admin_password: password,
					}),
				});

				if (filesResponse.ok) {
					const filesData: AdminResponse = await filesResponse.json();
					setIsAuthenticated(true);
					setFiles(filesData.files);
					setMessage(`${filesData.count} files found`);
				} else {
					const errorData = await filesResponse.json();
					setError(errorData.error || 'Failed to fetch files');
				}
			} else {
				const errorData = await authResponse.json();
				setError(errorData.error || 'Authentication failed');
			}
		} catch {
			setError('Network error occurred');
		} finally {
			setLoading(false);
		}
	};

	const refreshFileList = async () => {
		setLoading(true);
		setError('');

		try {
			const response = await fetch('/api/admin/files', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({
					admin_password: adminPassword,
				}),
			});

			if (response.ok) {
				const data: AdminResponse = await response.json();
				setFiles(data.files);
				setMessage(`${data.count} files found`);
			} else {
				const errorData = await response.json();
				setError(errorData.error || 'Failed to refresh file list');
			}
		} catch {
			setError('Network error occurred');
		} finally {
			setLoading(false);
		}
	};

	const deleteFile = async (fileId: string, filename: string) => {
		if (!confirm(`Are you sure you want to delete "${filename}"?`)) {
			return;
		}

		setLoading(true);
		setError('');

		try {
			const response = await fetch(`/api/admin/file/${fileId}`, {
				method: 'DELETE',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({
					admin_password: adminPassword,
				}),
			});

			if (response.ok) {
				setMessage(`File "${filename}" deleted successfully`);
				await refreshFileList();
			} else {
				const errorData = await response.json();
				setError(errorData.error || 'Failed to delete file');
			}
		} catch {
			setError('Network error occurred');
		} finally {
			setLoading(false);
		}
	};

	const updateExpiration = async (fileId: string, filename: string) => {
		const newDate = prompt(
			'Enter new expiration date (YYYY-MM-DD HH:MM:SS):',
			new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString().slice(0, 19).replace('T', ' ')
		);

		if (!newDate) return;

		// Convert to RFC3339 format
		const expirationDate = new Date(newDate).toISOString();

		setLoading(true);
		setError('');

		try {
			const response = await fetch(`/api/admin/file/${fileId}/expires`, {
				method: 'PUT',
				headers: {
					'Content-Type': 'application/json',
				},
				body: JSON.stringify({
					admin_password: adminPassword,
					expires_at: expirationDate,
				}),
			});

			if (response.ok) {
				setMessage(`Expiration updated for "${filename}"`);
				await refreshFileList();
			} else {
				const errorData = await response.json();
				setError(errorData.error || 'Failed to update expiration');
			}
		} catch {
			setError('Network error occurred');
		} finally {
			setLoading(false);
		}
	};

	const copyFileUrl = (fileId: string) => {
		const url = `${window.location.origin}/f/${fileId}`;
		navigator.clipboard
			.writeText(url)
			.then(() => {
				setMessage(`URL copied to clipboard: ${url}`);
				setTimeout(() => setMessage(''), 3000);
			})
			.catch(() => {
				setError('Failed to copy URL to clipboard');
				setTimeout(() => setError(''), 3000);
			});
	};

	const viewFile = (fileId: string) => {
		const url = `/f/${fileId}?admin_token=${encodeURIComponent(adminToken)}`;
		window.open(url, '_blank');
	};

	const logout = () => {
		setIsAuthenticated(false);
		setPassword('');
		setAdminPassword('');
		setAdminToken('');
		setFiles([]);
		setError('');
		setMessage('');
	};

	if (!isAuthenticated) {
		return (
			<div className='min-h-screen bg-gray-25 flex flex-col'>
				<main className='max-w-6xl mx-auto px-6 py-12 flex-1'>
					<div className='text-center mb-16'>
						<div className='w-16 h-16 bg-primary-500 mx-auto mb-8 flex items-center justify-center'>
							<Shield className='w-8 h-8 text-white' />
						</div>

						<h1 className='text-5xl md:text-6xl font-light text-gray-900 mb-6 tracking-tight'>
							Admin Access
						</h1>

						<p className='text-xl text-gray-600 mb-8 max-w-2xl mx-auto font-light'>
							Secure administrative panel for file management and system monitoring.
						</p>
					</div>

					<div className='max-w-md mx-auto'>
						<div className='bg-white border border-gray-200 p-8'>
							<form className='space-y-6' onSubmit={authenticate}>
								<Input
									label='Admin Password'
									type='password'
									value={password}
									onChange={(e) => setPassword(e.target.value)}
									placeholder='Enter admin password'
									required
									inputSize='md'
								/>

								{error && (
									<div className='text-red-600 text-sm bg-red-50 p-3 border border-red-200'>
										{error}
									</div>
								)}

								<Button
									type='submit'
									disabled={loading}
									variant='primary'
									size='md'
									className='w-full'
									loading={loading}
								>
									{loading ? 'Authenticating...' : 'Access Admin Panel'}
								</Button>
							</form>
						</div>
					</div>
				</main>
				<Footer />
			</div>
		);
	}

	return (
		<div className='min-h-screen bg-gray-25 flex flex-col'>
			<main className='max-w-6xl mx-auto px-6 py-12 flex-1 w-full'>
				{/* Header */}
				<div className='text-center mb-16'>
					<div className='w-16 h-16 bg-primary-500 mx-auto mb-8 flex items-center justify-center'>
						<Database className='w-8 h-8 text-white' />
					</div>

					<h1 className='text-5xl md:text-6xl font-light text-gray-900 mb-6 tracking-tight'>
						Admin Dashboard
					</h1>

					<p className='text-xl text-gray-600 mb-8 max-w-2xl mx-auto font-light'>
						Manage and monitor all uploaded files in the system.
					</p>

					<div className='flex items-center justify-center gap-4'>
						<Button
							onClick={refreshFileList}
							disabled={loading}
							variant='primary'
							size='md'
							icon={RefreshCw}
							loading={loading}
						>
							Refresh
						</Button>
						<Button
							onClick={logout}
							variant='secondary'
							size='md'
							icon={LogOut}
							className='bg-gray-600 hover:bg-gray-700 text-white border-gray-600'
						>
							Logout
						</Button>
					</div>
				</div>

				{/* Messages */}
				{message && (
					<div className='mb-8 bg-green-50 border border-green-200 text-green-700 px-4 py-3'>
						{message}
					</div>
				)}

				{error && (
					<div className='mb-8 bg-red-50 border border-red-200 text-red-700 px-4 py-3'>{error}</div>
				)}

				{/* Files Section */}
				<div className='bg-white border border-gray-200'>
					<div className='px-6 py-4 border-b border-gray-200'>
						<h3 className='text-lg font-medium text-gray-900'>Uploaded Files ({files.length})</h3>
						<p className='mt-1 text-sm text-gray-600'>Manage and monitor all uploaded files</p>
					</div>

					{files.length === 0 ? (
						<div className='text-center py-16'>
							<Database className='w-12 h-12 text-gray-400 mx-auto mb-4' />
							<p className='text-gray-500 text-lg'>No files found</p>
							<p className='text-gray-400 text-sm'>Upload some files to see them here</p>
						</div>
					) : (
						<div className='overflow-x-auto'>
							<table className='min-w-full'>
								<thead className='bg-gray-50 border-b border-gray-200'>
									<tr>
										<th className='px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider'>
											File
										</th>
										<th className='px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider'>
											Size
										</th>
										<th className='px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider'>
											Uploaded
										</th>
										<th className='px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider'>
											Expires
										</th>
										<th className='px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider'>
											Storage Location
										</th>
										<th className='px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider'>
											Actions
										</th>
									</tr>
								</thead>
								<tbody className='bg-white'>
									{files.map((file, index) => (
										<tr
											key={file.file_id}
											className={`${
												index % 2 === 0 ? 'bg-white' : 'bg-gray-25'
											} hover:bg-gray-50 transition-colors`}
										>
											<td className='px-6 py-4'>
												<div className='flex items-start'>
													<div className='flex-1'>
														<div className='flex items-center gap-2 mb-1'>
															<Button
																onClick={() => viewFile(file.file_id)}
																variant='secondary'
																size='sm'
																className='text-left truncate max-w-xs p-0 bg-transparent border-none text-primary-600 hover:text-primary-800 hover:bg-transparent font-medium'
																title={`View ${file.filename}`}
															>
																{file.filename}
															</Button>
															{file.has_password && (
																<div title='Password protected'>
																	<Lock className='w-3 h-3 text-yellow-600' />
																</div>
															)}
														</div>
														<div className='text-xs text-gray-500'>
															ID: {file.file_id.slice(0, 8)}...
														</div>
													</div>
												</div>
											</td>
											<td className='px-6 py-4 text-sm text-gray-900'>
												<div>
													{formatSize(file.size)}
													{file.compressed && file.original_size && (
														<div className='text-xs text-gray-500'>
															Original: {formatSize(file.original_size)}
														</div>
													)}
												</div>
											</td>
											<td className='px-6 py-4 text-sm text-gray-500'>
												{formatDate(file.uploaded_at)}
											</td>
											<td className='px-6 py-4 text-sm text-gray-500'>
												{formatDate(file.expires_at)}
											</td>
											<td className='px-6 py-4'>
												<div className='flex items-center gap-2'>
													{file.storage_type === 'disk' ? (
														<Archive className='w-4 h-4 text-blue-600' />
													) : (
														<Database className='w-4 h-4 text-green-600' />
													)}
													<div className='flex flex-col'>
														<span className='text-sm text-gray-700'>
															{file.storage_type === 'disk' ? 'Disk' : 'PostgreSQL'}
														</span>
														{file.compressed && (
															<span className='text-xs text-gray-500'>
																{file.compression
																	? `${file.compression.toUpperCase()} Compressed`
																	: 'Compressed'}
															</span>
														)}
													</div>
												</div>
											</td>
											<td className='px-6 py-4'>
												<div className='flex items-center gap-2'>
													<Button
														onClick={() => viewFile(file.file_id)}
														variant='secondary'
														size='sm'
														icon={Eye}
														className='p-2 text-green-600 hover:text-green-800 hover:bg-green-50 border-none bg-transparent'
														title='View file'
													>
														{''}
													</Button>
													<Button
														onClick={() => copyFileUrl(file.file_id)}
														variant='secondary'
														size='sm'
														icon={Copy}
														className='p-2 text-gray-600 hover:text-gray-800 hover:bg-gray-100 border-none bg-transparent'
														title='Copy URL'
													>
														{''}
													</Button>
													<Button
														onClick={() => updateExpiration(file.file_id, file.filename)}
														disabled={loading}
														variant='secondary'
														size='sm'
														icon={Clock}
														className='p-2 text-blue-600 hover:text-blue-800 hover:bg-blue-50 border-none bg-transparent'
														title='Update expiration'
													>
														{''}
													</Button>
													<Button
														onClick={() => deleteFile(file.file_id, file.filename)}
														disabled={loading}
														variant='danger'
														size='sm'
														icon={Trash2}
														className='p-2 hover:bg-red-50 border-none bg-transparent text-red-600 hover:text-red-800'
														title='Delete file'
													>
														{''}
													</Button>
												</div>
											</td>
										</tr>
									))}
								</tbody>
							</table>
						</div>
					)}
				</div>
			</main>
			<Footer />
		</div>
	);
};

export default AdminPage;
