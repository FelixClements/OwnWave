/** Centered sign-in style card for the login and invite pages. */
export function AuthCard({
  title,
  subtitle,
  footer,
  children,
}: {
  title: string;
  subtitle: string;
  footer?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <main className="min-h-full flex flex-col items-center justify-center px-4 py-12">
      <div className="w-full max-w-[360px]">
        <div className="flex flex-col items-center text-center mb-6">
          <span
            className="grid place-items-center w-11 h-11 rounded-xl bg-gradient-to-br from-accent to-blue-500 text-accent-fg shadow-lg shadow-accent/20 mb-4"
            aria-hidden="true"
          >
            <svg viewBox="0 0 24 24" className="w-6 h-6" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round">
              <path d="M3 12h2M7 8v8M11 5v14M15 9v6M19 11v2" />
            </svg>
          </span>
          <p className="text-[11.5px] font-semibold uppercase tracking-[0.14em] text-muted mb-1.5">OwnWave</p>
          <h1 className="text-2xl font-bold tracking-tight">{title}</h1>
          <p className="text-muted mt-1.5 text-[13.5px]">{subtitle}</p>
        </div>
        <div className="rounded-xl border border-line bg-surface/80 backdrop-blur p-6 shadow-xl shadow-black/20">{children}</div>
        {footer && <p className="mt-5 text-center text-muted">{footer}</p>}
      </div>
    </main>
  );
}
