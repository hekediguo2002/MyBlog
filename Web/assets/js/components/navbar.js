import { getCurrentUser, clearUserCache } from '../auth.js';
import { post } from '../api.js';
import { showToast } from './toast.js';

function createEl(tag, cls, text) {
  const el = document.createElement(tag);
  if (cls) el.className = cls;
  if (text !== undefined) el.textContent = text;
  return el;
}

export async function renderNavbar(mountId = 'navbar-mount') {
  const mount = document.getElementById(mountId);
  if (!mount) return;

  const user = await getCurrentUser();

  const nav = createEl('nav', 'navbar');
  const inner = createEl('div', 'navbar-inner');

  const left = createEl('div', 'navbar-left');
  const logo = createEl('a', 'navbar-logo', 'Blog');
  logo.href = '/list.html';
  left.appendChild(logo);

  const tagsWrap = createEl('div', 'nav-tags');
  tagsWrap.id = 'navbar-tags';
  tagsWrap.style.display = 'flex';
  tagsWrap.style.gap = '0.5rem';
  left.appendChild(tagsWrap);

  const right = createEl('div', 'navbar-right');

  if (user) {
    const writeBtn = createEl('a', 'btn btn-primary');
    writeBtn.href = '/editor.html';
    writeBtn.textContent = '写文章';
    right.appendChild(writeBtn);

    const nameSpan = createEl('span', '', user.name || user.username);
    nameSpan.style.fontSize = '0.875rem';
    nameSpan.style.cursor = 'pointer';
    nameSpan.addEventListener('click', () => {
      location.href = '/profile.html';
    });
    right.appendChild(nameSpan);

    const logoutBtn = createEl('button', 'btn btn-secondary');
    logoutBtn.textContent = '登出';
    logoutBtn.addEventListener('click', async () => {
      try {
        await post('/auth/logout');
        clearUserCache();
        showToast({ type: 'success', text: '已退出登录' });
        setTimeout(() => location.reload(), 400);
      } catch (err) {
        showToast({ type: 'error', text: err.message || '登出失败' });
      }
    });
    right.appendChild(logoutBtn);
  } else {
    const loginBtn = createEl('a', 'btn btn-secondary');
    loginBtn.href = '/login.html?redirect=' + encodeURIComponent(location.pathname + location.search);
    loginBtn.textContent = '登录';
    right.appendChild(loginBtn);
  }

  inner.appendChild(left);
  inner.appendChild(right);
  nav.appendChild(inner);
  mount.appendChild(nav);
}
