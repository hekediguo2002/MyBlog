const BASE = '/api/v1';

function getCookie(name) {
  const m = document.cookie.match(new RegExp('(?:^|; )' + name.replace(/([.$?*|{}()[\]\\/+^])/g, '\\$1') + '=([^;]*)'));
  return m ? decodeURIComponent(m[1]) : '';
}

export async function request(path, opts = {}) {
  const csrf = getCookie('csrf_token');
  const headers = {
    'Content-Type': 'application/json',
    ...(csrf ? { 'X-CSRF-Token': csrf } : {}),
    ...(opts.headers || {}),
  };

  if (opts.body instanceof FormData) {
    delete headers['Content-Type'];
  }

  const res = await fetch(BASE + path, {
    credentials: 'include',
    headers,
    ...opts,
  });

  let json;
  try {
    json = await res.json();
  } catch {
    throw { code: 5000, message: '系统繁忙，请稍后重试' };
  }

  if (json.code === 0) return json.data;

  if (json.code === 2001) {
    const msg = json.message || '登录已过期，请重新登录';
    import('./components/toast.js').then(m => m.showToast({ type: 'error', text: msg }));
    setTimeout(() => {
      location.href = '/login.html?redirect=' + encodeURIComponent(location.pathname + location.search);
    }, 100);
    throw json;
  }

  if (json.code === 2030) {
    import('./components/toast.js').then(m => m.showToast({ type: 'error', text: '安全校验失败，请刷新页面重试' }));
    setTimeout(() => location.reload(), 800);
    throw json;
  }

  throw json;
}

export const get = (p) => request(p);
export const post = (p, body) => request(p, { method: 'POST', body: JSON.stringify(body) });
export const put = (p, body) => request(p, { method: 'PUT', body: JSON.stringify(body) });
export const del = (p) => request(p, { method: 'DELETE' });
