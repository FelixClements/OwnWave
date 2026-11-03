import type { Config } from 'tailwindcss';

// Dark, Spotify-inspired palette with OwnWave's own teal accent.
export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        base: '#0d0d0f', // app background
        panel: '#141417', // sidebar and main panels
        card: '#1b1b1f',
        raised: '#26262c',
        line: '#2e2e35',
        ink: '#f5f5f7',
        muted: '#a1a1aa',
        faint: '#71717a',
        accent: { DEFAULT: '#2dd4bf', strong: '#14b8a6', soft: '#2dd4bf26' },
        danger: '#f87171',
      },
      fontFamily: {
        sans: ['"Inter Variable"', 'ui-sans-serif', 'system-ui', 'Segoe UI', 'Roboto', 'sans-serif'],
      },
      boxShadow: { lift: '0 8px 24px rgba(0,0,0,.5)' },
      keyframes: {
        'eq-bar': { '0%,100%': { transform: 'scaleY(.3)' }, '50%': { transform: 'scaleY(1)' } },
      },
      animation: { 'eq-bar': 'eq-bar 0.9s ease-in-out infinite' },
    },
  },
  plugins: [],
} satisfies Config;
