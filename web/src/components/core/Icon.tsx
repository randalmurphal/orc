/**
 * Icon component - Lucide icon wrapper with consistent sizing and colors.
 * Provides standardized icon rendering with accessibility support.
 */

import { forwardRef, type SVGAttributes } from 'react';
import type { LucideIcon } from 'lucide-react';
import './Icon.css';

export type IconSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl';
export type IconColor = 'primary' | 'secondary' | 'muted' | 'success' | 'warning' | 'error';

export interface IconProps extends Omit<SVGAttributes<SVGSVGElement>, 'ref'> {
	/** Lucide icon component to render */
	name: LucideIcon;
	/** Icon size: xs=12px, sm=14px, md=16px (default), lg=18px, xl=20px */
	size?: IconSize;
	/** Icon color mapped to CSS variables */
	color?: IconColor;
	/** Accessible label for interactive icons (required when not decorative) */
	'aria-label'?: string;
	/** Set to true for decorative icons that should be hidden from screen readers */
	'aria-hidden'?: boolean;
}

const sizeMap: Record<IconSize, number> = {
	xs: 12,
	sm: 14,
	md: 16,
	lg: 18,
	xl: 20,
};

/**
 * Icon component for rendering Lucide icons with consistent styling.
 *
 * @example
 * // Interactive icon with label
 * <Icon name={Home} size="lg" aria-label="Home" />
 *
 * @example
 * // Decorative icon (hidden from screen readers)
 * <Icon name={Check} color="success" aria-hidden />
 *
 * @example
 * // Error icon
 * <Icon name={AlertCircle} color="error" size="sm" />
 */
export const Icon = forwardRef<SVGSVGElement, IconProps>(
	(
		{
			name: IconComponent,
			size = 'md',
			color,
			className = '',
			'aria-label': ariaLabel,
			'aria-hidden': ariaHidden,
			...props
		},
		ref
	) => {
		const pixelSize = sizeMap[size];

		const classes = ['icon', `icon--${size}`, color && `icon--${color}`, className]
			.filter(Boolean)
			.join(' ');

		return (
			<IconComponent
				ref={ref}
				className={classes}
				width={pixelSize}
				height={pixelSize}
				aria-label={ariaLabel}
				aria-hidden={ariaHidden ?? !ariaLabel}
				role={ariaLabel ? 'img' : undefined}
				{...props}
			/>
		);
	}
);

Icon.displayName = 'Icon';
