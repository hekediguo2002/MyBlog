export function renderPager({ current = 1, total = 1, onChange }, mountId = 'pager-mount') {
  const mount = document.getElementById(mountId);
  if (!mount) return;
  mount.innerHTML = '';

  if (total <= 1) return;

  const wrap = document.createElement('div');
  wrap.className = 'pager';

  const prev = document.createElement('button');
  prev.textContent = '‹';
  prev.disabled = current <= 1;
  prev.addEventListener('click', () => onChange(current - 1));
  wrap.appendChild(prev);

  const maxVisible = 5;
  let start = Math.max(1, current - Math.floor(maxVisible / 2));
  let end = Math.min(total, start + maxVisible - 1);
  if (end - start + 1 < maxVisible) {
    start = Math.max(1, end - maxVisible + 1);
  }

  for (let i = start; i <= end; i++) {
    const btn = document.createElement('button');
    btn.textContent = String(i);
    if (i === current) btn.className = 'active';
    btn.addEventListener('click', () => onChange(i));
    wrap.appendChild(btn);
  }

  const next = document.createElement('button');
  next.textContent = '›';
  next.disabled = current >= total;
  next.addEventListener('click', () => onChange(current + 1));
  wrap.appendChild(next);

  mount.appendChild(wrap);
}
