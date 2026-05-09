import { getCurrentUser } from '../auth.js';
import { post } from '../api.js';
import { showToast } from '../components/toast.js';

(async () => {
  const me = await getCurrentUser();
  if (me) {
    location.href = '/list.html';
    return;
  }

  const form = document.getElementById('register-form');
  const submitBtn = document.getElementById('submit-btn');
  const hints = {
    username: document.getElementById('username-hint'),
    name: document.getElementById('name-hint'),
    password: document.getElementById('password-hint'),
    password2: document.getElementById('password2-hint'),
  };

  function setHint(key, msg, isOk = false) {
    hints[key].textContent = msg;
    hints[key].classList.toggle('ok', isOk);
  }

  const usernameRe = /^[a-zA-Z0-9_]{4,32}$/;
  const passwordRe = /^(?=.*[A-Za-z])(?=.*\d)[^]{8,64}$/;

  function validateAll() {
    let ok = true;
    const username = form.username.value.trim();
    const name = form.name.value.trim();
    const password = form.password.value;
    const password2 = form.password2.value;

    if (!usernameRe.test(username)) {
      setHint('username', '用户名应为 4–32 位字母、数字或下划线'); ok = false;
    } else {
      setHint('username', '');
    }

    if (name.length < 1 || name.length > 64) {
      setHint('name', '姓名长度应为 1–64 位'); ok = false;
    } else {
      setHint('name', '');
    }

    if (!passwordRe.test(password)) {
      setHint('password', '密码需 8–64 位且同时包含字母和数字'); ok = false;
    } else {
      setHint('password', '');
    }

    if (password2 !== password) {
      setHint('password2', '两次输入的密码不一致'); ok = false;
    } else if (password2) {
      setHint('password2', '');
    } else {
      setHint('password2', ''); ok = false;
    }

    return ok;
  }

  ['username', 'name', 'password', 'password2'].forEach(key => {
    form[key].addEventListener('blur', validateAll);
  });

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!validateAll()) return;

    submitBtn.disabled = true;
    submitBtn.innerHTML = '<span>创建中…</span>';

    try {
      await post('/auth/register', {
        username: form.username.value.trim(),
        name: form.name.value.trim(),
        password: form.password.value,
      });
      showToast({ type: 'success', text: '注册成功，自动登录中…' });
      try {
        await post('/auth/login', { username: form.username.value.trim(), password: form.password.value });
        setTimeout(() => { location.href = '/list.html'; }, 400);
      } catch {
        location.href = '/login.html';
      }
    } catch (err) {
      submitBtn.disabled = false;
      submitBtn.innerHTML = '<span>创建主页</span><span>⊕</span>';
      if (err.code === 1010) {
        setHint('username', '用户名已被使用');
        form.username.focus();
      } else if (err.code === 1001) {
        showToast({ type: 'error', text: '输入有误，请检查' });
      } else if (err.code === 2020) {
        showToast({ type: 'error', text: '操作过于频繁，请稍后再试' });
        submitBtn.disabled = true;
        setTimeout(() => { submitBtn.disabled = false; }, 5000);
      } else {
        showToast({ type: 'error', text: err.message || '注册失败' });
      }
    }
  });
})();
