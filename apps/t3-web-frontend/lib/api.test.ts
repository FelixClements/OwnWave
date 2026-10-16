import { describe, expect, it } from 'vitest';
import { errorText } from './api';

describe('errorText', () => {
  it('unwraps JSON error bodies and keeps plain text', () => {
    expect(errorText('{"detail":"task queue unavailable"}')).toBe('task queue unavailable');
    expect(errorText('{"error":"The pack failed."}')).toBe('The pack failed.');
    expect(errorText('username taken')).toBe('username taken');
    expect(errorText('{"detail":[{"msg":"bad"}]}')).toBe('{"detail":[{"msg":"bad"}]}');
    expect(errorText('')).toBe('');
  });
});
