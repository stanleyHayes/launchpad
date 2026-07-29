/**
 * Non-sensitive browser session state. Session tokens themselves live in
 * HttpOnly cookies set by the API and are never exposed to JavaScript; this
 * storage only records a "signed in" presence flag so pages can short-circuit
 * to the login screen without an API round-trip.
 */
export type SessionStorage = {
  /** Mark the session as present after a verified login. */
  saveSession(): void;
  /** Drop the presence flag and any legacy token remnants. */
  clearSession(): void;
  /**
   * Legacy alias kept for call sites that gated on a stored token; now
   * returns the non-sensitive presence flag ("1") or null. Use only as a
   * boolean check.
   */
  getAccessToken(): string | null;
};

/** createSessionStorage builds per-app session state keyed by prefix. */
export function createSessionStorage(prefix: string): SessionStorage {
  const sessionKey = `${prefix}.session`;

  return {
    saveSession() {
      window.localStorage.setItem(sessionKey, "1");
    },
    clearSession() {
      window.localStorage.removeItem(sessionKey);
      // Pre-cookie sessions stored raw tokens under these keys; remove any
      // leftovers so tokens never linger in browser storage.
      window.localStorage.removeItem(`${prefix}.accessToken`);
      window.localStorage.removeItem(`${prefix}.refreshToken`);
    },
    getAccessToken() {
      return window.localStorage.getItem(sessionKey);
    },
  };
}
