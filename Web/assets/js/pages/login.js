import { renderNavbar } from '../components/navbar.js';
import { getCurrentUser } from '../auth.js';
import { post } from '../api.js';
import { showToast } from '../components/toast.js';

(async () => {
  const me = await getCurrentUser();
  if (me) {
    const params = new URLSearchParams(location.search);
    const redirect = params.get('redirect') || '/list.html';
    location.href = redirect;
    return;
  }

  const form = document.getElementById('login-form');
  const submitBtn = document.getElementById('submit-btn');
  const usernameHint = document.getElementById('username-hint');
  const passwordHint = document.getElementById('password-hint');

  function setHint(el, msg, isOk = false) {
    el.textContent = msg;
    el.classList.toggle('ok', isOk);
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    setHint(usernameHint, '');
    setHint(passwordHint, '');

    const username = form.username.value.trim();
    const password = form.password.value;

    if (username.length < 4 || username.length > 32) {
      setHint(usernameHint, '用户名长度应为 4–32 位');
      form.username.focus();
      return;
    }
    if (password.length < 8 || password.length > 64) {
      setHint(passwordHint, '密码长度应为 8–64 位');
      form.password.focus();
      return;
    }

    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>登录中…</span>';

    try {
      await post('/auth/login', { username, password });
      const params = new URLSearchParams(location.search);
      const redirect = params.get('redirect') || '/list.html';
      showToast({ type: 'success', text: '登录成功' });
      setTimeout(() => { location.href = redirect; }, 400);
    } catch (err) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '<span>进入主页</span><span>→</span>';
      if (err.code === 2010) {
        setHint(passwordHint, '登录失败，账号或密码错误');
        form.password.focus();
      } else if (err.code === 2020) {
        showToast({ type: 'error', text: '操作过于频繁，请稍后再试' });
        submitBtn.disabled = true;
        setTimeout(() => { submitBtn.disabled = false; }, 5000);
      } else {
        showToast({ type: 'error', text: err.message || '登录失败' });
      }
    }
  });
})();
