import type { Config } from 'tailwindcss';

const config: Config = {
  content: [
    './app/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        spotify: {
          black: 'var(--spotify-black)',
          bg: 'var(--spotify-bg)',
          card: 'var(--spotify-card)',
          'card-hover': 'var(--spotify-card-hover)',
          green: 'var(--spotify-green)',
          'green-hover': 'var(--spotify-green-hover)',
          text: 'var(--spotify-text)',
          subdued: 'var(--spotify-subdued)',
          elevated: 'var(--spotify-elevated)',
          border: 'var(--spotify-border)',
        },
      },
    },
  },
  plugins: [],
};

export default config;
