import { renderNavbar } from '../components/navbar.js';
import { requireLogin } from '../auth.js';
import { get, del } from '../api.js';
import { showToast } from '../components/toast.js';

(async () => {
  const user = await requireLogin();
  if (!user || !user.isAdmin) {
    location.href = '/list.html';
    return;
  }

  renderNavbar('navbar-mount', { hideWrite: true, hideAdmin: true });
  loadUsers();
})();

async function loadUsers() {
  const container = document.getElementById('user-list');
  container.innerHTML = '<div class="admin-row"><span style="color:#5A5C68;padding:1.5rem 2rem">加载中…</span></div>';
  try {
    const users = await get('/admin/users');
    if (!users || users.length === 0) {
      container.innerHTML = '<div class="admin-row"><span style="color:#5A5C68;padding:1.5rem 2rem">暂无注册用户</span></div>';
      return;
    }
    container.innerHTML = '';
    users.forEach(u => {
      const row = document.createElement('div');
      row.className = 'admin-row';
      const date = u.created_at ? new Date(u.created_at * 1000).toLocaleDateString('zh-CN') : '-';
      row.innerHTML = `
        <span>${u.id}</span>
        <span>${escapeHtml(u.username)}</span>
        <span>${escapeHtml(u.name || '-')}</span>
        <span>${date}</span>
        <span style="text-align:right"><button class="btn btn-danger btn-sm" data-id="${u.id}" data-name="${escapeHtml(u.username)}">删除</button></span>
      `;
      container.appendChild(row);
    });

    container.querySelectorAll('button[data-id]').forEach(btn => {
      btn.addEventListener('click', () => {
        const id = btn.dataset.id;
        const name = btn.dataset.name;
        if (!confirm(`确定删除用户 "${name}" 及其发布的全部文章？此操作不可恢复。`)) return;
        doDelete(id);
      });
    });
  } catch (err) {
    container.innerHTML = '<div class="admin-row"><span style="color:#dc2626;padding:1.5rem 2rem">加载失败</span></div>';
    showToast({ type: 'error', text: err.message || '加载失败' });
  }
}

async function doDelete(id) {
  try {
    await del(`/admin/users/${id}`);
    showToast({ type: 'success', text: '删除成功' });
    loadUsers();
  } catch (err) {
    showToast({ type: 'error', text: err.message || '删除失败' });
  }
}

function escapeHtml(str) {
  if (!str) return '';
  return str.replace(/[<>"'&]/g, m => ({ '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;', '&': '&amp;' }[m]));
}
