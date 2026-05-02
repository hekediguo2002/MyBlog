function ensureContainer() {
  let el = document.getElementById('toast-container');
  if (!el) {
    el = document.createElement('div');
    el.id = 'toast-container';
    document.body.appendChild(el);
  }
  return el;
}

export function showToast({ type = 'info', text, duration = 3000 }) {
  const container = ensureContainer();
  const toast = document.createElement('div');
  toast.className = `toast ${type}`;
  toast.textContent = text;
  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateX(20px)';
    toast.style.transition = 'opacity .25s, transform .25s';
    setTimeout(() => toast.remove(), 250);
  }, duration);
}
