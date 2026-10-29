import { Logo } from '@/components/shell/NavRail';

/** The setup wizard: a wide, top-aligned column. */
export default function SetupLayout({ children }: { children: React.ReactNode }) {
  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-10 md:py-16">
      <div className="mb-8">
        <Logo />
      </div>
      {children}
    </main>
  );
}
