import { renderNavbar } from '../components/navbar.js?v=5';
import { get } from '../api.js';
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

function createRow(article, idx) {
  const row = document.createElement('div');
  row.className = 'article-row';

  const num = document.createElement('div');
  num.className = 'row-num';
  num.textContent = String(idx).padStart(2, '0');

  const main = document.createElement('div');
  main.className = 'row-main';

  const meta = document.createElement('div');
  meta.className = 'row-meta';
  const tags = article.tags || [];
  if (tags[0]) {
    const tag1 = document.createElement('span');
    tag1.className = 'tag';
    tag1.textContent = tags[0];
    meta.appendChild(tag1);
  }
  const date1 = document.createElement('span');
  date1.textContent = '·';
  meta.appendChild(date1);
  const date2 = document.createElement('span');
  date2.textContent = new Date(article.created_at * 1000).toLocaleDateString('zh-CN');
  meta.appendChild(date2);
  if (tags[1]) {
    const dot = document.createElement('span');
    dot.textContent = '·';
    meta.appendChild(dot);
    const tag2 = document.createElement('span');
    tag2.className = 'tag';
    tag2.textContent = tags[1];
    meta.appendChild(tag2);
  }

  const title = document.createElement('div');
  title.className = 'row-title';
  const titleLink = document.createElement('a');
  titleLink.href = `/detail.html?id=${article.id}`;
  titleLink.textContent = article.title;
  title.appendChild(titleLink);

  const summary = document.createElement('div');
  summary.className = 'row-summary';
  summary.textContent = article.summary || '';

  const foot = document.createElement('div');
  foot.className = 'row-foot';
  const author = document.createElement('a');
  author.href = `/profile.html?id=${article.user_id}`;
  author.textContent = article.author_name || '佚名';
  foot.appendChild(author);
  const sep1 = document.createElement('span');
  sep1.textContent = '·';
  foot.appendChild(sep1);
  const time = document.createElement('span');
  time.textContent = timeAgo(article.created_at);
  foot.appendChild(time);
  const sep2 = document.createElement('span');
  sep2.textContent = '·';
  foot.appendChild(sep2);
  const views = document.createElement('span');
  views.className = 'row-stat';
  views.textContent = '◉ ' + (article.view_count || 0) + ' 浏览';
  foot.appendChild(views);

  main.appendChild(meta);
  main.appendChild(title);
  main.appendChild(summary);
  main.appendChild(foot);

  const actions = document.createElement('div');
  actions.className = 'row-actions';
  const statBig = document.createElement('div');
  statBig.className = 'stat-big';
  statBig.textContent = (article.view_count || 0).toLocaleString();
  const statLabel = document.createElement('div');
  statLabel.className = 'stat-label';
  statLabel.textContent = '次浏览';
  actions.appendChild(statBig);
  actions.appendChild(statLabel);

  row.appendChild(num);
  row.appendChild(main);
  row.appendChild(actions);
  return row;
}

(async () => {
  await renderNavbar();

  const params = new URLSearchParams(location.search);
  const tag = params.get('tag') || '';
  let page = parseInt(params.get('page'), 10) || 1;
  const size = 10;

  const listEl = document.getElementById('article-list');

  async function loadData(p) {
    listEl.innerHTML = '<p style="color:#5A5C68;text-align:center;padding:2rem 0;font-family:JetBrains Mono,monospace">加载中…</p>';
    try {
      const q = new URLSearchParams({ page: String(p), size: String(size) });
      if (tag) q.set('tag', tag);
      const data = await get('/articles?' + q.toString());
      const articles = data.items || [];
      const total = data.total || 0;
      const totalPages = Math.ceil(total / size) || 1;

      listEl.innerHTML = '';
      if (articles.length === 0) {
        listEl.innerHTML = '<p style="color:#5A5C68;text-align:center;padding:2rem 0">暂无文章</p>';
      } else {
        articles.forEach((a, i) => listEl.appendChild(createRow(a, i + 1 + (p - 1) * size)));
      }

      renderPager({ current: p, total: totalPages, onChange: (np) => {
        page = np;
        const url = new URL(location.href);
        url.searchParams.set('page', String(np));
        history.pushState({}, '', url);
        loadData(np);
      }});
    } catch (err) {
      listEl.innerHTML = '<p style="color:#dc2626;text-align:center;padding:2rem 0">加载失败</p>';
      showToast({ type: 'error', text: err.message || '加载失败' });
    }
  }

  loadData(page);

  // Load sidebar data
  try {
    const trending = await get('/articles?page=1&size=5');
    const tList = document.getElementById('trending-list');
    tList.innerHTML = '';
    (trending.items || []).slice(0, 5).forEach(a => {
      const item = document.createElement('a');
      item.className = 'side-item';
      item.href = `/detail.html?id=${a.id}`;
      item.innerHTML = `<div class="si-title">${a.title}</div><div class="si-meta">${a.author_name || '佚名'} · ${timeAgo(a.created_at)}</div>`;
      tList.appendChild(item);
    });
  } catch {}

  try {
    const tags = await get('/tags');
    const tagList = document.getElementById('tags-list');
    tagList.innerHTML = '';
    (tags || []).slice(0, 12).forEach(t => {
      const chip = document.createElement('span');
      chip.className = 'tag-chip';
      chip.textContent = t.name;
      chip.style.cursor = 'pointer';
      chip.addEventListener('click', () => {
        location.href = `/list.html?tag=${encodeURIComponent(t.name)}`;
      });
      tagList.appendChild(chip);
    });
  } catch {}

  try {
    const writers = await get('/articles?page=1&size=20');
    const wMap = new Map();
    (writers.items || []).forEach(a => {
      if (!wMap.has(a.user_id)) wMap.set(a.user_id, { name: a.author_name, count: 0 });
      wMap.get(a.user_id).count++;
    });
    const wList = document.getElementById('writers-list');
    wList.innerHTML = '';
    Array.from(wMap.entries()).slice(0, 5).forEach(([uid, w]) => {
      const item = document.createElement('a');
      item.className = 'side-item';
      item.href = `/profile.html?id=${uid}`;
      item.innerHTML = `<div class="si-title">${w.name || '佚名'}</div><div class="si-meta">${w.count} 篇文章</div>`;
      wList.appendChild(item);
    });
  } catch {}
})();
