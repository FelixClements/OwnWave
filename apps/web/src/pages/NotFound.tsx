import { Link } from 'react-router-dom';
import { Empty } from '@/components/ui';

export function NotFoundPage() {
  return (
    <Empty
      title="Page not found"
      action={
        <Link to="/" className="rounded-full bg-ink px-5 py-2 text-sm font-bold text-black">
          Home
        </Link>
      }
    >
      This page does not exist.
    </Empty>
  );
}
