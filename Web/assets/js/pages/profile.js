import { renderNavbar } from '../components/navbar.js?v=5';
import { getCurrentUser } from '../auth.js';
import { get, del } from '../api.js';
import { renderPager } from '../components/pager.js';
import { showToast } from '../components/toast.js';

function timeAgo(ts) {
  if (!ts) return '刚刚';
  const diff = Date.now() - new Date(ts * 1000).getTime();
  const m = Math.floor(diff / 60000);
  const h = Math.floor(diff / 3600000);
  const d = Math.floor(diff / 86400000);
  if (d > 0) return d + ' 天前';
  if (h > 0) return h + ' 小时前';
  if (m > 0) return m + ' 分钟前';
  return '刚刚';
}

function createCard(article, idx) {
  const card = document.createElement('div');
  card.className = 'profile-card';

  const num = document.createElement('div');
  num.className = 'c-num';
  num.innerHTML = `<div class="idx">⊹  ${String(idx).padStart(2, '0')}</div><div class="date">${new Date(article.created_at * 1000).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })}</div>`;

  const main = document.createElement('div');
  main.className = 'c-main';
  const title = document.createElement('a');
  title.className = 'c-title';
  title.href = `/detail.html?id=${article.id}`;
  title.textContent = article.title;
  const summary = document.createElement('div');
  summary.className = 'c-summary';
  summary.textContent = article.summary || '';
  const tags = document.createElement('div');
  tags.className = 'c-tags';
  (article.tags || []).forEach((t, i) => {
    if (i > 0) tags.appendChild(document.createTextNode(' '));
    const span = document.createElement('span');
    span.className = 'tag';
    span.textContent = t;
    tags.appendChild(span);
    if (i < (article.tags || []).length - 1) {
      const sep = document.createElement('span');
      sep.className = 'sep';
      sep.textContent = '·';
      tags.appendChild(sep);
      const tname = document.createElement('span');
      tname.className = 'tname';
      tname.textContent = t;
      tags.appendChild(tname);
    }
  });
  main.appendChild(title);
  main.appendChild(summary);
  if ((article.tags || []).length > 0) main.appendChild(tags);

  const actions = document.createElement('div');
  actions.className = 'c-actions';
  const statBig = document.createElement('div');
  statBig.className = 'stat-big';
  statBig.textContent = (article.view_count || 0).toLocaleString();
  const statLabel = document.createElement('div');
  statLabel.className = 'stat-label';
  statLabel.textContent = '次浏览';
  actions.appendChild(statBig);
  actions.appendChild(statLabel);

  card.appendChild(num);
  card.appendChild(main);
  card.appendChild(actions);
  return card;
}

(async () => {
  await renderNavbar();

  const params = new URLSearchParams(location.search);
  const profileId = params.get('id');
  const me = await getCurrentUser();
  const isMe = me && (!profileId || String(me.id) === String(profileId));
  const targetId = profileId || (me ? me.id : null);

  if (!targetId) {
    showToast({ type: 'error', text: '请先登录' });
    location.href = '/login.html?redirect=' + encodeURIComponent(location.pathname + location.search);
    return;
  }

  try {
    let user = null;
    if (isMe && me) {
      user = me;
    }

    const articlesRes = await get('/users/' + targetId + '/articles?page=1&size=100');
    const articles = articlesRes.items || [];

    // Derive user info from articles if not current user
    if (!user && articles.length > 0) {
      user = {
        name: articles[0].author_name,
        username: 'user' + targetId,
        created_at: articles[0].created_at,
      };
    }
    if (!user) {
      user = { name: '佚名', username: 'user' + targetId, created_at: Date.now() / 1000 };
    }

    document.getElementById('name').textContent = user.name || user.username || '佚名';
    document.getElementById('handle').textContent = '@' + (user.username || 'anonymous') + ' ／ ' + (user.name || user.username);
    document.getElementById('bio').textContent = user.bio || '在写: 一个不需要打包的博客 · 一些做了三年没上线的玩具 · 凌晨写下的小段子。';
    document.getElementById('avatar').textContent = (user.name || user.username || '?').slice(0, 2).toUpperCase();
    document.getElementById('hero-kicker').textContent = '⊹  写作者档案  ·  自 ' + new Date(user.created_at * 1000).toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' }).replace(/\//g, '.');

    // Links
    const links = document.getElementById('links');
    if (user.location) {
      const loc = document.createElement('span');
      loc.className = 'loc';
      loc.textContent = '⌖ ' + user.location;
      links.appendChild(loc);
    }
    if (user.website) {
      if (links.children.length > 0) {
        const sep = document.createElement('span');
        sep.className = 'sep';
        sep.textContent = '·';
        links.appendChild(sep);
      }
      const a = document.createElement('a');
      a.href = user.website;
      a.textContent = '⊕ ' + user.website.replace(/^https?:\/\//, '');
      links.appendChild(a);
    }

    // Actions
    if (isMe) {
      document.getElementById('hAct').style.display = 'flex';
      document.getElementById('edit-btn').addEventListener('click', () => {
        showToast({ type: 'info', text: '编辑资料功能即将上线' });
      });
      document.getElementById('update').textContent = '上次更新 ' + new Date().toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }).replace(/\//g, '.');
    }

    // Stats
    const totalViews = articles.reduce((s, a) => s + (a.view_count || 0), 0);
    document.getElementById('stat-articles').textContent = articles.length;
    document.getElementById('stat-views').textContent = totalViews.toLocaleString();
    document.getElementById('stat-followers').textContent = '0';
    const weekAgo = Date.now() - 7 * 86400000;
    const weekCount = articles.filter(a => new Date(a.created_at * 1000).getTime() > weekAgo).length;
    document.getElementById('stat-week').textContent = weekCount;
    document.getElementById('tab-count').textContent = articles.length;

    // List
    const listEl = document.getElementById('profile-list');
    if (articles.length === 0) {
      listEl.innerHTML = '<p style="color:#5A5C68;text-align:center;padding:2rem 0">暂无文章</p>';
    } else {
      articles.forEach((a, i) => listEl.appendChild(createCard(a, i + 1)));
    }

    // Owner edit/delete buttons
    if (isMe) {
      listEl.querySelectorAll('.profile-card').forEach((card, i) => {
        const actions = card.querySelector('.c-actions');
        const btns = document.createElement('div');
        btns.className = 'btns';
        const editBtn = document.createElement('a');
        editBtn.className = 'btn btn-secondary btn-sm';
        editBtn.href = `/editor.html?id=${articles[i].id}`;
        editBtn.textContent = '编辑';
        const delBtn = document.createElement('button');
        delBtn.className = 'btn btn-danger btn-sm';
        delBtn.textContent = '删除';
        delBtn.addEventListener('click', async () => {
          if (!confirm('确定要删除这篇文章吗？此操作不可撤销。')) return;
          try {
            await del('/articles/' + articles[i].id);
            showToast({ type: 'success', text: '已删除' });
            card.remove();
          } catch (err) {
            showToast({ type: 'error', text: err.message || '删除失败' });
          }
        });
        btns.appendChild(editBtn);
        btns.appendChild(delBtn);
        actions.appendChild(btns);
      });
    }

  } catch (err) {
    showToast({ type: 'error', text: err.message || '加载失败' });
    location.href = '/list.html';
  }
})();
