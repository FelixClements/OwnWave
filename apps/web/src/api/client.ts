import createClient, { type Middleware } from 'openapi-fetch';
import type { paths } from './schema';

/** An API failure: the HTTP status and the server's plain-text message. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message || `Request failed (${status})`);
  }
}

/** Called on 401 so the app can drop back to the login page. */
let onUnauthorized: () => void = () => {};
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
}

const errors: Middleware = {
  async onResponse({ response }) {
    if (response.ok) return response;
    const text = (await response.clone().text()).trim();
    if (response.status === 401) onUnauthorized();
    throw new ApiError(response.status, text);
  },
};

/** Typed client for the OwnWave API (cookie session, same origin). */
export const api = createClient<paths>({
  // Absolute, so Request() works outside a page too (tests); same origin in the app.
  baseUrl: typeof window !== 'undefined' ? window.location.origin : '',
  credentials: 'include',
  // Looked up per request, so a replaced global fetch (tests) is used.
  fetch: (req) => globalThis.fetch(req),
});
api.use(errors);

/**
 * Like call, for endpoints whose body the spec types as a plain object
 * (shapes owned by the analytics service): the caller names the type.
 */
export function callAs<T>(p: Promise<{ data?: unknown; error?: unknown; response: Response }>): Promise<T> {
  return call(p) as Promise<T>;
}

/** Unwraps openapi-fetch results; errors were already thrown by the middleware. */
export async function call<T>(p: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
  const { data, response } = await p;
  if (response.status === 204) return undefined as T;
  return data as T;
}
