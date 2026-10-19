/** Page, panel and status building blocks. */

export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center gap-3 mb-4">
      <div className="flex-1 min-w-0">
        <h1 className="text-xl font-bold truncate">{title}</h1>
        {subtitle && <div className="text-muted mt-0.5">{subtitle}</div>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

export function Panel({
  title,
  actions,
  children,
  className = '',
}: {
  title?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={`rounded-lg border border-line bg-surface p-3.5 ${className}`} aria-label={title}>
      {(title || actions) && (
        <div className="flex items-center gap-2 mb-3">
          {title && <h2 className="flex-1 text-sm font-semibold">{title}</h2>}
          {actions}
        </div>
      )}
      {children}
    </section>
  );
}

export type NoticeValue = { type: 'success' | 'error' | 'info'; text: string } | null;

/** Inline status message; errors are announced assertively. */
export function Notice({ notice }: { notice: NoticeValue }) {
  if (!notice) return null;
  const tone =
    notice.type === 'error'
      ? 'text-danger border-danger/40 bg-danger/10'
      : notice.type === 'success'
        ? 'text-ok border-ok/40 bg-ok/10'
        : 'text-fg border-line bg-surface-2';
  return (
    <p role={notice.type === 'error' ? 'alert' : 'status'} className={`rounded-md border px-3 py-2 ${tone}`}>
      {notice.text}
    </p>
  );
}

export function Badge({ children, tone = 'neutral', className = '' }: { children: React.ReactNode; tone?: 'neutral' | 'accent' | 'danger' | 'ok'; className?: string }) {
  const tones = {
    neutral: 'text-muted border-line bg-surface-2',
    accent: 'text-accent border-accent/40 bg-accent/10',
    danger: 'text-danger border-danger/40 bg-danger/10',
    ok: 'text-ok border-ok/40 bg-ok/10',
  };
  return (
    <span className={`inline-flex items-center h-5 px-1.5 rounded border text-[11px] whitespace-nowrap ${tones[tone]} ${className}`}>
      {children}
    </span>
  );
}
