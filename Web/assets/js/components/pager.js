export function renderPager({ current = 1, total = 1, onChange }, mountId = 'pager-mount') {
  const mount = document.getElementById(mountId);
  if (!mount) return;
  mount.innerHTML = '';

  if (total <= 1) return;

  const wrap = document.createElement('div');
  wrap.className = 'pager';

  const info = document.createElement('div');
  info.className = 'pager-info';
  const start = (current - 1) * 10 + 1;
  const end = Math.min(current * 10, total * 10); // approximate
  info.innerHTML = `<span>显示</span><span class="cur">${start}–${end}</span><span>共 ${total} 页</span>`;
  wrap.appendChild(info);

  const btns = document.createElement('div');
  btns.className = 'pager-btns';

  const prev = document.createElement('button');
  prev.textContent = '‹';
  prev.disabled = current <= 1;
  prev.addEventListener('click', () => onChange(current - 1));
  btns.appendChild(prev);

  const maxVisible = 5;
  let startPage = Math.max(1, current - Math.floor(maxVisible / 2));
  let endPage = Math.min(total, startPage + maxVisible - 1);
  if (endPage - startPage + 1 < maxVisible) {
    startPage = Math.max(1, endPage - maxVisible + 1);
  }

  for (let i = startPage; i <= endPage; i++) {
    const btn = document.createElement('button');
    btn.textContent = String(i);
    if (i === current) btn.className = 'active';
    btn.addEventListener('click', () => onChange(i));
    btns.appendChild(btn);
  }

  const next = document.createElement('button');
  next.textContent = '›';
  next.disabled = current >= total;
  next.addEventListener('click', () => onChange(current + 1));
  btns.appendChild(next);

  wrap.appendChild(btns);
  mount.appendChild(wrap);
}
