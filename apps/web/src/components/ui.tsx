import { forwardRef, useEffect, useId, useRef, useState, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode } from 'react';
import clsx from 'clsx';
import { Loader2, X } from 'lucide-react';
import { nameColor, rgb } from '@/lib/color';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';

export const Button = forwardRef<
  HTMLButtonElement,
  ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; size?: 'sm' | 'md'; loading?: boolean }
>(function Button({ variant = 'secondary', size = 'md', loading, className, children, disabled, type = 'button', ...rest }, ref) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      className={clsx(
        'inline-flex items-center justify-center gap-2 rounded-full font-semibold transition disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent',
        size === 'sm' ? 'h-8 px-3.5 text-[13px]' : 'h-10 px-5 text-sm',
        variant === 'primary' && 'bg-accent text-black hover:bg-accent-strong hover:scale-[1.02]',
        variant === 'secondary' && 'border border-line text-ink hover:border-ink',
        variant === 'ghost' && 'text-muted hover:text-ink hover:bg-raised',
        variant === 'danger' && 'border border-danger/60 text-danger hover:bg-danger/10',
        className,
      )}
      {...rest}
    >
      {loading && <Loader2 className="h-4 w-4 animate-spin" aria-hidden />}
      {children}
    </button>
  );
});

export const IconButton = forwardRef<
  HTMLButtonElement,
  ButtonHTMLAttributes<HTMLButtonElement> & { label: string; active?: boolean; size?: 'sm' | 'md' | 'lg' }
>(function IconButton({ label, active, size = 'md', className, children, type = 'button', ...rest }, ref) {
  return (
    <button
      ref={ref}
      type={type}
      aria-label={label}
      title={label}
      className={clsx(
        'inline-flex items-center justify-center rounded-full transition disabled:opacity-40 focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent',
        size === 'sm' && 'h-8 w-8',
        size === 'md' && 'h-9 w-9',
        size === 'lg' && 'h-12 w-12',
        active ? 'text-accent' : 'text-muted hover:text-ink',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
});

/** The round accent play button. */
export function PlayButton({ playing, onClick, size = 'lg', label, className }: { playing: boolean; onClick: () => void; size?: 'md' | 'lg'; label: string; className?: string }) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={(e) => {
        e.stopPropagation();
        e.preventDefault();
        onClick();
      }}
      className={clsx(
        'inline-flex shrink-0 items-center justify-center rounded-full bg-accent text-black shadow-lift transition hover:scale-105 hover:bg-accent-strong focus-visible:outline focus-visible:outline-2 focus-visible:outline-white',
        size === 'lg' ? 'h-14 w-14' : 'h-11 w-11',
        className,
      )}
    >
      {playing ? <PauseGlyph /> : <PlayGlyph />}
    </button>
  );
}

export const PlayGlyph = ({ className = 'h-5 w-5' }: { className?: string }) => (
  <svg viewBox="0 0 24 24" className={clsx(className, 'translate-x-[1px]')} fill="currentColor" aria-hidden>
    <path d="M7 4.5v15a1 1 0 0 0 1.5.86l12.5-7.5a1 1 0 0 0 0-1.72L8.5 3.64A1 1 0 0 0 7 4.5Z" />
  </svg>
);
export const PauseGlyph = ({ className = 'h-5 w-5' }: { className?: string }) => (
  <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden>
    <rect x="6" y="4" width="4" height="16" rx="1.2" />
    <rect x="14" y="4" width="4" height="16" rx="1.2" />
  </svg>
);

/** Animated bars for the track that is playing. */
export function Equalizer({ paused }: { paused?: boolean }) {
  return (
    <span className="inline-flex h-3.5 items-end gap-[2px]" aria-label={paused ? 'Paused' : 'Playing'}>
      {[0, 0.2, 0.4].map((d) => (
        <span
          key={d}
          className={clsx('w-[3px] origin-bottom rounded-sm bg-accent', !paused && 'animate-eq-bar')}
          style={{ height: '100%', animationDelay: `${d}s`, transform: paused ? 'scaleY(.4)' : undefined }}
        />
      ))}
    </span>
  );
}

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement> & { label?: string; hint?: string }>(
  function Input({ label, hint, className, id, ...rest }, ref) {
    const auto = useId();
    const inputId = id ?? auto;
    return (
      <label htmlFor={inputId} className="block">
        {label && <span className="mb-1.5 block text-[13px] font-semibold text-ink">{label}</span>}
        <input
          ref={ref}
          id={inputId}
          className={clsx(
            'h-11 w-full rounded-md border border-line bg-card px-3.5 text-[15px] text-ink placeholder:text-faint outline-none transition focus:border-ink',
            className,
          )}
          {...rest}
        />
        {hint && <span className="mt-1 block text-[12px] text-muted">{hint}</span>}
      </label>
    );
  },
);

export function Select<T extends string>({ label, value, onChange, options }: { label: string; value: T; onChange: (v: T) => void; options: { value: T; label: string }[] }) {
  const id = useId();
  return (
    <label htmlFor={id} className="block">
      <span className="mb-1.5 block text-[13px] font-semibold text-ink">{label}</span>
      <select
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value as T)}
        className="h-11 w-full rounded-md border border-line bg-card px-3 text-[15px] text-ink outline-none focus:border-ink"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </label>
  );
}

export function Modal({ title, open, onClose, children, footer, wide }: { title: string; open: boolean; onClose: () => void; children: ReactNode; footer?: ReactNode; wide?: boolean }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) {
      if (typeof d.showModal === 'function') d.showModal();
      else d.setAttribute('open', ''); // no modal dialog support
    }
    if (!open && d.open) d.close?.();
  }, [open]);
  if (!open) return null;
  return (
    <dialog
      ref={ref}
      aria-label={title}
      onClose={onClose}
      onCancel={onClose}
      onClick={(e) => e.target === ref.current && onClose()}
      className={clsx('m-auto max-h-[90vh] w-[calc(100%-2rem)] rounded-xl bg-raised p-0 text-ink shadow-lift backdrop:bg-black/70', wide ? 'max-w-2xl' : 'max-w-md')}
    >
      <div className="flex max-h-[90vh] flex-col">
        <div className="flex items-center justify-between px-6 pb-2 pt-5">
          <h2 className="text-xl font-bold">{title}</h2>
          <IconButton label="Close" onClick={onClose}>
            <X className="h-5 w-5" />
          </IconButton>
        </div>
        <div className="overflow-y-auto px-6 py-3">{children}</div>
        {footer && <div className="flex justify-end gap-2 px-6 pb-5 pt-3">{footer}</div>}
      </div>
    </dialog>
  );
}

export function Tabs<T extends string>({ tabs, value, onChange, label }: { tabs: { id: T; label: string; count?: number }[]; value: T; onChange: (id: T) => void; label: string }) {
  return (
    <div role="tablist" aria-label={label} className="flex flex-wrap gap-2">
      {tabs.map((t) => (
        <button
          key={t.id}
          role="tab"
          type="button"
          aria-selected={value === t.id}
          onClick={() => onChange(t.id)}
          className={clsx(
            'h-8 rounded-full px-3.5 text-[13px] font-semibold transition',
            value === t.id ? 'bg-ink text-black' : 'bg-raised text-ink hover:bg-line',
          )}
        >
          {t.label}
          {t.count != null && <span className="ml-1.5 tabular-nums opacity-60">{t.count}</span>}
        </button>
      ))}
    </div>
  );
}

export function Chip({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onClick}
      className={clsx(
        'h-8 rounded-full border px-3 text-[13px] font-medium transition',
        selected ? 'border-accent bg-accent text-black' : 'border-line bg-card text-ink hover:border-muted',
      )}
    >
      {children}
    </button>
  );
}

export function Spinner({ label = 'Loading' }: { label?: string }) {
  return (
    <div role="status" className="flex items-center justify-center py-16 text-muted">
      <Loader2 className="h-6 w-6 animate-spin" aria-hidden />
      <span className="sr-only">{label}</span>
    </div>
  );
}

export function Empty({ icon, title, children, action }: { icon?: ReactNode; title: string; children?: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex flex-col items-center px-6 py-16 text-center">
      {icon && <div className="mb-4 text-faint">{icon}</div>}
      <h3 className="text-lg font-bold">{title}</h3>
      {children && <p className="mt-1 max-w-md text-sm text-muted">{children}</p>}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}

export function ErrorText({ error }: { error: unknown }) {
  if (!error) return null;
  return (
    <p role="alert" className="text-sm text-danger">
      {error instanceof Error ? error.message : String(error)}
    </p>
  );
}

/**
 * Artwork: the image when it loads, else a gradient from the name with
 * its initials, so missing cover art still looks deliberate.
 */
export function Art({ src, name, className, rounded = 'md' }: { src?: string; name: string; className?: string; rounded?: 'md' | 'full' | 'none' }) {
  const [failed, setFailed] = useState(false);
  useEffect(() => setFailed(false), [src]);
  const c = nameColor(name);
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase())
    .join('');
  const radius = rounded === 'full' ? 'rounded-full' : rounded === 'md' ? 'rounded-md' : '';
  return (
    <div
      className={clsx('relative shrink-0 overflow-hidden bg-card', radius, className)}
      style={{
        backgroundImage: `linear-gradient(135deg, ${rgb(c)}, ${rgb([c[0] / 3, c[1] / 3, c[2] / 3])})`,
        containerType: 'inline-size',
      }}
      aria-hidden
    >
      {!failed && src ? (
        <img src={src} alt="" loading="lazy" onError={() => setFailed(true)} className="h-full w-full object-cover" />
      ) : (
        <span className="absolute inset-0 flex items-center justify-center font-bold text-white/80" style={{ fontSize: 'clamp(10px, 32cqw, 64px)' }}>
          {initials}
        </span>
      )}
    </div>
  );
}
