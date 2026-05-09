import { renderNavbar } from '../components/navbar.js?v=5';
import { requireLogin } from '../auth.js';
import { get, post, put } from '../api.js';
import { showToast } from '../components/toast.js';

function $(id) { return document.getElementById(id); }

function insertText(textarea, text) {
  const start = textarea.selectionStart;
  const end = textarea.selectionEnd;
  const before = textarea.value.slice(0, start);
  const after = textarea.value.slice(end);
  textarea.value = before + text + after;
  textarea.selectionStart = textarea.selectionEnd = start + text.length;
  textarea.focus();
}

function updateGutter(textarea) {
  const lines = textarea.value.split('\n').length;
  const gutter = $('gutter');
  const curLine = textarea.value.slice(0, textarea.selectionStart).split('\n').length;
  gutter.innerHTML = '';
  for (let i = 1; i <= Math.max(lines, 1); i++) {
    const ln = document.createElement('div');
    ln.className = 'ln' + (i === curLine ? ' cur' : '');
    ln.textContent = String(i).padStart(2, '0');
    gutter.appendChild(ln);
  }
}

function updateStatus() {
  const now = new Date();
  $('sb-time').textContent = now.toTimeString().slice(0, 8);
  const content = $('content').value;
  const len = content.length;
  const mins = Math.max(1, Math.ceil(len / 250));
  $('sb-count').textContent = `${len.toLocaleString()} 字 · 约 ${mins} 分钟`;

  // Checklist
  const title = $('title').value.trim();
  const rawTags = $('tags') ? $('tags').value : '';
  const tagList = rawTags.split(/[,，]/).map(t => t.trim()).filter(Boolean);
  const summary = $('summary').value.trim();

  let done = 0;
  function setCk(id, ok) {
    const el = $(id);
    el.className = 'check-item ' + (ok ? 'done' : 'pending');
    el.querySelector('.check-icon').textContent = ok ? '✓' : '○';
    if (ok) done++;
  }
  setCk('ck-title', title.length >= 1 && title.length <= 200);
  setCk('ck-tags', tagList.length >= 1);
  setCk('ck-content', content.length >= 10);
  setCk('ck-summary', summary.length > 0);
  $('check-count').textContent = `${done} / 4`;
}

(async () => {
  await renderNavbar();
  const me = await requireLogin();

  const params = new URLSearchParams(location.search);
  const editId = params.get('id');
  const isEdit = !!editId;

  $('page-title').textContent = isEdit ? '编辑文章' : '撰写新稿';
  $('kicker').textContent = isEdit ? '⊹  编辑  ·  ' + editId : '⊹  新文章  ·  草稿 01';

  let original = null;
  if (isEdit) {
    try {
      original = await get('/articles/' + editId);
      if (original.user_id !== me.id) {
        showToast({ type: 'error', text: '无权操作此文章' });
        location.href = '/list.html';
        return;
      }
      $('title').value = original.title;
      $('summary').value = original.summary || '';
      $('content').value = original.content;
      $('slug').textContent = 'draft://' + original.title.slice(0, 20);
    } catch (err) {
      showToast({ type: 'error', text: err.message || '加载失败' });
      location.href = '/list.html';
      return;
    }
  }

  // Tag input handling (define before draft restore)
  const tagInput = $('tag-input');
  const tagsHidden = $('tags');
  let tagList = [];
  function renderTags() {
    Array.from($('tag-wrap').children).forEach(ch => {
      if (ch !== tagInput && ch !== tagsHidden) ch.remove();
    });
    tagList.forEach(t => {
      const chip = document.createElement('span');
      chip.className = 'tag-chip';
      chip.style.cssText = 'margin-right:0.5rem';
      chip.textContent = t;
      chip.title = '点击移除';
      chip.style.cursor = 'pointer';
      chip.addEventListener('click', () => {
        tagList = tagList.filter(x => x !== t);
        tagsHidden.value = tagList.join(',');
        renderTags();
        updateStatus();
      });
      tagInput.parentNode.insertBefore(chip, tagInput);
    });
  }

  const draftKey = 'draft:' + (editId || 'new');
  const saved = localStorage.getItem(draftKey);
  if (!isEdit && saved && !$('content').value) {
    if (confirm('检测到上次未保存的草稿，是否恢复？')) {
      const d = JSON.parse(saved);
      $('title').value = d.title || '';
      $('summary').value = d.summary || '';
      $('content').value = d.content || '';
      if (d.tags) {
        tagList = d.tags.split(/[,，]/).map(t => t.trim()).filter(Boolean);
        $('tags').value = tagList.join(',');
        renderTags();
      }
    }
  }

  // Load original tags in edit mode
  if (isEdit && original && original.tags) {
    tagList = original.tags.slice();
    $('tags').value = tagList.join(',');
    renderTags();
  }

  let draftTimer = null;
  function saveDraft() {
    localStorage.setItem(draftKey, JSON.stringify({
      title: $('title').value,
      summary: $('summary').value,
      tags: $('tags') ? $('tags').value : '',
      content: $('content').value,
      at: Date.now(),
    }));
    $('saved-txt').textContent = '草稿已同步 · ' + new Date().toTimeString().slice(0, 8);
    $('sb-status').textContent = '已自动保存到本机草稿';
    $('sb-dot').style.color = '#D4FF3D';
    setTimeout(() => {
      $('sb-status').textContent = '';
      $('sb-dot').style.color = '#5A5C68';
    }, 2000);
  }

  function onInput() {
    clearTimeout(draftTimer);
    draftTimer = setTimeout(saveDraft, 10000);
    updateGutter($('content'));
    updateStatus();
    $('slug').textContent = 'draft://' + ($('title').value || '未命名').slice(0, 20);
  }
  $('title').addEventListener('input', onInput);
  $('summary').addEventListener('input', onInput);
  $('content').addEventListener('input', onInput);
  $('content').addEventListener('click', () => updateGutter($('content')));
  $('content').addEventListener('keyup', () => updateGutter($('content')));

  $('btn-bold').addEventListener('click', () => insertText($('content'), '****'));
  $('btn-italic').addEventListener('click', () => insertText($('content'), '**'));
  $('btn-code').addEventListener('click', () => insertText($('content'), '\n```\n\n```\n'));
  $('btn-link').addEventListener('click', () => insertText($('content'), '[](url)'));
  $('btn-h2').addEventListener('click', () => insertText($('content'), '\n## \n'));
  $('btn-quote').addEventListener('click', () => insertText($('content'), '\n> \n'));

  tagInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      const v = tagInput.value.trim();
      if (v && !tagList.includes(v) && tagList.length < 5) {
        tagList.push(v);
        tagsHidden.value = tagList.join(',');
        tagInput.value = '';
        renderTags();
        updateStatus();
      }
    }
  });

  async function uploadImage(file) {
    if (file.size > 5 * 1024 * 1024) {
      showToast({ type: 'error', text: '文件过大（≤ 5MB）' });
      throw new Error('too large');
    }
    const fd = new FormData();
    fd.append('file', file);
    $('sb-status').textContent = '上传中…';
    try {
      const data = await post('/uploads/image', fd);
      $('sb-status').textContent = '上传完成';
      // Show in uploads card
      const upCard = $('uploads-card');
      upCard.style.display = 'flex';
      const upList = $('upload-list');
      const item = document.createElement('div');
      item.style.cssText = 'display:flex;align-items:center;justify-content:space-between;font-size:0.8125rem;color:#9A9CA8';
      item.innerHTML = `<span>${file.name}</span><span style="color:#D4FF3D;font-family:JetBrains Mono,monospace;font-size:0.6875rem">已上传</span>`;
      upList.appendChild(item);
      return data.url;
    } catch (err) {
      $('sb-status').textContent = '上传失败';
      if (err.code === 1020) showToast({ type: 'error', text: '文件类型不支持' });
      else if (err.code === 1021) showToast({ type: 'error', text: '文件过大（≤ 5MB）' });
      else showToast({ type: 'error', text: err.message || '上传失败' });
      throw err;
    }
  }

  $('btn-image').addEventListener('click', () => $('file-input').click());
  $('file-input').addEventListener('change', async (e) => {
    const file = e.target.files[0];
    if (!file) return;
    try {
      const url = await uploadImage(file);
      insertText($('content'), `\n![](${url})\n`);
    } catch {
      $('file-input').value = '';
    }
  });

  $('content').addEventListener('paste', async (e) => {
    const items = Array.from(e.clipboardData.items);
    const imageItem = items.find(it => it.type.startsWith('image/'));
    if (!imageItem) return;
    e.preventDefault();
    const file = imageItem.getAsFile();
    if (!file) return;
    try {
      const url = await uploadImage(file);
      insertText($('content'), `\n![](${url})\n`);
    } catch {}
  });

  $('content').addEventListener('keydown', (e) => {
    if (e.key === 'Tab') {
      e.preventDefault();
      insertText($('content'), '  ');
    }
  });

  $('cancel-btn').addEventListener('click', (e) => {
    if ($('title').value || $('content').value) {
      if (!confirm('有未保存的内容，确定要离开吗？')) {
        e.preventDefault();
      }
    }
  });

  $('publish-btn').addEventListener('click', async () => {
    const title = $('title').value.trim();
    const summary = $('summary').value.trim();
    const content = $('content').value;
    const tags = tagList;

    if (title.length < 1 || title.length > 200) {
      showToast({ type: 'error', text: '标题长度应为 1–200 字符' });
      $('title').focus();
      return;
    }
    if (content.length < 10 || content.length > 100000) {
      showToast({ type: 'error', text: '正文长度应为 10–100000 字符' });
      $('content').focus();
      return;
    }

    $('publish-btn').disabled = true;
    $('publish-btn').textContent = '发布中…';

    try {
      let articleId = editId;
      if (isEdit) {
        await put('/articles/' + editId, { title, content, summary, tags });
      } else {
        const data = await post('/articles', { title, content, summary, tags });
        articleId = data.id;
      }
      localStorage.removeItem(draftKey);
      location.href = '/detail.html?id=' + articleId;
    } catch (err) {
      $('publish-btn').disabled = false;
      $('publish-btn').textContent = '发布';
      showToast({ type: 'error', text: err.message || '发布失败' });
    }
  });

  $('draft-btn').addEventListener('click', () => {
    saveDraft();
    showToast({ type: 'success', text: '草稿已保存' });
  });

  updateGutter($('content'));
  updateStatus();
})();
