import { Logo } from '@/components/shell/NavRail';

/** Login, setup and invite: a centered column, no app shell. */
export default function PublicLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="h-screen overflow-y-auto bg-bg text-fg">
      <main className="mx-auto w-full max-w-2xl px-4 py-10 md:py-16">
        <div className="mb-8">
          <Logo />
        </div>
        {children}
      </main>
    </div>
  );
}
