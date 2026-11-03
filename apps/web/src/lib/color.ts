/** A dominant colour of an image, darkened enough for white text on top. */
export type Rgb = [number, number, number];

export const FALLBACK: Rgb = [45, 212, 191];

/**
 * Picks the most saturated of the image's averaged colours: averages each
 * cell of a 4x4 grid, then prefers colourful cells over grey ones.
 */
export function dominantColor(pixels: Uint8ClampedArray, width: number, height: number): Rgb {
  const grid = 4;
  let best: Rgb | null = null;
  let bestScore = -1;
  for (let gy = 0; gy < grid; gy++) {
    for (let gx = 0; gx < grid; gx++) {
      let r = 0, g = 0, b = 0, n = 0;
      for (let y = Math.floor((gy * height) / grid); y < Math.floor(((gy + 1) * height) / grid); y++) {
        for (let x = Math.floor((gx * width) / grid); x < Math.floor(((gx + 1) * width) / grid); x++) {
          const i = (y * width + x) * 4;
          if (pixels[i + 3] < 128) continue;
          r += pixels[i]; g += pixels[i + 1]; b += pixels[i + 2]; n++;
        }
      }
      if (!n) continue;
      const c: Rgb = [r / n, g / n, b / n];
      const max = Math.max(...c), min = Math.min(...c);
      const score = (max - min) * (1 - Math.abs(max - 140) / 255); // saturated, mid-bright
      if (score > bestScore) [best, bestScore] = [c, score];
    }
  }
  return best ? darken(best) : FALLBACK;
}

/** Scales the colour so its brightest channel is at most 150. */
export function darken([r, g, b]: Rgb, maxChannel = 150): Rgb {
  const max = Math.max(r, g, b, 1);
  const k = Math.min(1, maxChannel / max);
  return [Math.round(r * k), Math.round(g * k), Math.round(b * k)];
}

export const rgb = ([r, g, b]: Rgb, a = 1) => `rgba(${r}, ${g}, ${b}, ${a})`;

const cache = new Map<string, Promise<Rgb>>();

/** Loads an image (same origin) and returns its dominant colour. */
export function imageColor(url: string): Promise<Rgb> {
  let p = cache.get(url);
  if (!p) {
    p = new Promise<Rgb>((resolve) => {
      const img = new Image();
      img.onload = () => {
        try {
          const size = 32;
          const canvas = document.createElement('canvas');
          canvas.width = canvas.height = size;
          const ctx = canvas.getContext('2d');
          if (!ctx) return resolve(FALLBACK);
          ctx.drawImage(img, 0, 0, size, size);
          resolve(dominantColor(ctx.getImageData(0, 0, size, size).data, size, size));
        } catch {
          resolve(FALLBACK);
        }
      };
      img.onerror = () => resolve(FALLBACK);
      img.src = url;
    });
    cache.set(url, p);
  }
  return p;
}

/** Stable colour for things without artwork, from their name. */
export function nameColor(name: string): Rgb {
  let h = 0;
  for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  const hue = h % 360;
  // HSL(hue, 55%, 35%) to RGB.
  const s = 0.55, l = 0.35;
  const k = (n: number) => (n + hue / 30) % 12;
  const a = s * Math.min(l, 1 - l);
  const f = (n: number) => Math.round(255 * (l - a * Math.max(-1, Math.min(k(n) - 3, Math.min(9 - k(n), 1)))));
  return [f(0), f(8), f(4)];
}
