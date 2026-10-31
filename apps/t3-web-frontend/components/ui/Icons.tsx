type IconProps = { className?: string };

function Stroke({ className = 'w-4 h-4', children }: IconProps & { children: React.ReactNode }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.8}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

function Fill({ className = 'w-4 h-4', children }: IconProps & { children: React.ReactNode }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      {children}
    </svg>
  );
}

export const RadioIcon = (p: IconProps) => (
  <Stroke {...p}>
    <path d="M3 12a9 9 0 0 1 18 0M7 12a5 5 0 0 1 10 0" />
    <circle cx="12" cy="12" r="1.5" />
    <path d="M12 14v7" />
  </Stroke>
);
export const LibraryIcon = (p: IconProps) => (
  <Stroke {...p}>
    <path d="M4 4v16M9 4v16M14 4l5 16" />
  </Stroke>
);
export const HistoryIcon = (p: IconProps) => (
  <Stroke {...p}>
    <circle cx="12" cy="12" r="8" />
    <path d="M12 8v4l3 2" />
  </Stroke>
);
export const SimilarIcon = (p: IconProps) => (
  <Stroke {...p}>
    <circle cx="8" cy="12" r="4" />
    <circle cx="16" cy="12" r="4" />
  </Stroke>
);
export const StationsIcon = (p: IconProps) => (
  <Stroke {...p}>
    <rect x="3" y="7" width="18" height="13" rx="2" />
    <path d="m8 3 4 4 4-4" />
    <circle cx="15" cy="13.5" r="2.5" />
  </Stroke>
);
export const PlaylistIcon = (p: IconProps) => (
  <Stroke {...p}>
    <path d="M4 6h11M4 11h11M4 16h7" />
    <circle cx="17.5" cy="17" r="2.5" />
    <path d="M20 17V8l-2.5 1" />
  </Stroke>
);
export const ShieldIcon = (p: IconProps) => (
  <Stroke {...p}>
    <path d="M12 3 4 6v6c0 4.5 3.4 8.2 8 9 4.6-.8 8-4.5 8-9V6z" />
  </Stroke>
);
export const SettingsIcon = (p: IconProps) => (
  <Stroke {...p}>
    <circle cx="12" cy="12" r="3" />
    <path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9 7 7M17 17l2.1 2.1M4.9 19.1 7 17M17 7l2.1-2.1" />
  </Stroke>
);
export const SearchIcon = (p: IconProps) => (
  <Stroke {...p}>
    <circle cx="11" cy="11" r="7" />
    <path d="m20 20-3.5-3.5" />
  </Stroke>
);
export const MenuIcon = (p: IconProps) => (
  <Stroke {...p}>
    <path d="M4 6h16M4 12h16M4 18h16" />
  </Stroke>
);
export const CloseIcon = (p: IconProps) => (
  <Stroke {...p}>
    <path d="M6 6l12 12M18 6 6 18" />
  </Stroke>
);
export const SunIcon = (p: IconProps) => (
  <Stroke {...p}>
    <circle cx="12" cy="12" r="4" />
    <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
  </Stroke>
);
export const MoonIcon = (p: IconProps) => (
  <Stroke {...p}>
    <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />
  </Stroke>
);
export const BanIcon = (p: IconProps) => (
  <Stroke {...p}>
    <circle cx="12" cy="12" r="8" />
    <path d="m6.5 6.5 11 11" />
  </Stroke>
);
export const SkipNextIcon = (p: IconProps) => (
  <Fill {...p}>
    <path d="M5 4.5v15l10-7.5zM17 5h2.5v14H17z" />
  </Fill>
);
export const SkipPrevIcon = (p: IconProps) => (
  <Fill {...p}>
    <path d="M19 4.5v15L9 12zM4.5 5H7v14H4.5z" />
  </Fill>
);
export const PlayIcon = (p: IconProps) => (
  <Fill {...p}>
    <path d="M7 4.5v15l12-7.5z" />
  </Fill>
);
export const PauseIcon = (p: IconProps) => (
  <Fill {...p}>
    <path d="M6 5h4v14H6zM14 5h4v14h-4z" />
  </Fill>
);
export const HeartIcon = ({ filled, ...p }: IconProps & { filled?: boolean }) =>
  filled ? (
    <Fill {...p}>
      <path d="M12 21s-7.5-4.6-9.5-9.3C1 8 3.4 4.5 7 4.5c2 0 3.6 1.1 5 3 1.4-1.9 3-3 5-3 3.6 0 6 3.5 4.5 7.2C19.5 16.4 12 21 12 21z" />
    </Fill>
  ) : (
    <Stroke {...p}>
      <path d="M12 21s-7.5-4.6-9.5-9.3C1 8 3.4 4.5 7 4.5c2 0 3.6 1.1 5 3 1.4-1.9 3-3 5-3 3.6 0 6 3.5 4.5 7.2C19.5 16.4 12 21 12 21z" />
    </Stroke>
  );
export const Spinner = ({ className = 'w-3.5 h-3.5' }: IconProps) => (
  <svg className={`animate-spin ${className}`} viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <circle cx="12" cy="12" r="9" stroke="currentColor" strokeOpacity="0.25" strokeWidth="3" />
    <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
  </svg>
);

/** Animated playing indicator; static when paused. */
export function Equalizer({ paused }: { paused?: boolean }) {
  return (
    <span className={`eq ${paused ? 'paused' : ''}`} aria-hidden="true">
      <span />
      <span />
      <span />
    </span>
  );
}
