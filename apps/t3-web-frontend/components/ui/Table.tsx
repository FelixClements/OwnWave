/** Dense table primitives shared by every data table. */

export function Table({ label, children, className = '' }: { label: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={`overflow-x-auto ${className}`}>
      <table className="w-full border-collapse table-fixed">
        <caption className="sr-only">{label}</caption>
        {children}
      </table>
    </div>
  );
}

type Align = 'left' | 'right';

export function Th({
  children,
  align = 'left',
  className = '',
  ...rest
}: React.ThHTMLAttributes<HTMLTableCellElement> & { align?: Align }) {
  return (
    <th
      scope="col"
      className={`px-2 py-1.5 border-b border-line text-[10.5px] font-semibold uppercase tracking-wider text-muted whitespace-nowrap ${
        align === 'right' ? 'text-right' : 'text-left'
      } ${className}`}
      {...rest}
    >
      {children}
    </th>
  );
}

export function Td({
  children,
  align = 'left',
  className = '',
  ...rest
}: React.TdHTMLAttributes<HTMLTableCellElement> & { align?: Align }) {
  return (
    <td
      className={`px-2 h-[34px] border-b border-line/60 truncate ${align === 'right' ? 'text-right tabular-nums' : ''} ${className}`}
      {...rest}
    >
      {children}
    </td>
  );
}

/** Two-line title/subtitle cell content. */
export function TitleCell({ title, subtitle, active }: { title: string; subtitle?: string; active?: boolean }) {
  return (
    <div className="min-w-0 leading-tight">
      <div className={`truncate font-medium ${active ? 'text-accent' : ''}`}>{title}</div>
      {subtitle && <div className="truncate text-[11.5px] text-muted">{subtitle}</div>}
    </div>
  );
}

export function EmptyRow({ colSpan, children }: { colSpan: number; children: React.ReactNode }) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-2 py-6 text-center text-muted">
        {children}
      </td>
    </tr>
  );
}
