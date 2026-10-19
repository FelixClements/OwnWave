'use client';

import { useRef } from 'react';

type Tab<T extends string> = { id: T; label: string; count?: number };

/** WAI-ARIA tablist: arrow keys/Home/End move between tabs. Pair with TabPanel. */
export function Tabs<T extends string>({
  tabs,
  value,
  onChange,
  idPrefix,
  label,
}: {
  tabs: Tab<T>[];
  value: T;
  onChange: (id: T) => void;
  idPrefix: string;
  label: string;
}) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);

  const onKeyDown = (e: React.KeyboardEvent, index: number) => {
    const last = tabs.length - 1;
    const next =
      e.key === 'ArrowRight' ? (index === last ? 0 : index + 1)
      : e.key === 'ArrowLeft' ? (index === 0 ? last : index - 1)
      : e.key === 'Home' ? 0
      : e.key === 'End' ? last
      : null;
    if (next === null) return;
    e.preventDefault();
    onChange(tabs[next].id);
    refs.current[next]?.focus();
  };

  return (
    <div role="tablist" aria-label={label} className="flex gap-0.5 border-b border-line mb-3 overflow-x-auto">
      {tabs.map((tab, i) => {
        const selected = tab.id === value;
        return (
          <button
            key={tab.id}
            ref={(el) => {
              refs.current[i] = el;
            }}
            role="tab"
            type="button"
            id={`${idPrefix}-tab-${tab.id}`}
            aria-selected={selected}
            aria-controls={`${idPrefix}-panel-${tab.id}`}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(tab.id)}
            onKeyDown={(e) => onKeyDown(e, i)}
            className={`px-3 py-2 -mb-px border-b-2 font-medium whitespace-nowrap transition ${
              selected ? 'text-fg border-accent' : 'text-muted border-transparent hover:text-fg'
            }`}
          >
            {tab.label}
            {tab.count !== undefined && <span className="ml-1.5 text-[11px] text-muted tabular-nums">{tab.count}</span>}
          </button>
        );
      })}
    </div>
  );
}

export function TabPanel({ idPrefix, id, children }: { idPrefix: string; id: string; children: React.ReactNode }) {
  return (
    <div role="tabpanel" id={`${idPrefix}-panel-${id}`} aria-labelledby={`${idPrefix}-tab-${id}`} tabIndex={0} className="focus-visible:outline-none">
      {children}
    </div>
  );
}
