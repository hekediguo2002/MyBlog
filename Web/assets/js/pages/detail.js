import { renderNavbar } from '../components/navbar.js?v=5';
import { getCurrentUser } from '../auth.js';
import { get, del } from '../api.js';
import { renderMarkdown } from '../markdown.js';
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

(async () => {
  await renderNavbar();

  const params = new URLSearchParams(location.search);
  const id = params.get('id');
  if (!id) {
    showToast({ type: 'error', text: '缺少文章 ID' });
    location.href = '/list.html';
    return;
  }

  try {
    const article = await get('/articles/' + id);
    const me = await getCurrentUser();

    document.getElementById('title').textContent = article.title;
    document.getElementById('desc').textContent = article.summary || '';
    document.getElementById('footer-path').textContent = '文章详情 · 浏览数已 +1';

    // Kicker
    const tags = article.tags || [];
    const pill = document.getElementById('kicker-pill');
    const kMeta = document.getElementById('kicker-meta');
    if (tags[0]) {
      pill.textContent = tags[0];
    } else {
      pill.style.display = 'none';
    }
    const dateStr = new Date(article.created_at * 1000).toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' });
    kMeta.textContent = (tags[1] ? tags[1] + ' · ' : '') + dateStr;

    // Author
    const authorEl = document.getElementById('author');
    authorEl.innerHTML = `
      <div class="av">${(article.author_name || '佚名').slice(0,1)}</div>
      <div class="info">
        <div class="name">${article.author_name || '佚名'}</div>
        <div class="date">${timeAgo(article.created_at)} · ${article.view_count || 0} 次浏览</div>
      </div>
    `;

    // Content
    const contentEl = document.getElementById('content');
    contentEl.innerHTML = renderMarkdown(article.content);

    // TOC from headings
    const toc = document.getElementById('toc');
    const headings = contentEl.querySelectorAll('h1, h2, h3');
    if (headings.length > 0) {
      headings.forEach((h, i) => {
        const div = document.createElement('div');
        div.style.paddingLeft = (h.tagName === 'H2' ? '0.75rem' : h.tagName === 'H3' ? '1.5rem' : '0');
        div.style.cursor = 'pointer';
        div.textContent = h.textContent;
        div.addEventListener('click', () => h.scrollIntoView({ behavior: 'smooth' }));
        toc.appendChild(div);
      });
    } else {
      toc.textContent = '暂无目录';
    }

    // Stats
    document.getElementById('stat-date').textContent = dateStr;
    document.getElementById('stat-views').textContent = article.view_count || 0;
    const sAuth = document.getElementById('stat-author');
    sAuth.textContent = article.author_name || '佚名';
    sAuth.href = `/profile.html?id=${article.user_id}`;

    // Tags
    const tagRow = document.getElementById('tag-row');
    if (tags.length > 0) {
      tagRow.style.display = 'flex';
      tags.forEach(t => {
        const chip = document.createElement('span');
        chip.className = 'tag-chip';
        chip.textContent = '#' + t;
        chip.style.cursor = 'pointer';
        chip.addEventListener('click', () => {
          location.href = `/list.html?tag=${encodeURIComponent(t)}`;
        });
        tagRow.appendChild(chip);
      });
    }

    // Owner actions in navbar
    if (me && me.id === article.user_id) {
      const navR = document.querySelector('.navbar-right');
      if (navR) {
        const editBtn = document.createElement('a');
        editBtn.className = 'btn btn-secondary btn-sm';
        editBtn.href = `/editor.html?id=${article.id}`;
        editBtn.textContent = '编辑';
        navR.insertBefore(editBtn, navR.firstChild);

        const delBtn = document.createElement('button');
        delBtn.className = 'btn btn-danger btn-sm';
        delBtn.textContent = '删除';
        delBtn.addEventListener('click', async () => {
          if (!confirm('确定要删除这篇文章吗？此操作不可撤销。')) return;
          try {
            await del('/articles/' + id);
            showToast({ type: 'success', text: '已删除' });
            setTimeout(() => { location.href = '/list.html'; }, 400);
          } catch (err) {
            showToast({ type: 'error', text: err.message || '删除失败' });
          }
        });
        navR.insertBefore(delBtn, navR.firstChild);
      }
    }

    // Bio card
    const bioWrap = document.getElementById('bio-wrap');
    bioWrap.style.display = 'flex';
    document.getElementById('bio-av').textContent = (article.author_name || '佚名').slice(0, 2).toUpperCase();
    document.getElementById('bio-name').textContent = article.author_name || '佚名';
    document.getElementById('bio-handle').textContent = '@' + (article.author_name || 'anonymous');
    document.getElementById('bio-btn').href = `/profile.html?id=${article.user_id}`;

    // Related articles
    try {
      const related = await get('/articles?page=1&size=3');
      const items = (related.items || []).filter(a => String(a.id) !== String(id)).slice(0, 3);
      if (items.length > 0) {
        const relWrap = document.getElementById('rel-wrap');
        relWrap.style.display = 'block';
        const relGrid = document.getElementById('rel-grid');
        items.forEach(a => {
          const card = document.createElement('a');
          card.className = 'rel-card';
          card.href = `/detail.html?id=${a.id}`;
          card.innerHTML = `
            <div class="title">${a.title}</div>
            <div class="meta">${a.author_name || '佚名'} · ${timeAgo(a.created_at)} · ◉ ${a.view_count || 0}</div>
          `;
          relGrid.appendChild(card);
        });
      }
    } catch {}

  } catch (err) {
    if (err.code === 1030) {
      showToast({ type: 'error', text: '资源不存在' });
    } else {
      showToast({ type: 'error', text: err.message || '加载失败' });
    }
    setTimeout(() => { location.href = '/list.html'; }, 800);
  }
})();
