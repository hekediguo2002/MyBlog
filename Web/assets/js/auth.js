let cache;

export async function getCurrentUser(force = false) {
  if (!force && cache !== undefined) return cache;
  try {
    const csrf = document.cookie.match(/csrf_token=([^;]*)/)?.[1] || '';
    const r = await fetch('/api/v1/auth/me', {
      credentials: 'include',
      headers: { 'X-CSRF-Token': csrf },
    });
    const json = await r.json();
    if (json.code === 0) {
      cache = json.data || null;
      return cache;
    }
    cache = null;
    return null;
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
