import type { ReactNode } from 'react';
import clsx from 'clsx';

/** A titled group of settings rows in one bordered card. */
export function Section({ title, action, children, className }: { title?: ReactNode; action?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={clsx('mb-8', className)}>
      {(title || action) && (
        <div className="mb-2.5 flex flex-wrap items-center justify-between gap-3 px-0.5">
          {title && <h2 className="text-[15px] font-semibold">{title}</h2>}
          {action}
        </div>
      )}
      <div className="divide-y divide-line overflow-hidden rounded-xl border border-line bg-card">{children}</div>
    </section>
  );
}

/** One setting: title and description on the left, its control on the right. */
export function Row({ title, description, children, className }: { title: ReactNode; description?: ReactNode; children?: ReactNode; className?: string }) {
  return (
    <div className={clsx('flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3', className)}>
      <div className="min-w-0 flex-1 basis-56">
        <div className="text-sm font-medium">{title}</div>
        {description && <div className="mt-0.5 text-[13px] text-muted">{description}</div>}
      </div>
      {children && <div className="flex flex-wrap items-center gap-2">{children}</div>}
    </div>
  );
}

/** A row with free content (tables, lists, progress) instead of a control. */
export function Block({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={clsx('px-4 py-3', className)}>{children}</div>;
}

export function Switch({ label, checked, onChange, disabled }: { label: string; checked: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={clsx(
        'relative h-5 w-9 shrink-0 rounded-full border transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-accent disabled:opacity-50',
        checked ? 'border-accent-strong bg-accent-strong' : 'border-line bg-raised',
      )}
    >
      <span className={clsx('absolute top-0.5 h-3.5 w-3.5 rounded-full transition-all', checked ? 'left-[18px] bg-white' : 'left-0.5 bg-faint')} />
    </button>
  );
}

export function SelectControl<T extends string>({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: T;
  onChange: (v: T) => void;
  options: { value: T; label: string }[];
}) {
  return (
    <select
      aria-label={label}
      value={value}
      onChange={(e) => onChange(e.target.value as T)}
      className="h-9 rounded-lg border border-line bg-raised px-2.5 text-sm text-ink outline-none focus:border-ink"
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );
}

const tones = {
  neutral: 'bg-raised text-muted',
  ok: 'bg-accent-soft text-accent',
  warn: 'bg-amber-400/15 text-amber-300',
  bad: 'bg-danger/15 text-danger',
};

export function Tag({ tone = 'neutral', children }: { tone?: keyof typeof tones; children: ReactNode }) {
  return <span className={clsx('inline-block rounded px-1.5 py-0.5 text-[11px] font-medium', tones[tone])}>{children}</span>;
}

/** Small print under a section. */
export function Note({ children }: { children: ReactNode }) {
  return <p className="-mt-5 mb-8 px-0.5 text-[12px] text-faint">{children}</p>;
}

/** Table styles shared by the settings pages. */
export const table = {
  table: 'w-full text-left text-sm',
  th: 'px-4 py-2.5 text-[12px] font-medium text-faint',
  td: 'border-t border-line px-4 py-2.5',
};
