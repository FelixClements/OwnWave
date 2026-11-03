import '@testing-library/jest-dom/vitest';

// jsdom does not implement media playback.
Object.assign(HTMLMediaElement.prototype, {
  play: () => Promise.resolve(),
  pause: () => {},
  load: () => {},
});
