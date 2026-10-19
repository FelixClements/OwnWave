function hashString(s: string) {
  let h = 0;
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0;
  return Math.abs(h);
}

/** Generated station tile: two hues derived from the name. */
export function StationArt({ name, className = 'w-[18px] h-[18px] rounded' }: { name: string; className?: string }) {
  const hue = hashString(name) % 360;
  return (
    <span
      aria-hidden="true"
      className={`inline-block shrink-0 ${className}`}
      style={{ background: `linear-gradient(135deg, hsl(${hue} 65% 32%), hsl(${(hue + 60) % 360} 65% 50%))` }}
    />
  );
}
