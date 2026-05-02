import { get } from './api.js';

let cache = null;

export async function getCurrentUser(force = false) {
  if (!force && cache !== undefined) return cache;
  try {
    const u = await get('/auth/me');
    cache = u || null;
    return cache;
  } catch {
    cache = null;
    return null;
  }
}

export function clearUserCache() {
  cache = null;
}

export async function requireLogin() {
  const u = await getCurrentUser();
  if (!u) {
    location.href = '/login.html?redirect=' + encodeURIComponent(location.pathname + location.search);
    throw new Error('unauthenticated');
  }
  return u;
}
