import { forwardRef } from 'react';
import { Spinner } from './Icons';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';
type Size = 'sm' | 'md';

const VARIANTS: Record<Variant, string> = {
  primary: 'bg-accent text-accent-fg border-accent font-semibold hover:brightness-110',
  secondary: 'bg-surface text-fg border-line hover:bg-hover',
  ghost: 'bg-transparent text-muted border-transparent hover:text-fg hover:bg-hover',
  danger: 'bg-surface text-danger border-line hover:bg-hover',
};

const SIZES: Record<Size, string> = {
  sm: 'h-6 px-2 text-xs gap-1',
  md: 'h-[30px] px-3 text-[12.5px] gap-1.5',
};

/** Classes for a button-styled element; also used on Links. */
export function buttonClass(variant: Variant = 'secondary', size: Size = 'md', extra = '') {
  return `inline-flex items-center justify-center rounded-md border font-medium whitespace-nowrap transition disabled:opacity-50 disabled:pointer-events-none ${VARIANTS[variant]} ${SIZES[size]} ${extra}`;
}

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
  loading?: boolean;
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant, size, loading, className = '', type = 'button', disabled, children, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={buttonClass(variant, size, className)}
      {...rest}
    >
      {loading && <Spinner />}
      {children}
    </button>
  );
});

/** Square icon-only button; `label` is required for screen readers. */
export const IconButton = forwardRef<HTMLButtonElement, ButtonProps & { label: string }>(function IconButton(
  { label, className = '', type = 'button', ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      aria-label={label}
      title={label}
      className={`inline-grid place-items-center w-8 h-8 rounded-md text-muted hover:text-fg hover:bg-hover transition disabled:opacity-50 ${className}`}
      {...rest}
    />
  );
});
