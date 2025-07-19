import React from 'react';
import { LucideIcon } from 'lucide-react';
import { tv, type VariantProps } from 'tailwind-variants';

const button = tv({
	base: 'font-medium transition-all duration-200 inline-flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed',
	variants: {
		variant: {
			primary:
				'bg-primary-500 text-white hover:bg-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
			secondary:
				'bg-white text-gray-700 border border-gray-300 hover:bg-gray-50 hover:border-gray-400 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
			danger:
				'bg-red-500 text-white hover:bg-red-600 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2',
		},
		size: {
			sm: 'px-3 py-2 text-sm',
			md: 'px-6 py-3 text-base',
			lg: 'px-8 py-4 text-lg',
		},
	},
	defaultVariants: {
		variant: 'primary',
		size: 'md',
	},
});

const icon = tv({
	variants: {
		size: {
			sm: 'w-4 h-4',
			md: 'w-4 h-4',
			lg: 'w-5 h-5',
		},
		loading: {
			true: 'animate-spin',
		},
	},
});

export type ButtonVariant = VariantProps<typeof button>['variant'];
export type ButtonSize = VariantProps<typeof button>['size'];

interface ButtonProps
	extends React.ButtonHTMLAttributes<HTMLButtonElement>,
		VariantProps<typeof button> {
	icon?: LucideIcon;
	iconPosition?: 'left' | 'right';
	loading?: boolean;
	children?: React.ReactNode;
}

const Button: React.FC<ButtonProps> = ({
	variant,
	size,
	icon: Icon,
	iconPosition = 'left',
	loading = false,
	className,
	disabled,
	children,
	...props
}) => {
	const iconElement = Icon && <Icon className={icon({ size, loading })} />;

	return (
		<button
			className={button({ variant, size, className })}
			disabled={disabled || loading}
			{...props}
		>
			{iconPosition === 'left' && iconElement}
			{children}
			{iconPosition === 'right' && iconElement}
		</button>
	);
};

export default Button;
