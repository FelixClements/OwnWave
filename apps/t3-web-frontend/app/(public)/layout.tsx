/** Login, setup and invite: no app shell, just a softly lit backdrop. */
export default function PublicLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative h-screen overflow-y-auto bg-bg text-fg isolate">
      <div
        className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-[480px] bg-[radial-gradient(ellipse_60%_60%_at_50%_0%,rgb(var(--accent)/0.12),transparent)]"
        aria-hidden="true"
      />
      {children}
    </div>
  );
}
