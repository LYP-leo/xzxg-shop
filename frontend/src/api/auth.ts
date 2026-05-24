import type { Account, AuthSession } from '../types/auth';
import { requestJSON } from './http';

const STORAGE_KEY = 'xzxg_auth_session';

export async function login(input: { username: string; password: string }): Promise<AuthSession> {
  const session = await requestJSON<AuthSession>('/auth/login', {
    method: 'POST',
    body: JSON.stringify(input)
  });
  saveSession(session);
  return session;
}

export async function register(input: { username: string; password: string; display_name?: string }): Promise<AuthSession> {
  const session = await requestJSON<AuthSession>('/auth/register', {
    method: 'POST',
    body: JSON.stringify(input)
  });
  saveSession(session);
  return session;
}

export async function fetchMe(token: string): Promise<Account> {
  return requestJSON<Account>('/auth/me', {
    headers: {
      Authorization: `Bearer ${token}`
    }
  });
}

export function loadSession(): AuthSession | undefined {
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) return undefined;
  try {
    return JSON.parse(raw) as AuthSession;
  } catch {
    window.localStorage.removeItem(STORAGE_KEY);
    return undefined;
  }
}

export function saveSession(session: AuthSession) {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
}

export function clearSession() {
  window.localStorage.removeItem(STORAGE_KEY);
}
