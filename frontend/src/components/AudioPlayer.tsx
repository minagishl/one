import React, { useState, useRef, useEffect } from 'react';
import { Play, Pause, Volume2, VolumeX } from 'lucide-react';

interface AudioPlayerProps {
	src: string;
	mimeType?: string;
	className?: string;
}

const AudioPlayer: React.FC<AudioPlayerProps> = ({ src, mimeType, className = '' }) => {
	const audioRef = useRef<HTMLAudioElement>(null);
	const [isPlaying, setIsPlaying] = useState(false);
	const [currentTime, setCurrentTime] = useState(0);
	const [duration, setDuration] = useState(0);
	const [volume, setVolume] = useState(0.5);
	const [isMuted, setIsMuted] = useState(false);
	const [isLoading, setIsLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [isDraggingVolume, setIsDraggingVolume] = useState(false);
	const [isDraggingProgress, setIsDraggingProgress] = useState(false);

	useEffect(() => {
		const audio = audioRef.current;
		if (!audio) return;

		const handleLoadedMetadata = () => {
			setDuration(audio.duration);
			setIsLoading(false);
		};

		const handleTimeUpdate = () => {
			setCurrentTime(audio.currentTime);
		};

		const handleEnded = () => {
			setIsPlaying(false);
		};

		const handleError = (e: Event) => {
			setError('Failed to load audio file');
			setIsLoading(false);
			console.error('Audio error:', e);
		};

		const handleCanPlay = () => {
			setIsLoading(false);
		};

		// Set initial volume
		audio.volume = 0.5;

		audio.addEventListener('loadedmetadata', handleLoadedMetadata);
		audio.addEventListener('timeupdate', handleTimeUpdate);
		audio.addEventListener('ended', handleEnded);
		audio.addEventListener('error', handleError);
		audio.addEventListener('canplay', handleCanPlay);

		return () => {
			audio.removeEventListener('loadedmetadata', handleLoadedMetadata);
			audio.removeEventListener('timeupdate', handleTimeUpdate);
			audio.removeEventListener('ended', handleEnded);
			audio.removeEventListener('error', handleError);
			audio.removeEventListener('canplay', handleCanPlay);
		};
	}, [src]);

	const togglePlay = () => {
		const audio = audioRef.current;
		if (!audio) return;

		if (isPlaying) {
			audio.pause();
		} else {
			audio.play().catch((err) => {
				console.error('Failed to play audio:', err);
				setError('Failed to play audio');
			});
		}
		setIsPlaying(!isPlaying);
	};

	const updateProgressFromPosition = (e: React.MouseEvent<HTMLDivElement>) => {
		const audio = audioRef.current;
		if (!audio || !duration) return;

		const rect = e.currentTarget.getBoundingClientRect();
		const clickX = e.clientX - rect.left;
		const width = rect.width;
		const newTime = (clickX / width) * duration;

		audio.currentTime = newTime;
		setCurrentTime(newTime);
	};

	const handleProgressMouseDown = (e: React.MouseEvent<HTMLDivElement>) => {
		setIsDraggingProgress(true);
		updateProgressFromPosition(e);
	};

	const handleProgressMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
		if (isDraggingProgress) {
			updateProgressFromPosition(e);
		}
	};

	const handleProgressMouseUp = () => {
		setIsDraggingProgress(false);
	};

	const updateVolumeFromPosition = (e: React.MouseEvent<HTMLDivElement>) => {
		const audio = audioRef.current;
		if (!audio) return;

		const rect = e.currentTarget.getBoundingClientRect();
		const clickX = e.clientX - rect.left;
		const width = rect.width;
		const newVolume = Math.max(0, Math.min(1, clickX / width));

		setVolume(newVolume);
		audio.volume = newVolume;
		setIsMuted(newVolume === 0);
	};

	const handleVolumeMouseDown = (e: React.MouseEvent<HTMLDivElement>) => {
		setIsDraggingVolume(true);
		updateVolumeFromPosition(e);
	};

	const handleVolumeMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
		if (isDraggingVolume) {
			updateVolumeFromPosition(e);
		}
	};

	const handleVolumeMouseUp = () => {
		setIsDraggingVolume(false);
	};

	// Watch global mouse events to handle dragging operations
	useEffect(() => {
		const handleGlobalMouseMove = (e: MouseEvent) => {
			if (isDraggingVolume) {
				// Get the volume slider element
				const volumeSlider = document.querySelector('[data-volume-slider]') as HTMLElement;
				if (volumeSlider) {
					const rect = volumeSlider.getBoundingClientRect();
					const clickX = e.clientX - rect.left;
					const width = rect.width;
					const newVolume = Math.max(0, Math.min(1, clickX / width));

					const audio = audioRef.current;
					if (audio) {
						setVolume(newVolume);
						audio.volume = newVolume;
						setIsMuted(newVolume === 0);
					}
				}
			}

			if (isDraggingProgress) {
				// Get the progress bar element
				const progressBar = document.querySelector('[data-progress-bar]') as HTMLElement;
				if (progressBar && duration) {
					const rect = progressBar.getBoundingClientRect();
					const clickX = e.clientX - rect.left;
					const width = rect.width;
					const newTime = (clickX / width) * duration;

					const audio = audioRef.current;
					if (audio) {
						audio.currentTime = newTime;
						setCurrentTime(newTime);
					}
				}
			}
		};

		const handleGlobalMouseUp = () => {
			setIsDraggingVolume(false);
			setIsDraggingProgress(false);
		};

		if (isDraggingVolume || isDraggingProgress) {
			document.addEventListener('mousemove', handleGlobalMouseMove);
			document.addEventListener('mouseup', handleGlobalMouseUp);
		}

		return () => {
			document.removeEventListener('mousemove', handleGlobalMouseMove);
			document.removeEventListener('mouseup', handleGlobalMouseUp);
		};
	}, [isDraggingVolume, isDraggingProgress, duration]);

	const toggleMute = () => {
		const audio = audioRef.current;
		if (!audio) return;

		if (isMuted) {
			audio.volume = volume;
			setIsMuted(false);
		} else {
			audio.volume = 0;
			setIsMuted(true);
		}
	};

	const formatTime = (time: number) => {
		if (isNaN(time)) return '0:00';
		const minutes = Math.floor(time / 60);
		const seconds = Math.floor(time % 60);
		return `${minutes}:${seconds.toString().padStart(2, '0')}`;
	};

	if (error) {
		return (
			<div className={`bg-red-50 border border-red-200 p-4 ${className}`}>
				<div className='text-red-600 text-sm'>{error}</div>
			</div>
		);
	}

	return (
		<div className={`card p-4 ${className}`}>
			<audio
				ref={audioRef}
				src={src}
				preload='metadata'
				crossOrigin='anonymous'
				style={{ display: 'none' }}
			>
				{mimeType && <source src={src} type={mimeType} />}
				Your browser does not support the audio element.
			</audio>

			<div className='flex items-center space-x-4'>
				{/* Play/Pause Button */}
				<button
					onClick={togglePlay}
					disabled={isLoading}
					className='flex items-center justify-center w-12 h-12 bg-primary-500 hover:bg-primary-600 disabled:bg-gray-300 text-white transition-colors'
				>
					{isLoading ? (
						<div className='animate-spin w-5 h-5 border-2 border-white border-t-transparent'></div>
					) : isPlaying ? (
						<Pause className='w-5 h-5 ml-0.5' />
					) : (
						<Play className='w-5 h-5 ml-0.5' />
					)}
				</button>

				{/* Progress Bar */}
				<div className='flex-1'>
					<div
						className='relative h-2 bg-gray-200 cursor-pointer select-none'
						data-progress-bar
						onMouseDown={handleProgressMouseDown}
						onMouseMove={handleProgressMouseMove}
						onMouseUp={handleProgressMouseUp}
					>
						<div
							className='absolute h-2 bg-primary-500'
							style={{
								width: duration ? `${(currentTime / duration) * 100}%` : '0%',
							}}
						></div>
					</div>
					<div className='flex justify-between text-xs text-gray-500 mt-1'>
						<span>{formatTime(currentTime)}</span>
						<span>{formatTime(duration)}</span>
					</div>
				</div>

				{/* Volume Control */}
				<div className='flex items-center space-x-2'>
					<button
						onClick={toggleMute}
						className='text-gray-500 hover:text-gray-700 transition-colors'
					>
						{isMuted || volume === 0 ? (
							<VolumeX className='w-5 h-5' />
						) : (
							<Volume2 className='w-5 h-5' />
						)}
					</button>
					<div className='w-20'>
						<div
							className='relative h-2 bg-gray-200 cursor-pointer select-none'
							data-volume-slider
							onMouseDown={handleVolumeMouseDown}
							onMouseMove={handleVolumeMouseMove}
							onMouseUp={handleVolumeMouseUp}
						>
							<div
								className='absolute h-2 bg-primary-500'
								style={{
									width: `${(isMuted ? 0 : volume) * 100}%`,
								}}
							></div>
						</div>
					</div>
				</div>
			</div>
		</div>
	);
};

export default AudioPlayer;
