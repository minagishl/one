/** @type {import('tailwindcss').Config} */
export default {
	content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
	theme: {
		extend: {
			colors: {
				// Dropbox-inspired blue color palette
				primary: {
					50: '#f0f8ff',
					100: '#e0f0ff',
					200: '#bae1ff',
					300: '#7dc8ff',
					400: '#38abff',
					500: '#0061ff', // Main Dropbox blue
					600: '#0052db',
					700: '#0043b7',
					800: '#003693',
					900: '#002970',
					950: '#001a47',
				},
				// Clean grays for text and backgrounds
				gray: {
					25: '#fcfcfd',
					50: '#f9fafb',
					100: '#f2f4f7',
					200: '#e4e7ec',
					300: '#d0d5dd',
					400: '#98a2b3',
					500: '#667085',
					600: '#475467',
					700: '#344054',
					800: '#1d2939',
					900: '#101828',
					950: '#0c111d',
				},
			},
			fontFamily: {
				sans: [
					'-apple-system',
					'BlinkMacSystemFont',
					'Segoe UI',
					'Roboto',
					'Oxygen',
					'Ubuntu',
					'Cantarell',
					'sans-serif',
				],
			},
			animation: {
				'spin-slow': 'spin 3s linear infinite',
			},
			boxShadow: {
				angular: '0 2px 8px rgba(0, 0, 0, 0.1)',
				'angular-lg': '0 4px 16px rgba(0, 0, 0, 0.1)',
				'angular-xl': '0 8px 32px rgba(0, 0, 0, 0.12)',
			},
		},
	},
	plugins: [],
};
