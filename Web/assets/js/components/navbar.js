import { getCurrentUser, clearUserCache } from '../auth.js';
import { post, get } from '../api.js';
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
  const logo = createEl('a', 'navbar-logo');
  logo.href = '/list.html';
  const logoSq = createEl('span', 'logo-sq', 'N');
  const logoText = createEl('span', '', '小绿书');
  logo.appendChild(logoSq);
  logo.appendChild(logoText);
  left.appendChild(logo);

  const tagsWrap = createEl('div', 'nav-tags');
  tagsWrap.id = 'navbar-tags';
  left.appendChild(tagsWrap);

  const right = createEl('div', 'navbar-right');

  if (user) {
    const writeBtn = createEl('a', 'btn btn-primary btn-sm');
    writeBtn.href = '/editor.html';
    writeBtn.textContent = '写文章';
    right.appendChild(writeBtn);

    const userBtn = createEl('div', 'nav-user');
    const uAv = createEl('span', 'u-av', (user.name || user.username || '?').slice(0, 1).toUpperCase());
    const uName = createEl('span', '', user.name || user.username);
    userBtn.appendChild(uAv);
    userBtn.appendChild(uName);
    userBtn.addEventListener('click', () => {
      location.href = '/profile.html';
    });
    right.appendChild(userBtn);

    const logoutBtn = createEl('button', 'btn btn-secondary btn-sm');
    logoutBtn.textContent = '登出';
    logoutBtn.addEventListener('click', async () => {
      try {
        await post('/auth/logout');
        clearUserCache();
        showToast({ type: 'success', text: '已退出登录' });
        setTimeout(() => { location.href = '/login.html'; }, 400);
      } catch (err) {
        showToast({ type: 'error', text: err.message || '登出失败' });
      }
    });
    right.appendChild(logoutBtn);
  } else {
    const loginBtn = createEl('a', 'btn btn-secondary btn-sm');
    loginBtn.href = '/login.html?redirect=' + encodeURIComponent(location.pathname + location.search);
    loginBtn.textContent = '跳转到登录页';
    right.appendChild(loginBtn);
  }

  inner.appendChild(left);
  inner.appendChild(right);
  nav.appendChild(inner);
  mount.appendChild(nav);

  // Load tags asynchronously
  try {
    const tagData = await get('/tags');
    const tags = tagData || [];
    const navbarTags = document.getElementById('navbar-tags');
    if (navbarTags) {
      tags.slice(0, 6).forEach(t => {
        const chip = createEl('span', 'nav-tag', t.name);
        chip.addEventListener('click', () => {
          location.href = `/list.html?tag=${encodeURIComponent(t.name)}`;
        });
        navbarTags.appendChild(chip);
      });
    }
  } catch {}
}
