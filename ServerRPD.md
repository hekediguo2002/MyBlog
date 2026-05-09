# Server.md — 博客平台后端 PRD
> 面向：Go 后端开发者
> 配套前端文档：`WebPRD.md`
> 代码目录：`/Server/`
> 日期：2026-05-09

## 1. 背景与目标

为一个多用户博客平台提供后端服务：账号注册登录、文章 CRUD、按标签过滤、按作者展示、浏览计数、图片上传。预期规模 1K QPS 以下，重点是代码并发安全、查询性能起点正确、易于后续扩展。

### 1.1 成功标准
- 单进程 Go 服务在 4 核 8G 机器上稳定支持 ≥ 500 QPS 的列表+详情读
- 浏览计数在 1K 并发模拟刷新下不丢失（最多 30 秒延迟回写）
- 服务挂掉重启 30 秒内恢复，未回写计数最多丢一个聚合周期
- 单元测试覆盖核心业务 ≥ 60%；集成测试覆盖全部 API 主路径

### 1.2 非目标（YAGNI）
- 多机分布式部署、读写分离
- 文章全文搜索
- 评论系统
- 第三方登录、OAuth
- 微服务拆分

## 2. 技术栈

| 项 | 选型 | 备注 |
|---|---|---|
| 语言 | Go 1.22+ | |
| Web 框架 | `gin-gonic/gin` | |
| ORM | `gorm.io/gorm` + `gorm.io/driver/postgres` | |
| 数据库 | PostgreSQL 16 | |
| 缓存/计数 | Redis 7（`go-redis/redis/v9`） | |
| Session | `gin-contrib/sessions` + Redis store | |
| 密码 | `golang.org/x/crypto/bcrypt`（cost=12） | |
| 日志 | `rs/zerolog`，JSON 输出，按天滚动 | |
| 配置 | `spf13/viper`，YAML + 环境变量覆盖 | |
| 测试 | `stretchr/testify` + `testcontainers-go` | |
| 工具 | `Makefile`、`docker-compose.yml`（仅本地依赖） | |

## 3. 项目结构

```
Server/
├─ cmd/server/main.go              # 入口，加载配置 / 初始化 / 启动
├─ internal/
│  ├─ config/                      # 配置结构 + 加载
│  ├─ db/                          # PostgreSQL 初始化
│  ├─ cache/                       # Redis 初始化
│  ├─ middleware/
│  │  ├─ recover.go
│  │  ├─ logger.go
│  │  ├─ session.go
│  │  ├─ csrf.go
│  │  ├─ auth.go                   # 鉴权中间件（RequireAuth、RequireAdmin）
│  │  └─ ratelimit.go
│  ├─ model/                       # GORM 模型
│  │  ├─ user.go
│  │  ├─ article.go
│  │  ├─ tag.go
│  │  └─ article_tag.go
│  ├─ repository/                  # 仓储接口 + 实现（数据访问层）
│  │  ├─ user_repo.go
│  │  ├─ article_repo.go
│  │  ├─ tag_repo.go
│  │  └─ counter_repo.go           # Redis 浏览计数
│  ├─ service/                     # 业务层
│  │  ├─ auth_service.go
│  │  ├─ admin_service.go          # 管理员业务（用户列表、级联删除）
│  │  ├─ article_service.go
│  │  ├─ tag_service.go
│  │  ├─ upload_service.go
│  │  └─ view_counter_service.go   # 计数回写后台
│  ├─ handler/                     # HTTP 入口（gin Handler）
│  │  ├─ auth_handler.go
│  │  ├─ admin_handler.go          # 管理员 API（ListUsers、DeleteUser）
│  │  ├─ article_handler.go
│  │  ├─ tag_handler.go
│  │  └─ upload_handler.go
│  ├─ router/                      # 路由组装
│  │  └─ router.go
│  ├─ apperr/                      # 统一错误码与错误类型
│  │  └─ errors.go
│  └─ pkg/
│     ├─ httpresp/                 # 统一响应外壳
│     ├─ password/                 # bcrypt 工具
│     ├─ idgen/                    # uuid
│     └─ markdownx/                # 取摘要等小工具
├─ migrations/                     # SQL 迁移脚本（手写）
│  ├─ 001_init.up.sql
│  └─ 001_init.down.sql
├─ uploads/                        # 上传文件根目录（运行时生成）
├─ config.yaml                     # 默认配置
├─ go.mod / go.sum
├─ Makefile
└─ docker-compose.yml              # 本地起 PostgreSQL + Redis
```

### 3.1 分层职责
- **handler**：参数绑定、调用 service、组织响应。**禁止直接访问 DB**。
- **service**：业务规则与事务编排。**唯一持有 repository 接口的层**。
- **repository**：仅做数据访问，无业务判断。GORM 写在这一层。
- **model**：GORM 表映射结构体，可加方法但不带业务依赖。

## 4. API 契约

> 所有响应外层：`{"code": <int>, "msg": <string>, "data": <object|array|null>}`，`code = 0` 表示成功。错误码见 §7。
>
> 时间格式：RFC3339（如 `2026-05-01T16:42:00+08:00`）。

### 4.1 认证

#### POST /api/v1/auth/register
注册并自动登录。

请求：
```json
{
  "username": "alice",
  "password": "Password123",
  "name": "爱丽丝"
}
```
响应 data：
```json
{ "id": 1, "username": "alice", "name": "爱丽丝", "isAdmin": false }
```
副作用：注册成功后立即建立 Session，与 `/auth/login` 同样下发 `sid` Cookie + `csrf_token` Cookie。
错误：`1001` 参数无效；`1010` 用户名已存在。

#### POST /api/v1/auth/login
请求：
```json
{ "username": "alice", "password": "Password123" }
```
响应 data：
```json
{ "id": 1, "username": "alice", "name": "爱丽丝", "isAdmin": false }
```
**管理员登录**：内置固定账号 `sysadmin` / `admin111`，不在数据库中。登录后 `isAdmin` 为 `true`、`id=0`、`name="系统管理员"`，Session 中 `IsAdmin=true`。管理员不注册、不出现于用户列表。
副作用：种 Session Cookie + CSRF Cookie。
错误：`2010` 账号或密码错误；`2020` 频次超限。

#### POST /api/v1/auth/logout
鉴权（中间件可放宽：未登录直接当成功处理，幂等）。请求体：空。响应 data：null。
副作用：删除 Redis `sess:<sid>` 与 `csrf:<sid>`；下发 `Max-Age=0` 的 `sid` 与 `csrf_token` 同名空 Cookie 强制浏览器清除。

#### GET /api/v1/auth/me
未登录返回 `2001`；已登录返回当前用户（含 `isAdmin` 字段）。管理员登录时返回 `{"id":0, "username":"sysadmin", "name":"系统管理员", "isAdmin":true}`。

### 4.2 文章

#### GET /api/v1/articles
分页列表。Query：
- `page`（默认 1，≥1）
- `size`（默认 10，1–50）
- `tag`（可选）
- `user_id`（可选；当 `/users/:id/articles` 路由内部转发）

响应 data：
```json
{
  "total": 123,
  "list": [
    {
      "id": 10,
      "title": "Hello",
      "summary": "去 Markdown 标记后的前 200 字...",
      "view_count": 999,
      "tags": ["go", "blog"],
      "author": { "id": 1, "name": "爱丽丝" },
      "created_at": "2026-04-30T10:00:00+08:00"
    }
  ]
}
```

#### GET /api/v1/articles/:id
详情。**触发浏览计数 +1**：执行 `INCR view:<id>` 与 `SADD view:dirty <id>`。
响应 data：
```json
{
  "id": 10,
  "title": "Hello",
  "content": "# Markdown 原文…",
  "view_count": 999,
  "tags": ["go", "blog"],
  "author": { "id": 1, "name": "爱丽丝" },
  "created_at": "...",
  "updated_at": "..."
}
```
注：返回的 `view_count` = `articles.view_count + view:<id> in Redis`，对调用方透明。

#### POST /api/v1/articles
鉴权。请求：
```json
{ "title": "Hello", "content": "# md", "tags": ["go", "blog"] }
```
响应 data：`{ "id": 10 }`。

#### PUT /api/v1/articles/:id
鉴权 + 仅作者本人。请求体同上。错误 `2002` 无权操作。

#### DELETE /api/v1/articles/:id
鉴权 + 仅作者本人。软删（写 `deleted_at`）。

#### GET /api/v1/users/:id/articles
等价于 `/articles?user_id=...`，已发布文章。

### 4.3 标签

#### GET /api/v1/tags
所有标签 + 文章数。
响应 data：
```json
[ { "id": 1, "name": "go", "article_count": 12 }, ... ]
```

### 4.4 上传

#### POST /api/v1/uploads/image
鉴权。`multipart/form-data`，字段 `file`。
- 接收 MIME 必须以 `image/` 开头
- 后缀白名单：`jpg, jpeg, png, gif, webp`
- 大小 ≤ 5 MB
- 保存到 `uploads/{yyyy}/{mm}/{uuid}.{ext}`
- 返回 URL：`/uploads/{yyyy}/{mm}/{uuid}.{ext}`

响应 data：`{ "url": "/uploads/2026/05/abc.png" }`。
错误：`1020` 类型不支持；`1021` 文件过大。

### 4.5 管理员

#### GET /api/v1/admin/users
鉴权：管理员。返回所有注册用户列表。
响应 data：
```json
[
  {"id": 2, "username": "alice", "name": "Alice", "created_at": 1714521600},
  {"id": 3, "username": "bob", "name": "Bob", "created_at": 1714608000}
]
```
错误：`2001` 未登录；`2002` 非管理员。

#### DELETE /api/v1/admin/users/:id
鉴权：管理员。级联硬删除该用户及其全部文章（使用数据库事务，不可恢复）。
响应 data：null。
错误：`1030` 用户不存在；`2001` 未登录；`2002` 非管理员。

### 4.6 静态资源
- `GET /uploads/*` → `./uploads/`（gin.Static）
- `GET /` → `./Web/index.html`
- `GET /<file>` → `./Web/<file>`（front 由 gin.NoRoute 兜底，找不到再 404）

## 5. 数据模型

### 5.1 SQL 定义

```sql
CREATE TABLE users (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username      VARCHAR(32)     NOT NULL,
  password_hash CHAR(60)        NOT NULL,
  name          VARCHAR(64)     NOT NULL,
  created_at    DATETIME        NOT NULL,
  updated_at    DATETIME        NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE articles (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id     BIGINT UNSIGNED NOT NULL,
  title       VARCHAR(200)    NOT NULL,
  content     MEDIUMTEXT      NOT NULL,
  view_count  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status      TINYINT         NOT NULL DEFAULT 1,  -- 1 已发布
  created_at  DATETIME        NOT NULL,
  updated_at  DATETIME        NOT NULL,
  deleted_at  DATETIME        NULL,
  PRIMARY KEY (id),
  KEY idx_user_id (user_id),
  KEY idx_status_created (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE tags (
  id   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(32)     NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE article_tags (
  article_id BIGINT UNSIGNED NOT NULL,
  tag_id     BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (article_id, tag_id),
  KEY idx_tag_article (tag_id, article_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 5.2 GORM 模型映射要点
- 软删：`articles.deleted_at` 用 `gorm.DeletedAt`，所有查询自动过滤
- 关联：`Article.Tags []Tag` 用 `many2many:article_tags`
- `User`、`Article` 都内嵌 `CreatedAt`、`UpdatedAt`，由 GORM 维护

### 5.3 Redis 键

| Key | 类型 | TTL | 说明 |
|---|---|---|---|
| `sess:<sid>` | Hash | 30 分钟（每次请求滑动续期） | 用户会话 |
| `csrf:<sid>` | String | 30 分钟（同步续期） | CSRF token |
| `view:<article_id>` | Integer | 永不过期，回写后值归 0 | 增量计数 |
| `view:dirty` | Set | 永不过期 | 待回写文章 id 集合 |
| `rl:login:<ip>` | Integer | 60s | 登录限流计数 |
| `rl:upload:<uid>` | Integer | 60s | 上传限流计数 |

## 6. 关键设计

### 6.1 认证与会话

- **会话时长策略**：30 分钟滑动过期（idle timeout）。每一次通过鉴权中间件的请求都把 Session TTL 与 CSRF TTL 重置为 30 分钟。30 分钟内无任何请求 → Redis 键自然过期 → Cookie 失效，下次访问跳登录页。
- **持久化 Cookie**：登录成功时下发 Cookie 带 `Max-Age=1800`（30 分钟），关闭浏览器后再打开仍可在 30 分钟有效期内免登录直接进入列表。续期时同步刷新 Cookie 的 `Max-Age`，让浏览器侧也跟着延长。
- **登录流程**：
  1. handler 校验入参 → 先判断是否为 sysadmin/admin111 硬编码账号（是则直接创建管理员 Session，跳过 DB 查询）
  2. 否则 service 取 user → bcrypt.CompareHashAndPassword
  3. 生成 sid（uuid） → 写 Redis `sess:<sid>`（EXPIRE 1800s）存 `{user_id, name, isAdmin}`
  4. 生成 csrf token，写 `csrf:<sid>`（EXPIRE 1800s）
  5. 同步两个 Cookie（均设置 `Max-Age=1800`、`Path=/`）：
     - `sid`：HttpOnly, Secure(prod), SameSite=Lax
     - `csrf_token`：非 HttpOnly（前端 JS 可读），SameSite=Lax
- **Session 结构**：`{user_id uint64, name string, isAdmin bool}`。`isAdmin` 由登录时写入，中间件读取后注入 Gin Context。
- **管理员中间件 `RequireAdmin`**：从 Context 取出 Session，若 `!IsAdmin` 返回 `2002`。管理员 API 路由组统一使用此中间件。
- **退出**：`POST /api/v1/auth/logout` 删 Redis 对应键，下发 `Max-Age=0` 的同名空 Cookie 让浏览器清除
- **鉴权中间件 `auth.go`**：
  ```text
  读 sid Cookie → Redis 查 session
    存在 → EXPIRE sess:<sid> 1800; EXPIRE csrf:<sid> 1800; 重新下发 Set-Cookie 续期 Max-Age=1800
          → 注入 user 到 c.Set("user", ...)
    不存在 → 返回 2001
  ```
  滑动续期由中间件统一处理，handler 无感。
- **CSRF 中间件 `csrf.go`**：仅作用于写方法（POST/PUT/DELETE），比对 Cookie `csrf_token` 与 Header `X-CSRF-Token` 是否一致

### 6.2 浏览计数（高并发热点）

**写入路径**（`GET /articles/:id` 命中详情）：
```go
counter.Inc(ctx, articleID)   // INCR view:<id>; SADD view:dirty <id>
```
两个命令用 pipeline 一次发出，错误降级为 log + 继续返回详情，不阻塞主流程。

**回写后台 `view_counter_service.go`**：
- 启动时 `go svc.runFlushLoop(ctx)`，每 30 秒触发一次：
  1. 获取待回写 id：`SMEMBERS view:dirty`（脏集合预期百量级以内；若运行后远超此规模，改用 `SSCAN view:dirty COUNT 1000` 分批扫描）
  2. 对每个 id 用 pipeline 执行 `GETSET view:<id> 0`，原子取出本周期增量并归零，拿到 `(id, delta)` 列表（delta=0 的跳过）
  3. 在一个 SQL 事务里批量 `UPDATE articles SET view_count = view_count + ? WHERE id = ?`
  4. 事务**成功** → `SREM view:dirty <id>...`（不再 DEL `view:<id>`，因为它已经在第 2 步被置为 0；新增的 INCR 应继续累加，无需清空）
  5. 事务**失败** → 对每个 id 执行 `INCRBY view:<id> delta` 把已取走的增量补回 Redis（防止丢失），保留 `view:dirty` 中的成员，等待下个周期重试
- 中途进程崩溃（步骤 2 之后、3 完成之前）：本周期已 GETSET 取走的增量会丢失。可接受范围，最多丢一个 30 秒聚合周期的计数。
- 优雅退出：`SIGTERM` 后立即触发一次 flush 再退出，把窗口内已累加的请求落库。

**详情读取的 view_count**：
```text
db_count = articles.view_count
redis_inc = GET view:<id>  // 可能为 nil
return db_count + redis_inc
```
保证用户看到的浏览数实时变化，没有「30 秒后才跳一次」的体感。

### 6.3 文章列表查询

- `WHERE status=1 AND deleted_at IS NULL [AND user_id=?]` + 复合索引 `(status, created_at)` 命中
- tag 过滤：先查 `article_tags WHERE tag_id=? ORDER BY article_id DESC LIMIT ?,?`，再 IN 查 articles
- summary：从 content 取前 200 字符，做最简单的 Markdown 标记剥离（`pkg/markdownx`），不用第三方解析器

### 6.4 上传

- 启动时确保 `uploads/` 存在，权限 0755
- 每次上传：
  1. `c.FormFile("file")` 拿 FileHeader
  2. 检查 size、Content-Type
  3. 通过文件头嗅探（前 512 字节 `http.DetectContentType`）二次校验，防伪造扩展名
  4. 生成 uuid 文件名，路径 `uploads/{yyyy}/{mm}/`
  5. `c.SaveUploadedFile`
  6. 返回相对 URL
- **孤儿清理**：每天 03:00 跑一次 cron-style goroutine，扫 `uploads/` 中超过 30 天且未在任何 articles.content 中出现的文件，移动到 `uploads/.trash/`（不直接删，便于 review）。本期可选实现，写入 PRD 但允许首版跳过。

### 6.5 限流

- 登录：`POST /auth/login` 每 IP 1 分钟 5 次。Redis key `rl:login:<ip>` INCR，第一次 EXPIRE 60s。
- 上传：每用户 1 分钟 10 次。
- 通用 IP 限流（兜底）：每 IP 1 分钟 300 次（防爬虫）。
- 超限返回 `2020`。

## 7. 错误码

| code | 含义 | HTTP Status |
|---|---|---|
| 0 | 成功 | 200 |
| 1001 | 参数无效 | 400 |
| 1010 | 用户名已存在 | 409 |
| 1020 | 文件类型不支持 | 400 |
| 1021 | 文件过大 | 413 |
| 1030 | 资源不存在 | 404 |
| 2001 | 未登录 | 401 |
| 2002 | 无权操作 | 403 |
| 2010 | 账号或密码错误 | 401 |
| 2020 | 操作频繁 | 429 |
| 2030 | CSRF 校验失败 | 403 |
| 5001 | 数据库异常 | 500 |
| 5002 | Redis 异常 | 500 |
| 5099 | 未知服务端错误 | 500 |

错误类型在 `internal/apperr` 定义：
```go
type AppErr struct { Code int; Msg string; HTTP int; Wrapped error }
```
service 抛 AppErr，handler 通过中间件统一渲染。

## 8. 并发与性能配置

| 项 | 值 |
|---|---|
| GORM `MaxOpenConns` | 50 |
| GORM `MaxIdleConns` | 10 |
| GORM `ConnMaxLifetime` | 30 min |
| Redis pool size | 20 |
| Redis dial/read/write timeout | 2s / 3s / 3s |
| HTTP 服务 ReadTimeout | 10s |
| HTTP 服务 WriteTimeout | 30s（含上传） |
| HTTP 服务 IdleTimeout | 60s |
| Gin recovery | 开启 |
| GORM logger 慢查询阈值 | 200 ms（warn） |

## 9. 测试策略

### 9.1 单元测试
- 目录：`internal/service/*_test.go`、`internal/repository/*_test.go`
- service 层注入 mock repository（用 testify/mock）
- 重点用例：
  - 注册：用户名重复、合法
  - 登录：错误密码、限流
  - 发布：参数无效、tag 自动 upsert
  - 计数 flush：增量取出 + DB 更新 + Set 清理
  - 权限：编辑非本人文章
- 目标：service 层 ≥ 60% 覆盖率

### 9.2 集成测试
- 目录：`Server/test/integration/`
- 工具：`testcontainers-go` 启 PostgreSQL + Redis
- 跑全部 API 主路径：注册→登录→发布→列表→详情(计数+1)→编辑→删除→退出
- CI 必跑，超时 5 分钟

### 9.3 压力测试（手动）
- 用 `vegeta` 或 `wrk`：对 `GET /articles/:id` 1000 并发 30 秒，断言：
  - p99 < 100 ms
  - 错误率 < 0.1%
  - DB CPU < 50%
  - 计数总和（DB+Redis）= 实际请求数 ± 1（30s flush 边界容差）

## 10. 失败容错

| 故障 | 应对 |
|---|---|
| Redis 不可用 | 计数累加直接跳过 + log warn；session 缺失视作未登录；限流退化为放过 |
| PostgreSQL 慢查询 | 200ms 报 warn；连接池满返回 5001 |
| PostgreSQL 不可用 | 5001 + 频繁告警；进程不主动退出，等运维介入 |
| 上传文件写入失败 | 返回 5099；已落盘半截文件由启动时清扫 |
| Panic | gin.Recovery 接住，日志含 stack，返回 5099 |
| 服务重启 | 优雅停机：context 取消 → 等正在执行请求结束 ≤ 30s → 触发一次计数 flush → 退出 |

## 11. 配置（config.yaml）

```yaml
server:
  addr: ":8080"
  static_dir: "../Web"
  upload_dir: "./uploads"
  csrf_cookie_secure: false  # prod 改 true

db:
  dsn: "host=127.0.0.1 port=5432 user=blog password=blog dbname=blog sslmode=disable TimeZone=Asia/Shanghai"

redis:
  addr: "127.0.0.1:6379"
  db: 0

session:
  cookie_name: "sid"
  ttl_minutes: 30          # 滑动过期：每次请求重置
  cookie_secret: "CHANGE_ME_32_BYTES_HEX"

upload:
  max_bytes: 5242880      # 5 MB
  allowed_ext: ["jpg","jpeg","png","gif","webp"]

ratelimit:
  login_per_min: 5
  upload_per_min: 10
  global_per_min: 300

view_flush:
  interval_seconds: 30
  batch_size: 1000

log:
  level: "info"
  file: "./logs/server.log"
```

环境变量优先级最高，例如 `DB_DSN` 覆盖 `db.dsn`。

## 12. Makefile 目标

```makefile
make run         # go run ./cmd/server
make test        # go test ./... -race -count=1
make integration # 跑集成测试（testcontainers）
make build       # go build -o bin/server ./cmd/server
make migrate-up  # 应用 migrations/*.up.sql
make lint        # golangci-lint run
```

## 13. 验证清单（后端开发者完成功能后逐条手动跑一遍）

- [ ] `make migrate-up && make run` 一次成功，端口 8080
- [ ] 注册同一用户名第二次返回 1010
- [ ] 错误密码登录返回 2010
- [ ] 一分钟内 6 次错误密码登录返回 2020
- [ ] 未登录调用写接口返回 2001
- [ ] 用户 A 编辑用户 B 的文章 PUT 返回 2002
- [ ] 上传 6 MB 图片返回 1021
- [ ] 上传 .exe 改后缀为 .png 被嗅探拦截，返回 1020
- [ ] 1000 次 GET 同一文章详情后，等待 35 秒（一个 flush 周期 + 余量），DB `view_count` 增加 1000，Redis 中 `view:<id>` 值为 0（或被新请求继续累加），`view:dirty` 中已无该 id（除非在等待期间又有新请求）
- [ ] 杀掉 Redis 后 detail 仍能正常返回（仅日志告警，不计数）
- [ ] `kill -TERM` 后服务在 30s 内退出，且本次计数已 flush
- [ ] 登录成功后，关闭浏览器再开（5 分钟内）→ 直接进入列表页，未跳登录
- [ ] 登录后静置 31 分钟 → 任何鉴权请求返回 2001，前端跳登录页
- [ ] 登录后每隔 25 分钟做一次操作，2 小时后仍处于登录态（验证滑动续期）
- [ ] 调用 logout → 服务返回成功，浏览器 Cookie 被清除，再访问需登录页面跳转登录
- [ ] sysadmin / admin111 登录 → 返回 `isAdmin:true`，前端跳 `/admin.html`
- [ ] sysadmin 调用 GET /api/v1/admin/users → 返回全部注册用户列表
- [ ] sysadmin 调用 DELETE /api/v1/admin/users/:id → 级联硬删除用户及其文章，再次查询用户不出现
- [ ] 普通用户调用 GET /api/v1/admin/users → 返回 `{"code":2002,"msg":"无权访问"}`
- [ ] 普通用户直接访问 `/admin.html` → admin.js 检测非 admin 后跳 `/list.html`

---

文档结束。前端契约见 `WebPRD.md`。
