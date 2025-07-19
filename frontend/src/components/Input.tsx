import React from 'react';
import { LucideIcon } from 'lucide-react';
import { tv, type VariantProps } from 'tailwind-variants';

const input = tv({
	base: 'border bg-white text-gray-900 placeholder-gray-500 transition-all duration-200 focus:outline-none w-full',
	variants: {
		inputSize: {
			sm: 'px-3 py-2 text-sm',
			md: 'px-4 py-3 text-base',
			lg: 'px-5 py-4 text-lg',
		},
		error: {
			true: 'border-red-300 focus:border-red-500 focus:ring-1 focus:ring-red-500',
			false: 'border-gray-300 focus:border-primary-500 focus:ring-1 focus:ring-primary-500',
		},
		hasIcon: {
			true: '', // Will be computed in compound variants
		},
		iconPosition: {
			left: '',
			right: '',
		},
	},
	compoundVariants: [
		{
			hasIcon: true,
			iconPosition: 'left',
			inputSize: 'sm',
			class: 'pl-9',
		},
		{
			hasIcon: true,
			iconPosition: 'left',
			inputSize: 'md',
			class: 'pl-10',
		},
		{
			hasIcon: true,
			iconPosition: 'left',
			inputSize: 'lg',
			class: 'pl-12',
		},
		{
			hasIcon: true,
			iconPosition: 'right',
			inputSize: 'sm',
			class: 'pr-9',
		},
		{
			hasIcon: true,
			iconPosition: 'right',
			inputSize: 'md',
			class: 'pr-10',
		},
		{
			hasIcon: true,
			iconPosition: 'right',
			inputSize: 'lg',
			class: 'pr-12',
		},
	],
	defaultVariants: {
		inputSize: 'md',
		error: false,
		iconPosition: 'left',
	},
});

const inputIcon = tv({
	base: 'text-gray-400',
	variants: {
		inputSize: {
			sm: 'w-4 h-4',
			md: 'w-5 h-5',
			lg: 'w-6 h-6',
		},
	},
});

const iconContainer = tv({
	base: 'absolute inset-y-0 flex items-center pointer-events-none',
	variants: {
		position: {
			left: '',
			right: '',
		},
		inputSize: {
			sm: '',
			md: '',
			lg: '',
		},
	},
	compoundVariants: [
		{
			position: 'left',
			inputSize: 'sm',
			class: 'left-3',
		},
		{
			position: 'left',
			inputSize: 'md',
			class: 'left-3',
		},
		{
			position: 'left',
			inputSize: 'lg',
			class: 'left-4',
		},
		{
			position: 'right',
			inputSize: 'sm',
			class: 'right-3',
		},
		{
			position: 'right',
			inputSize: 'md',
			class: 'right-3',
		},
		{
			position: 'right',
			inputSize: 'lg',
			class: 'right-4',
		},
	],
});

export type InputSize = VariantProps<typeof input>['inputSize'];

interface InputProps
	extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size'>,
		Omit<VariantProps<typeof input>, 'error'> {
	label?: string;
	error?: string;
	icon?: LucideIcon;
	iconPosition?: 'left' | 'right';
	containerClassName?: string;
}

const Input: React.FC<InputProps> = ({
	inputSize,
	label,
	error,
	icon: Icon,
	iconPosition = 'left',
	className,
	containerClassName = '',
	id,
	...props
}) => {
	const inputId = id || `input-${Math.random().toString(36).substr(2, 9)}`;
	const hasError = Boolean(error);
	const hasIcon = Boolean(Icon);

	return (
		<div className={containerClassName}>
			{label && (
				<label htmlFor={inputId} className='block text-sm font-medium text-gray-900 mb-2'>
					{label}
				</label>
			)}
			<div className='relative'>
				<input
					id={inputId}
					className={input({
						inputSize,
						error: hasError,
						hasIcon,
						iconPosition,
						className,
					})}
					{...props}
				/>
				{Icon && (
					<div className={iconContainer({ position: iconPosition, inputSize })}>
						<Icon className={inputIcon({ inputSize })} />
					</div>
				)}
			</div>
			{error && <p className='mt-1 text-sm text-red-600'>{error}</p>}
		</div>
	);
};

export default Input;
