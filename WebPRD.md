# WebPRD — 博客平台前端 PRD

> 面向：前端开发者
> 配套后端文档：`Server.md`
> 代码目录：`/Web/`
> 日期：2026-05-01

## 1. 背景与目标

构建一个多用户博客平台的前端：任何人可注册账号写自己的文章，已发布文章对所有人公开可读。前端使用纯 HTML5 + CSS3 + 原生 ES 模块化 JavaScript，零构建步骤，由后端 Go 服务以静态资源形式直接服务。

### 1.1 成功标准
- 所有页面无构建工具直接通过浏览器打开（开发期 file:// 也能跑通主路径）
- 移动端最小可用（375px 起不破版，但不做深度响应式）
- 主流 Chrome / Edge / Firefox 最新版上无报错
- 页面平均 LCP < 1.5s（开发机 + 本机后端）

### 1.2 不做（YAGNI）
- 单页应用（SPA）路由
- 实时 Markdown 预览
- 评论、搜索、第三方登录
- 深度移动端适配、PWA、暗色模式
- 任何打包工具（webpack/vite/rollup 等）

## 2. 技术约束

| 项 | 选型 |
|---|---|
| HTML | HTML5 原生 |
| CSS | CSS3 + 自写样式（不引入 Tailwind/Bootstrap） |
| JS | 原生 ES2020 模块化（`<script type="module">`） |
| 第三方库 | 仅 `marked.js`（Markdown 渲染）、`highlight.js`（代码高亮），离线引入到 `vendor/` |
| API 通信 | `fetch` + JSON，凭证 `credentials: 'include'` |
| 鉴权 | 后端 Session Cookie，前端无需自管理 token |

## 3. 目录结构

```
Web/
├─ index.html              # 入口，重定向到 list.html
├─ login.html              # 登录页
├─ register.html           # 注册页
├─ list.html               # 文章列表（首页）
├─ detail.html             # 文章详情
├─ editor.html             # 编辑器（新建+编辑共用）
├─ profile.html            # 作者个人中心
├─ assets/
│  ├─ css/
│  │  └─ main.css          # 全局样式
│  └─ js/
│     ├─ api.js            # fetch 封装、错误处理、CSRF
│     ├─ auth.js           # 登录态判断、跳转
│     ├─ markdown.js       # 把 Markdown 字符串渲染成 HTML
│     ├─ pages/
│     │  ├─ login.js
│     │  ├─ register.js
│     │  ├─ list.js
│     │  ├─ detail.js
│     │  ├─ editor.js
│     │  └─ profile.js
│     └─ components/
│        ├─ navbar.js      # 顶部导航条（注入 DOM）
│        ├─ pager.js       # 分页器组件
│        └─ toast.js       # 全局提示
└─ vendor/
   ├─ marked.min.js
   ├─ highlight.min.js
   └─ github-md.css
```

## 4. 用户流程

### 4.1 未登录访问者
```
直达 list.html → 浏览列表 → 点击文章 → detail.html 阅读
                                   ↓
              点「写文章」/ 头像 → 跳 login.html
```

### 4.2 已登录用户
```
list.html ─ 写文章 ─→ editor.html?new
         ─ 头像菜单 ─→ profile.html / 退出登录
detail.html（自己的文章）─ 编辑按钮 ─→ editor.html?id=xxx
```

## 5. 模块详细设计

### 5.1 登录模块（login.html）

**字段**
- 用户名（username）：必填，4–32 字符
- 密码（password）：必填，8–64 字符

**按钮**
- 主：「登录」
- 次：「没有账号？去注册」 → 跳 `register.html`

**交互**
1. 提交时禁用按钮，显示 loading
2. 调用 `POST /api/v1/auth/login`
3. 成功 → 跳转到 URL 参数 `?redirect=` 指定的页面，否则跳 `list.html`
4. 失败 → toast 显示错误码对应文案，按钮恢复

**校验**
- 前端只校验非空和长度，强密码规则交后端兜底

**已登录守卫**
- 进页前先 `GET /api/v1/auth/me`，已登录直接跳 `list.html`，避免重复登录

### 5.2 注册模块（register.html）

**字段**
- 用户名 username：4–32 字符，仅 `[a-zA-Z0-9_]`，纯用户名无后缀
- 密码 password：8–64 字符，至少包含字母 + 数字
- 姓名/昵称 name：1–64 字符，允许中文
- 确认密码 password2：与 password 一致

**前端实时校验**
- 失焦时显示规则错误
- 用户名 onBlur 后调 `GET /api/v1/auth/check?username=xx`（可选优化，本期跳过，统一交提交时报错）

**提交行为**
1. `POST /api/v1/auth/register`
2. 成功后**自动登录**（后端注册接口同时种 Session Cookie）→ 跳 `list.html`
3. 用户名已存在 → toast「用户名已被使用」，焦点回到用户名

### 5.3 列表模块（list.html，首页）

**布局**
```
┌────────────────────────────────────────────────────────────────────┐
│ [Logo] [全部 标签1 标签2 ...]   [写文章] [昵称▾] [登出]              │  ← 顶栏（已登录）
│ [Logo] [全部 标签1 标签2 ...]                        [跳转到登录页]│  ← 顶栏（未登录）
├────────────────────────────────────────────────────────────────────┤
│ ┌──────────────────────────────┐                                   │
│ │ 标题（粗，h2）                  │                                   │
│ │ 摘要前 80 字...                  │                                   │
│ │ 作者 · 标签 · 浏览 123 · 2 天前  │                                   │
│ └──────────────────────────────┘                                   │
│ ...                                                                  │
│              ‹ 1 2 3 4 5 ›                                           │
└────────────────────────────────────────────────────────────────────┘
```

**数据**
- `GET /api/v1/articles?page=1&size=10&tag=xxx`
- `GET /api/v1/tags` 一次拉取所有标签放顶栏

**交互**
- 点击卡片 → `detail.html?id={id}`
- 点击标签 chip → `list.html?tag={name}`
- 「写文章」按钮：未登录隐藏；已登录跳 `editor.html`
- 「昵称▾」：已登录显示，下拉「我的主页」
- **「登出」按钮**：已登录时与昵称并列显示在顶栏右侧（**显式按钮，不藏在下拉里**），点击：
  1. `POST /api/v1/auth/logout`
  2. 清空内存中的 user 状态
  3. 重新刷新当前页（顶栏切换为未登录视图）
- 「跳转到登录页」按钮：未登录显示，跳 `login.html?redirect=/list.html`

**摘要**
- 后端列表接口返回 `summary` 字段（content 去 Markdown 标记后取前 200 字符），前端不做二次处理

### 5.4 详情模块（detail.html）

**URL** `detail.html?id=123`

**数据加载**
1. `GET /api/v1/articles/123` → 此调用即触发后端浏览计数 +1
2. 渲染：标题、作者（点击跳 `profile.html?id={author_id}`）、发布时间、标签
3. content 字段交给 `markdown.js` 渲染为 HTML，再用 `highlight.js` 处理 `<pre><code>`

**编辑入口**
- 若 `current_user.id === article.user_id`，右上显示「编辑」「删除」按钮
- 编辑 → `editor.html?id=123`
- 删除 → 二次确认弹窗 → `DELETE /api/v1/articles/123` → 跳 `list.html`

**安全**
- 渲染前用 marked.js 默认 sanitize 选项，禁止裸 `<script>`
- 不直接 innerHTML 用户名等字段，使用 `textContent`

### 5.5 编辑器模块（editor.html）

**URL**
- `editor.html`：新建
- `editor.html?id=123`：编辑指定文章（必须本人）

**布局**
```
┌────────────────────────────────────────────┐
│ [Logo]                          [取消] [发布]│
├────────────────────────────────────────────┤
│ 标题: [_____________________________]       │
│ 标签: [tag1, tag2, tag3] (逗号分隔)          │
│ ┌──────────────────────────────┐           │
│ │ 工具栏: [图片] [代码] [链接] [B] [I]      │
│ ├──────────────────────────────┤           │
│ │                                │           │
│ │   <textarea  Markdown 内容>     │           │
│ │                                │           │
│ └──────────────────────────────┘           │
└────────────────────────────────────────────┘
```

**字段**
- title：1–200 字符
- tags：以英文/中文逗号分隔，最多 5 个
- content：Markdown 文本，10–100000 字符

**关键交互**

a) **图片按钮**
- 弹起 `<input type="file" accept="image/*">`
- 选中后 `POST /api/v1/uploads/image` (multipart)
- 拿到 url，往光标位置插入 `\n![](url)\n`

b) **粘贴上传**
- textarea 上 `paste` 事件 → 检查 `e.clipboardData.items` 中的 `image/*`
- 有则 preventDefault，转 Blob 上传，行为同 a)
- 上传期间在底部展示「上传中…」状态

c) **Tab 缩进**
- 拦截 Tab 键，插入两空格而不是切换焦点

d) **保存草稿**
- 仅保存到 localStorage（key: `draft:<id-or-new>`），10 秒节流
- 进入页面时若发现草稿且后端无 id，提示「检测到上次未保存的草稿，是否恢复」

e) **发布**
- 校验 → `POST /api/v1/articles`（新建）或 `PUT /api/v1/articles/:id`（编辑）
- 成功 → 清草稿 → 跳 `detail.html?id=xxx`

f) **取消**
- 若有未保存修改，确认弹窗

### 5.6 个人中心（profile.html）

**URL** `profile.html?id=123`（无 id 默认当前登录用户）

**展示**
- 顶部：头像（首字母占位）、昵称、用户名、注册时间、文章总数、总浏览数
- 列表：该作者的文章（同 list.html 卡片样式），分页 `GET /api/v1/users/:id/articles`

**自己的主页 vs 别人的主页**
- 自己：每篇卡片右下显示「编辑/删除」
- 别人：纯展示

## 6. 公共能力

### 6.1 `assets/js/api.js`

```javascript
// 伪代码示意
const BASE = '/api/v1';

export async function request(path, opts = {}) {
  const csrf = getCookie('csrf_token');
  const res = await fetch(BASE + path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': csrf,
      ...(opts.headers || {}),
    },
    ...opts,
  });
  const json = await res.json();
  if (json.code === 0) return json.data;
  if (json.code === 2001) {
    showToast({ type: 'error', text: '登录已过期，请重新登录' });
    location.href = '/login.html?redirect=' + encodeURIComponent(location.pathname + location.search);
    throw json;
  }
  throw json;
}
export const get = (p) => request(p);
export const post = (p, body) => request(p, { method: 'POST', body: JSON.stringify(body) });
export const put = (p, body) => request(p, { method: 'PUT', body: JSON.stringify(body) });
export const del = (p) => request(p, { method: 'DELETE' });
```

### 6.2 `assets/js/auth.js`

```javascript
// requireLogin() — 在需要登录的页面顶部调用
export async function getCurrentUser() {
  try { return await get('/auth/me'); } catch { return null; }
}
export async function requireLogin() {
  const u = await getCurrentUser();
  if (!u) location.href = '/login.html?redirect=' + encodeURIComponent(location.pathname);
  return u;
}
```

### 6.3 `assets/js/markdown.js`
- 调用 `marked.parse(content)` 得 HTML
- 遍历产物中的 `pre code` 元素，调 `hljs.highlightElement(el)`
- 默认开启 marked 的 sanitize 或 DOMPurify（如引入），本期使用 marked 的默认安全配置

### 6.4 `components/toast.js`
- 全局提示，参数 `{type: 'success'|'error'|'info', text, duration=3000}`
- 右上角弹窗，3s 后自动消失

## 7. 错误处理对照表

| code | 文案（前端展示） | 后续动作 |
|---|---|---|
| 0 | — | 业务继续 |
| 1001 | 输入有误，请检查 | 高亮报错字段 |
| 1010 | 用户名已被使用 | 焦点回输入框 |
| 1020 | 文件类型不支持 | 上传组件复位 |
| 1021 | 文件过大（≤ 5MB） | 同上 |
| 1030 | 资源不存在 | 退回列表或显示 404 占位 |
| 2001 | （静默） | 跳登录页 |
| 2002 | 无权操作此文章 | 退回列表 |
| 2010 | 登录失败，账号或密码错误 | 焦点回密码 |
| 2020 | 操作过于频繁，请稍后再试 | 禁用按钮 5 秒 |
| 2030 | 安全校验失败，请刷新页面重试 | 强制刷新当前页 |
| 5xxx | 系统繁忙，请稍后重试 | 一般 toast |

完整错误码以 `Server.md` 为准。

## 8. 会话与安全

### 8.1 Cookie 会话保持
- 登录或注册成功后，后端下发**带 `Max-Age=1800`（30 分钟）的持久化 Cookie**：`sid`（HttpOnly）+ `csrf_token`（前端可读）
- 关闭浏览器再打开（30 分钟内）→ Cookie 仍然存在 → 进入页面时 `/auth/me` 返回当前用户 → 直接看到登录态视图，**无需再输入账号密码**
- 30 分钟内有任何鉴权请求，后端会同步刷新 Cookie 与 Redis 的过期时间，**滑动续期**：只要保持活跃，会话不会过期
- 30 分钟无任何活跃请求 → Cookie 自动失效 → 下次访问需要登录页面跳转登录页

**前端无需特别处理 Cookie**：浏览器自动管理。前端只需通过 `/auth/me` 探测当前是否登录。

### 8.2 会话过期 UX
- 用户在某个写操作时，发现服务返回 `2001`（Session 已失效）→ `api.js` 拦截统一处理：
  1. toast「登录已过期，请重新登录」
  2. 跳 `/login.html?redirect=<当前页 path+query>`
- 用户在登录页登录成功后，根据 `redirect` 参数跳回原页

### 8.3 登出
- 调用 `POST /api/v1/auth/logout`，后端清 Redis 与下发 `Max-Age=0` 的同名空 Cookie
- 前端在请求成功后立即清空内存中的当前用户对象，并跳 `/list.html`（或刷新当前页）

### 8.4 CSRF 处理
- 后端在 Session 创建时下发 `csrf_token` Cookie（非 HttpOnly）
- 前端所有写请求（POST/PUT/DELETE）的 `X-CSRF-Token` 头取自该 Cookie
- 由 `api.js` 统一处理，业务页面无感
- 校验失败返回 `2030`，前端 toast「安全校验失败，请刷新页面重试」并强制刷新

## 9. 验证清单（前端开发者完成功能后逐条手动跑一遍）

- [ ] 注册 → 自动登录 → 写一篇文章 → 详情页能看到，浏览数为 1
- [ ] 退出登录 → 列表页仍能看到所有文章 → 点详情浏览数 +1
- [ ] 注册同一用户名报「用户名已被使用」
- [ ] 错误密码登录显示「账号或密码错误」
- [ ] 编辑器粘贴一张本地截图，自动插入 `![](url)`
- [ ] 编辑器选择 6MB 图片，被拒，显示「文件过大」
- [ ] 上传 .exe 改后缀，被拒，显示「文件类型不支持」
- [ ] 编辑别人的文章，URL 直接传别人 id，按钮不出现，PUT 接口返回 2002
- [ ] 关闭后端，操作时 toast「系统繁忙」
- [ ] 移动端 Chrome 375px 宽度下能正常浏览首页和详情
- [ ] 登录成功后关闭浏览器再开（5 分钟内）→ 直接进入列表页，未跳登录
- [ ] 静置 31 分钟以上后做任意操作 → toast「登录已过期」并跳 `login.html?redirect=...`，登录后跳回原页
- [ ] 列表页登录态下，点击「登出」按钮 → Cookie 被清除 → 顶栏切换为未登录视图
- [ ] 已登录状态访问 `/login.html` → 自动跳回 `/list.html`，不要求再次输密码

---

文档结束。具体接口契约见 `Server.md` 第 4 章。
