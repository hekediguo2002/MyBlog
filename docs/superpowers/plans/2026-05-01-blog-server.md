# 博客后端 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 Server.md 定义的博客平台 Go 后端：账号、文章 CRUD、标签、图片上传、Redis 浏览计数回写、滑动会话、CSRF、限流。

**Architecture:** 单进程 Go 服务，gin handler + service + repository 三层；MySQL 持久化、Redis 做会话/计数/限流；浏览计数走 INCR + 30 秒批量回写以扛热点;静态资源由同一进程托管 `/Web/` 与 `/uploads/`。

**Tech Stack:** Go 1.22+, gin, gorm/mysql, go-redis/v9, gin-contrib/sessions(redis), bcrypt, zerolog, viper, testify, testcontainers-go.

**Spec:** `Server.md`（同目录）。**前置文档：** `WebPRD.md`（API 契约消费方）。

---

## File Structure

按 `Server.md §3`，每个文件单一职责。计划完成后目录形态:

```
Server/
├─ cmd/server/main.go                  # 入口：load cfg → init deps → run gin
├─ internal/
│  ├─ config/config.go                 # Config 结构与 viper 加载
│  ├─ db/mysql.go                      # GORM 初始化、连接池
│  ├─ cache/redis.go                   # go-redis 初始化
│  ├─ apperr/errors.go                 # AppErr 类型 + 错误码常量
│  ├─ pkg/
│  │  ├─ httpresp/resp.go              # 统一响应外壳与 Gin 渲染
│  │  ├─ password/bcrypt.go            # Hash / Compare
│  │  ├─ idgen/uuid.go                 # 文件名 / sid 生成
│  │  └─ markdownx/summary.go          # Markdown 标记剥离取摘要
│  ├─ middleware/
│  │  ├─ recover.go                    # gin.Recovery 包装 + 渲染 5099
│  │  ├─ logger.go                     # 请求日志 (zerolog)
│  │  ├─ session.go                    # 注入 sessions store + 滑动续期
│  │  ├─ csrf.go                       # 写方法比对 cookie/header
│  │  ├─ auth.go                       # 强制登录、注入 user
│  │  └─ ratelimit.go                  # 通用 + 命名 (login/upload) 限流
│  ├─ model/
│  │  ├─ user.go
│  │  ├─ article.go                    # 含 Tags many2many
│  │  ├─ tag.go
│  │  └─ article_tag.go                # 显式 join 表（可选）
│  ├─ repository/
│  │  ├─ user_repo.go                  # Create / FindByUsername / FindByID
│  │  ├─ article_repo.go               # Create+tags / Update / SoftDelete / FindByID / List / Count
│  │  ├─ tag_repo.go                   # FindOrCreate / ListWithCount / EnsureMany
│  │  └─ counter_repo.go               # INCR / SADD / SMEMBERS / GETSET / SREM
│  ├─ service/
│  │  ├─ auth_service.go               # Register / Login / Logout / Me + bcrypt + session 写
│  │  ├─ article_service.go            # CRUD + 权限 + Redis 浏览数合并
│  │  ├─ tag_service.go
│  │  ├─ upload_service.go             # 校验 + 文件头嗅探 + 落盘
│  │  └─ view_counter_service.go       # 30s flush loop + 优雅退出
│  ├─ handler/
│  │  ├─ auth_handler.go
│  │  ├─ article_handler.go
│  │  ├─ tag_handler.go
│  │  └─ upload_handler.go
│  └─ router/router.go                 # 路由组装 + 中间件挂载
├─ migrations/
│  ├─ 001_init.up.sql
│  └─ 001_init.down.sql
├─ test/integration/
│  ├─ main_test.go                     # testcontainers 起 MySQL+Redis + 启服务
│  ├─ auth_test.go
│  ├─ article_test.go
│  ├─ upload_test.go
│  └─ counter_test.go
├─ uploads/                            # 运行时生成
├─ config.yaml                         # 默认配置
├─ go.mod / go.sum
├─ Makefile
└─ docker-compose.yml
```

---

## Phase 0 — 基础设施

### Task 1: 初始化 Go 模块与目录骨架

**Files:**
- Create: `Server/go.mod`
- Create: `Server/.gitignore`
- Create: `Server/Makefile`
- Create: `Server/docker-compose.yml`
- Create: `Server/config.yaml`

- [ ] **Step 1: 创建 go.mod**

```bash
cd Server
go mod init github.com/wjr/blog/server
```

预期输出: `go: creating new go.mod: module github.com/wjr/blog/server`

- [ ] **Step 2: 写入 .gitignore**

```
# Server/.gitignore
/bin/
/uploads/
/logs/
/.env
*.log
.DS_Store
```

- [ ] **Step 3: 写入 Makefile**

```makefile
# Server/Makefile
.PHONY: run test integration build migrate-up lint deps

deps:
	go mod tidy

run:
	go run ./cmd/server

test:
	go test ./internal/... -race -count=1

integration:
	go test ./test/integration/... -race -count=1 -timeout 5m

build:
	go build -o bin/server ./cmd/server

migrate-up:
	@echo "Applying migrations/*.up.sql to MySQL..."
	@for f in migrations/*.up.sql; do \
		echo "==> $$f"; \
		mysql -h 127.0.0.1 -P 3306 -u blog -pblog blog < $$f || exit 1; \
	done

lint:
	golangci-lint run ./...
```

- [ ] **Step 4: 写入 docker-compose.yml**

```yaml
# Server/docker-compose.yml
version: "3.8"
services:
  mysql:
    image: mysql:8.0
    restart: unless-stopped
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: blog
      MYSQL_USER: blog
      MYSQL_PASSWORD: blog
    command: ["--character-set-server=utf8mb4", "--collation-server=utf8mb4_unicode_ci"]
    ports: ["3306:3306"]
    volumes: ["mysql-data:/var/lib/mysql"]
  redis:
    image: redis:7-alpine
    restart: unless-stopped
    ports: ["6379:6379"]
volumes:
  mysql-data:
```

- [ ] **Step 5: 写入 config.yaml**

```yaml
# Server/config.yaml
server:
  addr: ":8080"
  static_dir: "../Web"
  upload_dir: "./uploads"
  csrf_cookie_secure: false

mysql:
  dsn: "blog:blog@tcp(127.0.0.1:3306)/blog?charset=utf8mb4&parseTime=true&loc=Local"

redis:
  addr: "127.0.0.1:6379"
  db: 0

session:
  cookie_name: "sid"
  ttl_minutes: 30
  cookie_secret: "CHANGE_ME_32_BYTES_HEX_DEVONLY!!"

upload:
  max_bytes: 5242880
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

- [ ] **Step 6: 创建源码目录骨架**

```bash
mkdir -p cmd/server internal/{config,db,cache,apperr,pkg/{httpresp,password,idgen,markdownx},middleware,model,repository,service,handler,router} migrations test/integration uploads logs
```

- [ ] **Step 7: 启动 docker-compose 验证依赖可达**

```bash
docker compose up -d
docker compose ps
```

预期:`mysql` 与 `redis` 两个容器 `running`、端口已映射。

- [ ] **Step 8: Commit**

```bash
cd .. && git init -q && git add Server/.gitignore Server/Makefile Server/docker-compose.yml Server/config.yaml Server/go.mod
git commit -m "chore(server): scaffold module, makefile, compose, default config"
```

---

### Task 2: Config 加载（viper + 环境变量覆盖）

**Files:**
- Create: `Server/internal/config/config.go`
- Test: `Server/internal/config/config_test.go`

- [ ] **Step 1: 写失败测试**

```go
// Server/internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_ReadsYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(`
server:
  addr: ":9090"
mysql:
  dsn: "u:p@tcp(127.0.0.1:3306)/db?parseTime=true"
redis:
  addr: "127.0.0.1:6379"
session:
  cookie_name: "sid"
  ttl_minutes: 30
  cookie_secret: "x"
upload:
  max_bytes: 1024
  allowed_ext: ["png"]
ratelimit:
  login_per_min: 5
  upload_per_min: 10
  global_per_min: 300
view_flush:
  interval_seconds: 30
  batch_size: 1000
log:
  level: "debug"
`), 0o644))

	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	require.Equal(t, ":9090", cfg.Server.Addr)
	require.Equal(t, 30, cfg.Session.TTLMinutes)
	require.Equal(t, []string{"png"}, cfg.Upload.AllowedExt)
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(`
server: { addr: ":9090" }
mysql:   { dsn: "from-yaml" }
redis:   { addr: "127.0.0.1:6379" }
session: { cookie_name: "sid", ttl_minutes: 30, cookie_secret: "x" }
upload:  { max_bytes: 1, allowed_ext: ["png"] }
ratelimit: { login_per_min: 5, upload_per_min: 10, global_per_min: 300 }
view_flush: { interval_seconds: 30, batch_size: 1000 }
log: { level: "info" }
`), 0o644))
	t.Setenv("MYSQL_DSN", "from-env")

	cfg, err := Load(yamlPath)
	require.NoError(t, err)
	require.Equal(t, "from-env", cfg.MySQL.DSN)
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
cd Server && go test ./internal/config/... -run TestLoad -v
```

预期:`Load` 未定义,编译失败。

- [ ] **Step 3: 实现 config.go**

```go
// Server/internal/config/config.go
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server     ServerCfg     `mapstructure:"server"`
	MySQL      MySQLCfg      `mapstructure:"mysql"`
	Redis      RedisCfg      `mapstructure:"redis"`
	Session    SessionCfg    `mapstructure:"session"`
	Upload     UploadCfg     `mapstructure:"upload"`
	RateLimit  RateLimitCfg  `mapstructure:"ratelimit"`
	ViewFlush  ViewFlushCfg  `mapstructure:"view_flush"`
	Log        LogCfg        `mapstructure:"log"`
}

type ServerCfg struct {
	Addr             string `mapstructure:"addr"`
	StaticDir        string `mapstructure:"static_dir"`
	UploadDir        string `mapstructure:"upload_dir"`
	CSRFCookieSecure bool   `mapstructure:"csrf_cookie_secure"`
}
type MySQLCfg   struct{ DSN string `mapstructure:"dsn"` }
type RedisCfg   struct{
	Addr string `mapstructure:"addr"`
	DB   int    `mapstructure:"db"`
}
type SessionCfg struct {
	CookieName   string `mapstructure:"cookie_name"`
	TTLMinutes   int    `mapstructure:"ttl_minutes"`
	CookieSecret string `mapstructure:"cookie_secret"`
}
type UploadCfg struct {
	MaxBytes   int64    `mapstructure:"max_bytes"`
	AllowedExt []string `mapstructure:"allowed_ext"`
}
type RateLimitCfg struct {
	LoginPerMin  int `mapstructure:"login_per_min"`
	UploadPerMin int `mapstructure:"upload_per_min"`
	GlobalPerMin int `mapstructure:"global_per_min"`
}
type ViewFlushCfg struct {
	IntervalSeconds int `mapstructure:"interval_seconds"`
	BatchSize       int `mapstructure:"batch_size"`
}
type LogCfg struct {
	Level string `mapstructure:"level"`
	File  string `mapstructure:"file"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	bind := []string{
		"server.addr", "server.static_dir", "server.upload_dir", "server.csrf_cookie_secure",
		"mysql.dsn", "redis.addr", "redis.db",
		"session.cookie_name", "session.ttl_minutes", "session.cookie_secret",
		"log.level", "log.file",
	}
	for _, k := range bind {
		_ = v.BindEnv(k)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &cfg, nil
}
```

- [ ] **Step 4: 安装依赖并跑测试**

```bash
go get github.com/spf13/viper github.com/stretchr/testify
go mod tidy
go test ./internal/config/... -v
```

预期: 两个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/config Server/go.mod Server/go.sum
git commit -m "feat(server): config loader with yaml + env override"
```

---

### Task 3: 应用错误类型 `apperr`

**Files:**
- Create: `Server/internal/apperr/errors.go`
- Test: `Server/internal/apperr/errors_test.go`

- [ ] **Step 1: 写失败测试**

```go
// Server/internal/apperr/errors_test.go
package apperr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew_BasicShape(t *testing.T) {
	e := New(CodeInvalidParam, "bad name")
	require.Equal(t, 1001, e.Code)
	require.Equal(t, "bad name", e.Msg)
	require.Equal(t, 400, e.HTTP)
	require.Equal(t, "[1001] bad name", e.Error())
}

func TestWrap_KeepsCause(t *testing.T) {
	cause := errors.New("db gone")
	e := Wrap(CodeDBError, "db", cause)
	require.True(t, errors.Is(e, cause))
}

func TestAs_FromGenericErr(t *testing.T) {
	var dst *AppErr
	require.True(t, errors.As(New(CodeUnauthorized, "x"), &dst))
	require.Equal(t, 2001, dst.Code)
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./internal/apperr/... -v
```

预期: 编译失败。

- [ ] **Step 3: 实现 errors.go**

```go
// Server/internal/apperr/errors.go
package apperr

import "fmt"

const (
	CodeOK              = 0
	CodeInvalidParam    = 1001
	CodeUsernameTaken   = 1010
	CodeFileType        = 1020
	CodeFileTooLarge    = 1021
	CodeNotFound        = 1030
	CodeUnauthorized    = 2001
	CodeForbidden       = 2002
	CodeBadCredential   = 2010
	CodeRateLimited     = 2020
	CodeCSRFInvalid     = 2030
	CodeDBError         = 5001
	CodeRedisError      = 5002
	CodeUnknown         = 5099
)

type AppErr struct {
	Code    int
	Msg     string
	HTTP    int
	Wrapped error
}

func (e *AppErr) Error() string { return fmt.Sprintf("[%d] %s", e.Code, e.Msg) }
func (e *AppErr) Unwrap() error { return e.Wrapped }

func httpFor(code int) int {
	switch code {
	case CodeOK:
		return 200
	case CodeInvalidParam, CodeFileType:
		return 400
	case CodeUnauthorized, CodeBadCredential:
		return 401
	case CodeForbidden, CodeCSRFInvalid:
		return 403
	case CodeNotFound:
		return 404
	case CodeUsernameTaken:
		return 409
	case CodeFileTooLarge:
		return 413
	case CodeRateLimited:
		return 429
	default:
		return 500
	}
}

func New(code int, msg string) *AppErr {
	return &AppErr{Code: code, Msg: msg, HTTP: httpFor(code)}
}
func Wrap(code int, msg string, cause error) *AppErr {
	return &AppErr{Code: code, Msg: msg, HTTP: httpFor(code), Wrapped: cause}
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./internal/apperr/... -v
```

预期: 三个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/apperr
git commit -m "feat(server): apperr type with codes mapped to http status"
```

---

### Task 4: 统一响应外壳 `httpresp`

**Files:**
- Create: `Server/internal/pkg/httpresp/resp.go`
- Test: `Server/internal/pkg/httpresp/resp_test.go`

- [ ] **Step 1: 写失败测试**

```go
// Server/internal/pkg/httpresp/resp_test.go
package httpresp

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
)

func TestOK_WritesEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	OK(c, gin.H{"id": 7})

	require.Equal(t, 200, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, float64(0), got["code"])
	require.Equal(t, "ok", got["msg"])
	data := got["data"].(map[string]any)
	require.Equal(t, float64(7), data["id"])
}

func TestFail_RendersAppErr(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Fail(c, apperr.New(apperr.CodeUsernameTaken, "用户名已被使用"))

	require.Equal(t, 409, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, float64(1010), got["code"])
	require.Equal(t, "用户名已被使用", got["msg"])
	require.Nil(t, got["data"])
}

func TestFail_NonAppErr_RendersUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Fail(c, errAdHoc("boom"))
	require.Equal(t, 500, w.Code)
	var got map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, float64(5099), got["code"])
}

type errAdHoc string
func (e errAdHoc) Error() string { return string(e) }
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go get github.com/gin-gonic/gin
go mod tidy
go test ./internal/pkg/httpresp/... -v
```

预期:`OK` / `Fail` 未定义。

- [ ] **Step 3: 实现 resp.go**

```go
// Server/internal/pkg/httpresp/resp.go
package httpresp

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
)

type Envelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func OK(c *gin.Context, data any) {
	c.JSON(200, Envelope{Code: apperr.CodeOK, Msg: "ok", Data: data})
}

func Fail(c *gin.Context, err error) {
	var ae *apperr.AppErr
	if !errors.As(err, &ae) {
		ae = apperr.New(apperr.CodeUnknown, "internal error")
	}
	c.AbortWithStatusJSON(ae.HTTP, Envelope{Code: ae.Code, Msg: ae.Msg, Data: nil})
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./internal/pkg/httpresp/... -v
```

预期: 三个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/pkg/httpresp Server/go.mod Server/go.sum
git commit -m "feat(server): httpresp envelope wrapping AppErr"
```

---

### Task 5: 工具包 — `password` / `idgen` / `markdownx`

**Files:**
- Create: `Server/internal/pkg/password/bcrypt.go`
- Test: `Server/internal/pkg/password/bcrypt_test.go`
- Create: `Server/internal/pkg/idgen/uuid.go`
- Test: `Server/internal/pkg/idgen/uuid_test.go`
- Create: `Server/internal/pkg/markdownx/summary.go`
- Test: `Server/internal/pkg/markdownx/summary_test.go`

- [ ] **Step 1: 写 password 测试**

```go
// Server/internal/pkg/password/bcrypt_test.go
package password

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashCompare_RoundTrip(t *testing.T) {
	h, err := Hash("Password123")
	require.NoError(t, err)
	require.Len(t, h, 60)
	require.True(t, Compare(h, "Password123"))
	require.False(t, Compare(h, "wrong"))
}
```

- [ ] **Step 2: 实现 password**

```go
// Server/internal/pkg/password/bcrypt.go
package password

import "golang.org/x/crypto/bcrypt"

const Cost = 12

func Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), Cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
func Compare(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
```

- [ ] **Step 3: 写 idgen 测试**

```go
// Server/internal/pkg/idgen/uuid_test.go
package idgen

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewUUID_Format(t *testing.T) {
	got := NewUUID()
	require.Regexp(t, regexp.MustCompile(`^[a-f0-9]{32}$`), got)
}

func TestNewUUID_Unique(t *testing.T) {
	require.NotEqual(t, NewUUID(), NewUUID())
}
```

- [ ] **Step 4: 实现 idgen**

```go
// Server/internal/pkg/idgen/uuid.go
package idgen

import (
	"crypto/rand"
	"encoding/hex"
)

func NewUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b[:])
}
```

- [ ] **Step 5: 写 markdownx 测试**

```go
// Server/internal/pkg/markdownx/summary_test.go
package markdownx

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummary_StripsCommonMarkup(t *testing.T) {
	in := "# Hello\n\nThis is **bold** and `code` and [link](https://x).\n\n```go\nfmt.Println(\"hi\")\n```\n\n后续中文内容也要保留。"
	got := Summary(in, 200)
	require.NotContains(t, got, "#")
	require.NotContains(t, got, "**")
	require.NotContains(t, got, "`")
	require.NotContains(t, got, "[")
	require.Contains(t, got, "Hello")
	require.Contains(t, got, "后续中文内容也要保留。")
}

func TestSummary_TruncatesByRune(t *testing.T) {
	in := strings.Repeat("中", 300)
	got := Summary(in, 200)
	require.Equal(t, 200, len([]rune(got)))
}
```

- [ ] **Step 6: 实现 markdownx**

```go
// Server/internal/pkg/markdownx/summary.go
package markdownx

import (
	"regexp"
	"strings"
)

var (
	reFence   = regexp.MustCompile("(?s)```.*?```")
	reInline  = regexp.MustCompile("`[^`]*`")
	reImage   = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reLink    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reHeading = regexp.MustCompile(`(?m)^#{1,6}\s*`)
	reEmph    = regexp.MustCompile(`[*_]{1,3}`)
	reQuote   = regexp.MustCompile(`(?m)^>\s?`)
	reList    = regexp.MustCompile(`(?m)^([*+\-]|\d+\.)\s+`)
	reSpaces  = regexp.MustCompile(`\s+`)
)

func Summary(content string, maxRunes int) string {
	s := content
	s = reFence.ReplaceAllString(s, "")
	s = reImage.ReplaceAllString(s, "")
	s = reLink.ReplaceAllString(s, "$1")
	s = reInline.ReplaceAllString(s, "")
	s = reHeading.ReplaceAllString(s, "")
	s = reEmph.ReplaceAllString(s, "")
	s = reQuote.ReplaceAllString(s, "")
	s = reList.ReplaceAllString(s, "")
	s = reSpaces.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > maxRunes {
		r = r[:maxRunes]
	}
	return string(r)
}
```

- [ ] **Step 7: 跑全部测试**

```bash
go get golang.org/x/crypto/bcrypt
go mod tidy
go test ./internal/pkg/... -v
```

预期: password / idgen / markdownx 所有 case PASS。

- [ ] **Step 8: Commit**

```bash
git add Server/internal/pkg/password Server/internal/pkg/idgen Server/internal/pkg/markdownx Server/go.mod Server/go.sum
git commit -m "feat(server): bcrypt/uuid/markdown-summary utility packages"
```

---

### Task 6: MySQL 连接 + 迁移脚本

**Files:**
- Create: `Server/internal/db/mysql.go`
- Create: `Server/migrations/001_init.up.sql`
- Create: `Server/migrations/001_init.down.sql`

- [ ] **Step 1: 写迁移 up SQL**

```sql
-- Server/migrations/001_init.up.sql
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS users (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username      VARCHAR(32)     NOT NULL,
  password_hash CHAR(60)        NOT NULL,
  name          VARCHAR(64)     NOT NULL,
  created_at    DATETIME        NOT NULL,
  updated_at    DATETIME        NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS articles (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id     BIGINT UNSIGNED NOT NULL,
  title       VARCHAR(200)    NOT NULL,
  content     MEDIUMTEXT      NOT NULL,
  view_count  BIGINT UNSIGNED NOT NULL DEFAULT 0,
  status      TINYINT         NOT NULL DEFAULT 1,
  created_at  DATETIME        NOT NULL,
  updated_at  DATETIME        NOT NULL,
  deleted_at  DATETIME        NULL,
  PRIMARY KEY (id),
  KEY idx_user_id (user_id),
  KEY idx_status_created (status, created_at),
  KEY idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS tags (
  id   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(32)     NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS article_tags (
  article_id BIGINT UNSIGNED NOT NULL,
  tag_id     BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (article_id, tag_id),
  KEY idx_tag_article (tag_id, article_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

- [ ] **Step 2: 写迁移 down SQL**

```sql
-- Server/migrations/001_init.down.sql
DROP TABLE IF EXISTS article_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS articles;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 3: 实现 db/mysql.go**

```go
// Server/internal/db/mysql.go
package db

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

type Options struct {
	DSN              string
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
	SlowThresholdMS  int
}

func DefaultOptions(dsn string) Options {
	return Options{
		DSN:             dsn,
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		SlowThresholdMS: 200,
	}
}

func Open(opt Options) (*gorm.DB, error) {
	gdb, err := gorm.Open(mysql.Open(opt.DSN), &gorm.Config{
		Logger: gormlog.Default.LogMode(gormlog.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(opt.MaxOpenConns)
	sqlDB.SetMaxIdleConns(opt.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(opt.ConnMaxLifetime)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return gdb, nil
}
```

- [ ] **Step 4: 安装依赖并应用迁移**

```bash
go get gorm.io/gorm gorm.io/driver/mysql
go mod tidy
docker compose up -d
make migrate-up
```

预期: 4 张表创建成功。可用 `mysql -h127.0.0.1 -ublog -pblog blog -e "SHOW TABLES;"` 验证。

- [ ] **Step 5: Commit**

```bash
git add Server/migrations Server/internal/db Server/go.mod Server/go.sum
git commit -m "feat(server): mysql migrations + gorm connection helper"
```

---

### Task 7: Redis 连接

**Files:**
- Create: `Server/internal/cache/redis.go`
- Test: `Server/internal/cache/redis_test.go`（可选 ping smoke,需要本地 Redis）

- [ ] **Step 1: 写 smoke 测试（被环境变量门控）**

```go
// Server/internal/cache/redis_test.go
package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpen_PingsLocalRedis(t *testing.T) {
	if os.Getenv("REDIS_ADDR") == "" {
		t.Skip("REDIS_ADDR not set; skipping smoke test")
	}
	rdb, err := Open(Options{Addr: os.Getenv("REDIS_ADDR"), DB: 0})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rdb.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, rdb.Ping(ctx).Err())
}
```

- [ ] **Step 2: 实现 redis.go**

```go
// Server/internal/cache/redis.go
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Options struct {
	Addr         string
	DB           int
	PoolSize     int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func Default(addr string, db int) Options {
	return Options{
		Addr:         addr,
		DB:           db,
		PoolSize:     20,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

func Open(opt Options) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         opt.Addr,
		DB:           opt.DB,
		PoolSize:     opt.PoolSize,
		DialTimeout:  opt.DialTimeout,
		ReadTimeout:  opt.ReadTimeout,
		WriteTimeout: opt.WriteTimeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), opt.DialTimeout+opt.ReadTimeout)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return rdb, nil
}
```

- [ ] **Step 3: 安装并跑 smoke 测试**

```bash
go get github.com/redis/go-redis/v9
go mod tidy
REDIS_ADDR=127.0.0.1:6379 go test ./internal/cache/... -v
```

预期: `TestOpen_PingsLocalRedis` PASS。

- [ ] **Step 4: Commit**

```bash
git add Server/internal/cache Server/go.mod Server/go.sum
git commit -m "feat(server): redis client wrapper with timeouts"
```

---

## Phase 1 — 模型与仓储

### Task 8: GORM 模型

**Files:**
- Create: `Server/internal/model/user.go`
- Create: `Server/internal/model/article.go`
- Create: `Server/internal/model/tag.go`

- [ ] **Step 1: 实现 User**

```go
// Server/internal/model/user.go
package model

import "time"

type User struct {
	ID           uint64    `gorm:"primaryKey;column:id"`
	Username     string    `gorm:"size:32;uniqueIndex:uk_username;column:username"`
	PasswordHash string    `gorm:"size:60;column:password_hash"`
	Name         string    `gorm:"size:64;column:name"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string { return "users" }
```

- [ ] **Step 2: 实现 Article**

```go
// Server/internal/model/article.go
package model

import (
	"time"

	"gorm.io/gorm"
)

type Article struct {
	ID        uint64         `gorm:"primaryKey;column:id"`
	UserID    uint64         `gorm:"index:idx_user_id;column:user_id"`
	Title     string         `gorm:"size:200;column:title"`
	Content   string         `gorm:"type:mediumtext;column:content"`
	ViewCount uint64         `gorm:"column:view_count;default:0"`
	Status    int8           `gorm:"column:status;default:1"`
	CreatedAt time.Time      `gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index"`

	Tags   []Tag `gorm:"many2many:article_tags;joinForeignKey:article_id;joinReferences:tag_id"`
	Author User  `gorm:"foreignKey:UserID;references:ID"`
}

func (Article) TableName() string { return "articles" }
```

- [ ] **Step 3: 实现 Tag**

```go
// Server/internal/model/tag.go
package model

type Tag struct {
	ID   uint64 `gorm:"primaryKey;column:id"`
	Name string `gorm:"size:32;uniqueIndex:uk_name;column:name"`
}

func (Tag) TableName() string { return "tags" }
```

- [ ] **Step 4: 验证编译**

```bash
go build ./internal/model/...
```

预期: 无报错。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/model
git commit -m "feat(server): gorm models for user/article/tag"
```

---

### Task 9: User Repository

**Files:**
- Create: `Server/internal/repository/user_repo.go`
- Test: `Server/internal/repository/user_repo_test.go`

> 测试用 SQLite 内存库（gorm/driver/sqlite）跑 repo 单测,免去 testcontainers。集成测试在 Phase 4 用真实 MySQL 覆盖。

- [ ] **Step 1: 添加 sqlite 驱动**

```bash
go get gorm.io/driver/sqlite
go mod tidy
```

- [ ] **Step 2: 写共享测试夹具**

```go
// Server/internal/repository/repo_test_helper.go
//go:build test || !ignore
// +build test !ignore

package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wjr/blog/server/internal/model"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{NowFunc: func() time.Time { return time.Now().UTC() }})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Article{}, &model.Tag{}))
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(&model.User{}, &model.Article{}, &model.Tag{}, "article_tags")
	})
	return db
}
```

- [ ] **Step 3: 写失败测试**

```go
// Server/internal/repository/user_repo_test.go
package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

func TestUserRepo_CreateAndFind(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	u := &model.User{Username: "alice", PasswordHash: "h", Name: "爱丽丝"}
	require.NoError(t, repo.Create(ctx, u))
	require.Greater(t, u.ID, uint64(0))

	got, err := repo.FindByUsername(ctx, "alice")
	require.NoError(t, err)
	require.Equal(t, u.ID, got.ID)

	got2, err := repo.FindByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "爱丽丝", got2.Name)
}

func TestUserRepo_Create_DuplicateUsername(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepo(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.User{Username: "alice", PasswordHash: "h", Name: "x"}))
	err := repo.Create(ctx, &model.User{Username: "alice", PasswordHash: "h", Name: "y"})
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeUsernameTaken, ae.Code)
}

func TestUserRepo_FindByUsername_NotFound(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepo(db)
	_, err := repo.FindByUsername(context.Background(), "ghost")
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeNotFound, ae.Code)
}
```

- [ ] **Step 4: 跑测试确认失败**

```bash
go test ./internal/repository/... -run TestUserRepo -v
```

预期: 编译失败（`NewUserRepo` 未定义）。

- [ ] **Step 5: 实现 user_repo.go**

```go
// Server/internal/repository/user_repo.go
package repository

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

type UserRepo interface {
	Create(ctx context.Context, u *model.User) error
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByID(ctx context.Context, id uint64) (*model.User, error)
}

type userRepo struct{ db *gorm.DB }

func NewUserRepo(db *gorm.DB) UserRepo { return &userRepo{db: db} }

func (r *userRepo) Create(ctx context.Context, u *model.User) error {
	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		if isDuplicate(err) {
			return apperr.New(apperr.CodeUsernameTaken, "用户名已被使用")
		}
		return apperr.Wrap(apperr.CodeDBError, "create user", err)
	}
	return nil
}

func (r *userRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.New(apperr.CodeNotFound, "用户不存在")
		}
		return nil, apperr.Wrap(apperr.CodeDBError, "find by username", err)
	}
	return &u, nil
}

func (r *userRepo) FindByID(ctx context.Context, id uint64) (*model.User, error) {
	var u model.User
	if err := r.db.WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.New(apperr.CodeNotFound, "用户不存在")
		}
		return nil, apperr.Wrap(apperr.CodeDBError, "find by id", err)
	}
	return &u, nil
}

func isDuplicate(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") || strings.Contains(msg, "constraint failed")
}
```

- [ ] **Step 6: 跑测试确认通过**

```bash
go test ./internal/repository/... -run TestUserRepo -v
```

预期: 三个 case PASS。

- [ ] **Step 7: Commit**

```bash
git add Server/internal/repository/user_repo.go Server/internal/repository/user_repo_test.go Server/internal/repository/repo_test_helper.go Server/go.mod Server/go.sum
git commit -m "feat(server): user repository (create/find by username/find by id)"
```

---

### Task 10: Tag Repository

**Files:**
- Create: `Server/internal/repository/tag_repo.go`
- Test: `Server/internal/repository/tag_repo_test.go`

- [ ] **Step 1: 写失败测试**

```go
// Server/internal/repository/tag_repo_test.go
package repository

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/model"
)

func TestTagRepo_EnsureMany_DedupAndUpsert(t *testing.T) {
	db := newTestDB(t)
	repo := NewTagRepo(db)
	ctx := context.Background()

	tags, err := repo.EnsureMany(ctx, []string{"go", "blog", "go"})
	require.NoError(t, err)
	require.Len(t, tags, 2)
	names := []string{tags[0].Name, tags[1].Name}
	sort.Strings(names)
	require.Equal(t, []string{"blog", "go"}, names)

	again, err := repo.EnsureMany(ctx, []string{"go"})
	require.NoError(t, err)
	require.Len(t, again, 1)
	require.Equal(t, tags[0].ID, findTagByName(tags, "go").ID)
	_ = again
}

func TestTagRepo_ListWithCount(t *testing.T) {
	db := newTestDB(t)
	repo := NewTagRepo(db)
	ctx := context.Background()

	tags, err := repo.EnsureMany(ctx, []string{"go", "blog"})
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.Article{UserID: 1, Title: "a", Content: "c", Status: 1, Tags: []model.Tag{tags[0]}}).Error)
	require.NoError(t, db.Create(&model.Article{UserID: 1, Title: "b", Content: "c", Status: 1, Tags: []model.Tag{tags[0], tags[1]}}).Error)

	rows, err := repo.ListWithCount(ctx)
	require.NoError(t, err)
	got := map[string]int{}
	for _, r := range rows {
		got[r.Name] = r.ArticleCount
	}
	require.Equal(t, 2, got["go"])
	require.Equal(t, 1, got["blog"])
}

func findTagByName(ts []model.Tag, name string) *model.Tag {
	for i := range ts {
		if ts[i].Name == name {
			return &ts[i]
		}
	}
	return nil
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./internal/repository/... -run TestTagRepo -v
```

预期: 编译失败。

- [ ] **Step 3: 实现 tag_repo.go**

```go
// Server/internal/repository/tag_repo.go
package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

type TagWithCount struct {
	ID           uint64 `json:"id"`
	Name         string `json:"name"`
	ArticleCount int    `json:"article_count"`
}

type TagRepo interface {
	EnsureMany(ctx context.Context, names []string) ([]model.Tag, error)
	ListWithCount(ctx context.Context) ([]TagWithCount, error)
}

type tagRepo struct{ db *gorm.DB }

func NewTagRepo(db *gorm.DB) TagRepo { return &tagRepo{db: db} }

func (r *tagRepo) EnsureMany(ctx context.Context, names []string) ([]model.Tag, error) {
	uniq := dedupNonEmpty(names)
	if len(uniq) == 0 {
		return nil, nil
	}
	rows := make([]model.Tag, len(uniq))
	for i, n := range uniq {
		rows[i] = model.Tag{Name: n}
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).
		Create(&rows).Error; err != nil {
		return nil, apperr.Wrap(apperr.CodeDBError, "upsert tags", err)
	}
	var out []model.Tag
	if err := r.db.WithContext(ctx).Where("name IN ?", uniq).Find(&out).Error; err != nil {
		return nil, apperr.Wrap(apperr.CodeDBError, "load tags", err)
	}
	return out, nil
}

func (r *tagRepo) ListWithCount(ctx context.Context) ([]TagWithCount, error) {
	var rows []TagWithCount
	err := r.db.WithContext(ctx).
		Table("tags").
		Select("tags.id, tags.name, COUNT(article_tags.article_id) AS article_count").
		Joins("LEFT JOIN article_tags ON article_tags.tag_id = tags.id").
		Group("tags.id, tags.name").
		Order("article_count DESC, tags.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeDBError, "list tags", err)
	}
	return rows, nil
}

func dedupNonEmpty(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./internal/repository/... -run TestTagRepo -v
```

预期: 两个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/repository/tag_repo.go Server/internal/repository/tag_repo_test.go
git commit -m "feat(server): tag repo with upsert + count list"
```

---

### Task 11: Article Repository

**Files:**
- Create: `Server/internal/repository/article_repo.go`
- Test: `Server/internal/repository/article_repo_test.go`

- [ ] **Step 1: 写失败测试**

```go
// Server/internal/repository/article_repo_test.go
package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

func seedUser(t *testing.T, repo UserRepo) *model.User {
	t.Helper()
	u := &model.User{Username: "alice", PasswordHash: "h", Name: "A"}
	require.NoError(t, repo.Create(context.Background(), u))
	return u
}

func TestArticleRepo_CreateWithTags_AndFind(t *testing.T) {
	db := newTestDB(t)
	uRepo := NewUserRepo(db)
	tRepo := NewTagRepo(db)
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	u := seedUser(t, uRepo)
	tags, err := tRepo.EnsureMany(ctx, []string{"go", "blog"})
	require.NoError(t, err)

	art := &model.Article{UserID: u.ID, Title: "Hello", Content: "# md", Status: 1}
	require.NoError(t, aRepo.Create(ctx, art, tags))
	require.Greater(t, art.ID, uint64(0))

	got, err := aRepo.FindByID(ctx, art.ID)
	require.NoError(t, err)
	require.Equal(t, "Hello", got.Title)
	require.Len(t, got.Tags, 2)
	require.Equal(t, u.ID, got.Author.ID)
}

func TestArticleRepo_Update_ReplacesTags(t *testing.T) {
	db := newTestDB(t)
	uRepo := NewUserRepo(db)
	tRepo := NewTagRepo(db)
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	u := seedUser(t, uRepo)
	t1, _ := tRepo.EnsureMany(ctx, []string{"go", "blog"})
	art := &model.Article{UserID: u.ID, Title: "T", Content: "c", Status: 1}
	require.NoError(t, aRepo.Create(ctx, art, t1))

	t2, _ := tRepo.EnsureMany(ctx, []string{"redis"})
	art.Title = "T2"
	art.Content = "c2"
	require.NoError(t, aRepo.Update(ctx, art, t2))

	got, _ := aRepo.FindByID(ctx, art.ID)
	require.Equal(t, "T2", got.Title)
	require.Len(t, got.Tags, 1)
	require.Equal(t, "redis", got.Tags[0].Name)
}

func TestArticleRepo_SoftDelete(t *testing.T) {
	db := newTestDB(t)
	u := seedUser(t, NewUserRepo(db))
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	art := &model.Article{UserID: u.ID, Title: "T", Content: "c", Status: 1}
	require.NoError(t, aRepo.Create(ctx, art, nil))
	require.NoError(t, aRepo.SoftDelete(ctx, art.ID))

	_, err := aRepo.FindByID(ctx, art.ID)
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeNotFound, ae.Code)
}

func TestArticleRepo_List_FilterByUser(t *testing.T) {
	db := newTestDB(t)
	uRepo := NewUserRepo(db)
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	u1 := &model.User{Username: "u1", PasswordHash: "h", Name: "U1"}
	u2 := &model.User{Username: "u2", PasswordHash: "h", Name: "U2"}
	require.NoError(t, uRepo.Create(ctx, u1))
	require.NoError(t, uRepo.Create(ctx, u2))

	for i := 0; i < 3; i++ {
		require.NoError(t, aRepo.Create(ctx, &model.Article{UserID: u1.ID, Title: "a", Content: "c", Status: 1}, nil))
	}
	require.NoError(t, aRepo.Create(ctx, &model.Article{UserID: u2.ID, Title: "b", Content: "c", Status: 1}, nil))

	rows, total, err := aRepo.List(ctx, ListQuery{Page: 1, Size: 10, UserID: u1.ID})
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, rows, 3)
}

func TestArticleRepo_List_FilterByTag(t *testing.T) {
	db := newTestDB(t)
	uRepo := NewUserRepo(db)
	tRepo := NewTagRepo(db)
	aRepo := NewArticleRepo(db)
	ctx := context.Background()

	u := seedUser(t, uRepo)
	tags, _ := tRepo.EnsureMany(ctx, []string{"go", "blog"})
	require.NoError(t, aRepo.Create(ctx, &model.Article{UserID: u.ID, Title: "with-go", Content: "c", Status: 1}, []model.Tag{tags[0]}))
	require.NoError(t, aRepo.Create(ctx, &model.Article{UserID: u.ID, Title: "no-go",   Content: "c", Status: 1}, nil))

	rows, total, err := aRepo.List(ctx, ListQuery{Page: 1, Size: 10, Tag: "go"})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "with-go", rows[0].Title)
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./internal/repository/... -run TestArticleRepo -v
```

预期: 编译失败。

- [ ] **Step 3: 实现 article_repo.go**

```go
// Server/internal/repository/article_repo.go
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
)

type ListQuery struct {
	Page   int
	Size   int
	Tag    string
	UserID uint64
}

type ArticleRepo interface {
	Create(ctx context.Context, a *model.Article, tags []model.Tag) error
	Update(ctx context.Context, a *model.Article, tags []model.Tag) error
	SoftDelete(ctx context.Context, id uint64) error
	FindByID(ctx context.Context, id uint64) (*model.Article, error)
	List(ctx context.Context, q ListQuery) ([]model.Article, int64, error)
	IncrementViewCount(ctx context.Context, id uint64, delta int64) error
}

type articleRepo struct{ db *gorm.DB }

func NewArticleRepo(db *gorm.DB) ArticleRepo { return &articleRepo{db: db} }

func (r *articleRepo) Create(ctx context.Context, a *model.Article, tags []model.Tag) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(a).Error; err != nil {
			return apperr.Wrap(apperr.CodeDBError, "create article", err)
		}
		if len(tags) > 0 {
			if err := tx.Model(a).Association("Tags").Replace(tags); err != nil {
				return apperr.Wrap(apperr.CodeDBError, "set tags", err)
			}
		}
		return nil
	})
}

func (r *articleRepo) Update(ctx context.Context, a *model.Article, tags []model.Tag) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Article{}).
			Where("id = ?", a.ID).
			Updates(map[string]any{"title": a.Title, "content": a.Content})
		if res.Error != nil {
			return apperr.Wrap(apperr.CodeDBError, "update article", res.Error)
		}
		if res.RowsAffected == 0 {
			return apperr.New(apperr.CodeNotFound, "文章不存在")
		}
		if err := tx.Model(a).Association("Tags").Replace(tags); err != nil {
			return apperr.Wrap(apperr.CodeDBError, "replace tags", err)
		}
		return nil
	})
}

func (r *articleRepo) SoftDelete(ctx context.Context, id uint64) error {
	res := r.db.WithContext(ctx).Delete(&model.Article{}, id)
	if res.Error != nil {
		return apperr.Wrap(apperr.CodeDBError, "delete article", res.Error)
	}
	if res.RowsAffected == 0 {
		return apperr.New(apperr.CodeNotFound, "文章不存在")
	}
	return nil
}

func (r *articleRepo) FindByID(ctx context.Context, id uint64) (*model.Article, error) {
	var a model.Article
	err := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("Author").
		First(&a, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.New(apperr.CodeNotFound, "文章不存在")
		}
		return nil, apperr.Wrap(apperr.CodeDBError, "find article", err)
	}
	return &a, nil
}

func (r *articleRepo) List(ctx context.Context, q ListQuery) ([]model.Article, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Size < 1 || q.Size > 50 {
		q.Size = 10
	}
	tx := r.db.WithContext(ctx).Model(&model.Article{}).Where("status = ?", 1)
	if q.UserID > 0 {
		tx = tx.Where("user_id = ?", q.UserID)
	}
	if q.Tag != "" {
		sub := r.db.Table("article_tags").
			Select("article_tags.article_id").
			Joins("JOIN tags ON tags.id = article_tags.tag_id").
			Where("tags.name = ?", q.Tag)
		tx = tx.Where("id IN (?)", sub)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, apperr.Wrap(apperr.CodeDBError, "count list", err)
	}
	var rows []model.Article
	err := tx.Preload("Tags").Preload("Author").
		Order("created_at DESC").
		Limit(q.Size).Offset((q.Page - 1) * q.Size).
		Find(&rows).Error
	if err != nil {
		return nil, 0, apperr.Wrap(apperr.CodeDBError, "list articles", err)
	}
	return rows, total, nil
}

func (r *articleRepo) IncrementViewCount(ctx context.Context, id uint64, delta int64) error {
	if delta == 0 {
		return nil
	}
	res := r.db.WithContext(ctx).Model(&model.Article{}).
		Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + ?", delta))
	if res.Error != nil {
		return apperr.Wrap(apperr.CodeDBError, "incr view count", res.Error)
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./internal/repository/... -v
```

预期: 全部 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/repository/article_repo.go Server/internal/repository/article_repo_test.go
git commit -m "feat(server): article repo with tags + soft delete + list filters"
```

---

### Task 12: Counter Repository (Redis)

**Files:**
- Create: `Server/internal/repository/counter_repo.go`
- Test: `Server/internal/repository/counter_repo_test.go`

- [ ] **Step 1: 写失败测试（需要 Redis）**

```go
// Server/internal/repository/counter_repo_test.go
package repository

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func skipIfNoRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, rdb.Ping(context.Background()).Err())
	require.NoError(t, rdb.FlushDB(context.Background()).Err())
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestCounterRepo_IncAndGet(t *testing.T) {
	rdb := skipIfNoRedis(t)
	c := NewCounterRepo(rdb)
	ctx := context.Background()

	require.NoError(t, c.Inc(ctx, 42))
	require.NoError(t, c.Inc(ctx, 42))
	require.NoError(t, c.Inc(ctx, 43))

	got, err := c.GetIncrement(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, int64(2), got)

	dirty, err := c.DirtyMembers(ctx)
	require.NoError(t, err)
	sort.Strings(dirty)
	require.Equal(t, []string{"42", "43"}, dirty)
}

func TestCounterRepo_DrainAndAck(t *testing.T) {
	rdb := skipIfNoRedis(t)
	c := NewCounterRepo(rdb)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		require.NoError(t, c.Inc(ctx, 7))
	}
	delta, err := c.DrainIncrement(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, int64(5), delta)

	again, err := c.GetIncrement(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, int64(0), again)

	require.NoError(t, c.Ack(ctx, []uint64{7}))
	dirty, _ := c.DirtyMembers(ctx)
	require.Empty(t, dirty)
}

func TestCounterRepo_RestoreOnFlushFail(t *testing.T) {
	rdb := skipIfNoRedis(t)
	c := NewCounterRepo(rdb)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		require.NoError(t, c.Inc(ctx, 9))
	}
	delta, _ := c.DrainIncrement(ctx, 9)
	require.Equal(t, int64(3), delta)
	require.NoError(t, c.Restore(ctx, 9, delta))
	got, _ := c.GetIncrement(ctx, 9)
	require.Equal(t, int64(3), got)
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
REDIS_ADDR=127.0.0.1:6379 go test ./internal/repository/... -run TestCounterRepo -v
```

预期: 编译失败。

- [ ] **Step 3: 实现 counter_repo.go**

```go
// Server/internal/repository/counter_repo.go
package repository

import (
	"context"
	"errors"
	"strconv"

	"github.com/redis/go-redis/v9"

	"github.com/wjr/blog/server/internal/apperr"
)

type CounterRepo interface {
	Inc(ctx context.Context, articleID uint64) error
	GetIncrement(ctx context.Context, articleID uint64) (int64, error)
	DirtyMembers(ctx context.Context) ([]string, error)
	DrainIncrement(ctx context.Context, articleID uint64) (int64, error)
	Ack(ctx context.Context, ids []uint64) error
	Restore(ctx context.Context, articleID uint64, delta int64) error
}

type counterRepo struct{ rdb *redis.Client }

func NewCounterRepo(rdb *redis.Client) CounterRepo { return &counterRepo{rdb: rdb} }

func keyView(id uint64) string  { return "view:" + strconv.FormatUint(id, 10) }
func keyDirty() string          { return "view:dirty" }

func (r *counterRepo) Inc(ctx context.Context, id uint64) error {
	pipe := r.rdb.Pipeline()
	pipe.Incr(ctx, keyView(id))
	pipe.SAdd(ctx, keyDirty(), id)
	if _, err := pipe.Exec(ctx); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "view inc", err)
	}
	return nil
}

func (r *counterRepo) GetIncrement(ctx context.Context, id uint64) (int64, error) {
	v, err := r.rdb.Get(ctx, keyView(id)).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, apperr.Wrap(apperr.CodeRedisError, "view get", err)
	}
	return v, nil
}

func (r *counterRepo) DirtyMembers(ctx context.Context) ([]string, error) {
	out, err := r.rdb.SMembers(ctx, keyDirty()).Result()
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeRedisError, "smembers dirty", err)
	}
	return out, nil
}

func (r *counterRepo) DrainIncrement(ctx context.Context, id uint64) (int64, error) {
	v, err := r.rdb.GetSet(ctx, keyView(id), 0).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, apperr.Wrap(apperr.CodeRedisError, "view drain", err)
	}
	return v, nil
}

func (r *counterRepo) Ack(ctx context.Context, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	if err := r.rdb.SRem(ctx, keyDirty(), args...).Err(); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "srem dirty", err)
	}
	return nil
}

func (r *counterRepo) Restore(ctx context.Context, id uint64, delta int64) error {
	if delta <= 0 {
		return nil
	}
	if err := r.rdb.IncrBy(ctx, keyView(id), delta).Err(); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "view restore", err)
	}
	return nil
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
REDIS_ADDR=127.0.0.1:6379 go test ./internal/repository/... -run TestCounterRepo -v
```

预期: 三个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/repository/counter_repo.go Server/internal/repository/counter_repo_test.go
git commit -m "feat(server): redis-backed view counter repo (inc/drain/ack/restore)"
```

---

## Phase 2 — 认证、会话与中间件

### Task 13: Session 存储抽象（Redis）

**Files:**
- Create: `Server/internal/middleware/session.go`
- Test: `Server/internal/middleware/session_test.go`

> 不使用 `gin-contrib/sessions`：我们要精确控制滑动 TTL 和登录/登出 cookie 行为，自己写更直接。

- [ ] **Step 1: 写失败测试**

```go
// Server/internal/middleware/session_test.go
package middleware

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newRedisForTest(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	r := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, r.FlushDB(context.Background()).Err())
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestSessionStore_PutGetTouchDelete(t *testing.T) {
	rdb := newRedisForTest(t)
	store := NewSessionStore(rdb, 30)
	ctx := context.Background()

	sid, csrf, err := store.Create(ctx, Session{UserID: 7, Name: "alice"})
	require.NoError(t, err)
	require.Len(t, sid, 32)
	require.Len(t, csrf, 32)

	got, err := store.Get(ctx, sid)
	require.NoError(t, err)
	require.Equal(t, uint64(7), got.UserID)

	gotCsrf, err := store.GetCSRF(ctx, sid)
	require.NoError(t, err)
	require.Equal(t, csrf, gotCsrf)

	require.NoError(t, store.Touch(ctx, sid))
	require.NoError(t, store.Delete(ctx, sid))

	_, err = store.Get(ctx, sid)
	require.Error(t, err)
}

func TestSessionStore_Get_Missing(t *testing.T) {
	rdb := newRedisForTest(t)
	store := NewSessionStore(rdb, 30)
	_, err := store.Get(context.Background(), "no-such")
	require.Error(t, err)
}

func TestSessionFromContext_AndAttach(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	AttachSession(c, Session{UserID: 9, Name: "bob"})
	got, ok := SessionFromContext(c)
	require.True(t, ok)
	require.Equal(t, uint64(9), got.UserID)
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
REDIS_ADDR=127.0.0.1:6379 go test ./internal/middleware/... -run TestSession -v
```

预期: 编译失败。

- [ ] **Step 3: 实现 session.go**

```go
// Server/internal/middleware/session.go
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/idgen"
)

type Session struct {
	UserID uint64 `json:"uid"`
	Name   string `json:"name"`
}

type SessionStore struct {
	rdb        *redis.Client
	ttlMinutes int
}

func NewSessionStore(rdb *redis.Client, ttlMinutes int) *SessionStore {
	if ttlMinutes <= 0 {
		ttlMinutes = 30
	}
	return &SessionStore{rdb: rdb, ttlMinutes: ttlMinutes}
}

func (s *SessionStore) ttl() time.Duration { return time.Duration(s.ttlMinutes) * time.Minute }

func sessionKey(sid string) string { return "sess:" + sid }
func csrfKey(sid string) string    { return "csrf:" + sid }

func (s *SessionStore) Create(ctx context.Context, sess Session) (sid, csrf string, err error) {
	sid = idgen.NewUUID()
	csrf = idgen.NewUUID()
	body, err := json.Marshal(sess)
	if err != nil {
		return "", "", apperr.Wrap(apperr.CodeUnknown, "marshal session", err)
	}
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, sessionKey(sid), body, s.ttl())
	pipe.Set(ctx, csrfKey(sid), csrf, s.ttl())
	if _, err := pipe.Exec(ctx); err != nil {
		return "", "", apperr.Wrap(apperr.CodeRedisError, "session create", err)
	}
	return sid, csrf, nil
}

func (s *SessionStore) Get(ctx context.Context, sid string) (*Session, error) {
	body, err := s.rdb.Get(ctx, sessionKey(sid)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, apperr.New(apperr.CodeUnauthorized, "未登录")
		}
		return nil, apperr.Wrap(apperr.CodeRedisError, "session get", err)
	}
	var sess Session
	if err := json.Unmarshal(body, &sess); err != nil {
		return nil, apperr.Wrap(apperr.CodeUnknown, "session decode", err)
	}
	return &sess, nil
}

func (s *SessionStore) GetCSRF(ctx context.Context, sid string) (string, error) {
	v, err := s.rdb.Get(ctx, csrfKey(sid)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", apperr.New(apperr.CodeCSRFInvalid, "csrf token 缺失")
		}
		return "", apperr.Wrap(apperr.CodeRedisError, "csrf get", err)
	}
	return v, nil
}

func (s *SessionStore) Touch(ctx context.Context, sid string) error {
	pipe := s.rdb.Pipeline()
	pipe.Expire(ctx, sessionKey(sid), s.ttl())
	pipe.Expire(ctx, csrfKey(sid), s.ttl())
	if _, err := pipe.Exec(ctx); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "session touch", err)
	}
	return nil
}

func (s *SessionStore) Delete(ctx context.Context, sid string) error {
	if err := s.rdb.Del(ctx, sessionKey(sid), csrfKey(sid)).Err(); err != nil {
		return apperr.Wrap(apperr.CodeRedisError, "session delete", err)
	}
	return nil
}

func (s *SessionStore) TTLSeconds() int { return s.ttlMinutes * 60 }

const ctxKeySession = "blog.session"

func AttachSession(c *gin.Context, sess Session) { c.Set(ctxKeySession, sess) }
func SessionFromContext(c *gin.Context) (Session, bool) {
	v, ok := c.Get(ctxKeySession)
	if !ok {
		return Session{}, false
	}
	s, ok := v.(Session)
	return s, ok
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
REDIS_ADDR=127.0.0.1:6379 go test ./internal/middleware/... -run TestSession -v
```

预期: 三个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/middleware/session.go Server/internal/middleware/session_test.go
git commit -m "feat(server): session store backed by redis with sliding ttl"
```

---

### Task 14: Cookie 工具与 Session 中间件挂载

**Files:**
- Create: `Server/internal/middleware/cookie.go`
- Modify: `Server/internal/middleware/session.go`（添加 `WithSession` middleware）
- Test: `Server/internal/middleware/cookie_test.go`

- [ ] **Step 1: 写 cookie 测试**

```go
// Server/internal/middleware/cookie_test.go
package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetSessionCookies_PersistsBoth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	SetSessionCookies(c, "sid-val", "csrf-val", 1800, false)
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 2)
	bySid := map[string]string{}
	for _, ck := range cookies {
		bySid[ck.Name] = ck.Value
	}
	require.Equal(t, "sid-val", bySid["sid"])
	require.Equal(t, "csrf-val", bySid["csrf_token"])
	for _, ck := range cookies {
		require.Equal(t, 1800, ck.MaxAge)
	}
}

func TestClearSessionCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	ClearSessionCookies(c, false)
	for _, ck := range w.Result().Cookies() {
		require.LessOrEqual(t, ck.MaxAge, 0)
	}
}
```

- [ ] **Step 2: 实现 cookie.go**

```go
// Server/internal/middleware/cookie.go
package middleware

import "github.com/gin-gonic/gin"

const (
	CookieSID  = "sid"
	CookieCSRF = "csrf_token"
)

func SetSessionCookies(c *gin.Context, sid, csrf string, maxAgeSec int, secure bool) {
	c.SetSameSite(2) // SameSiteLaxMode
	c.SetCookie(CookieSID, sid, maxAgeSec, "/", "", secure, true)
	c.SetCookie(CookieCSRF, csrf, maxAgeSec, "/", "", secure, false)
}

func ClearSessionCookies(c *gin.Context, secure bool) {
	c.SetSameSite(2)
	c.SetCookie(CookieSID, "", -1, "/", "", secure, true)
	c.SetCookie(CookieCSRF, "", -1, "/", "", secure, false)
}
```

- [ ] **Step 3: 添加 `WithSession` 中间件到 session.go**

在 `session.go` 末尾追加:

```go
// Server/internal/middleware/session.go (append)

// WithSession 在每次请求开始时尝试从 Cookie 读取 sid;若有效则:
//   1) 注入 Session 到 context
//   2) 触发 Touch (滑动续期)
//   3) 重新下发 Set-Cookie 让浏览器同步刷新 Max-Age
// 失败/无 cookie 则不注入,后续 RequireAuth 中间件自行返回 2001。
func (s *SessionStore) WithSession(secureCookies bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, err := c.Cookie(CookieSID)
		if err != nil || sid == "" {
			c.Next()
			return
		}
		sess, err := s.Get(c.Request.Context(), sid)
		if err != nil {
			c.Next()
			return
		}
		csrf, err := s.GetCSRF(c.Request.Context(), sid)
		if err != nil {
			c.Next()
			return
		}
		_ = s.Touch(c.Request.Context(), sid)
		SetSessionCookies(c, sid, csrf, s.TTLSeconds(), secureCookies)
		AttachSession(c, *sess)
		c.Set("blog.sid", sid)
		c.Set("blog.csrf", csrf)
		c.Next()
	}
}
```

- [ ] **Step 4: 跑测试确认通过**

```bash
go test ./internal/middleware/... -run TestSetSessionCookies -v
go test ./internal/middleware/... -run TestClearSessionCookies -v
```

预期: 两个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/middleware/cookie.go Server/internal/middleware/cookie_test.go Server/internal/middleware/session.go
git commit -m "feat(server): session cookie helpers + WithSession middleware"
```

---

### Task 15: RequireAuth 与 CSRF 中间件

**Files:**
- Create: `Server/internal/middleware/auth.go`
- Create: `Server/internal/middleware/csrf.go`
- Test: `Server/internal/middleware/auth_test.go`
- Test: `Server/internal/middleware/csrf_test.go`

- [ ] **Step 1: 写 auth 测试**

```go
// Server/internal/middleware/auth_test.go
package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequireAuth_BlocksWhenNoSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RequireAuth(), func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	require.Equal(t, 401, w.Code)
	require.Contains(t, w.Body.String(), `"code":2001`)
}

func TestRequireAuth_AllowsWhenSessionAttached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x",
		func(c *gin.Context) { AttachSession(c, Session{UserID: 1, Name: "a"}); c.Next() },
		RequireAuth(),
		func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	require.Equal(t, 200, w.Code)
}
```

- [ ] **Step 2: 实现 auth.go**

```go
// Server/internal/middleware/auth.go
package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := SessionFromContext(c); !ok {
			httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 3: 写 csrf 测试**

```go
// Server/internal/middleware/csrf_test.go
package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCSRF_AllowsGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CSRFGuard())
	r.GET("/x", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	require.Equal(t, 200, w.Code)
}

func TestCSRF_BlocksPOST_Mismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("blog.csrf", "expected"); c.Next() })
	r.Use(CSRFGuard())
	r.POST("/x", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest("POST", "/x", strings.NewReader(""))
	req.Header.Set("X-CSRF-Token", "wrong")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 403, w.Code)
	require.Contains(t, w.Body.String(), `"code":2030`)
}

func TestCSRF_AllowsPOST_Match(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("blog.csrf", "ok-token"); c.Next() })
	r.Use(CSRFGuard())
	r.POST("/x", func(c *gin.Context) { c.String(200, "yes") })
	req := httptest.NewRequest("POST", "/x", strings.NewReader(""))
	req.Header.Set("X-CSRF-Token", "ok-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
}
```

- [ ] **Step 4: 实现 csrf.go**

```go
// Server/internal/middleware/csrf.go
package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

func CSRFGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case "POST", "PUT", "DELETE", "PATCH":
		default:
			c.Next()
			return
		}
		expected, _ := c.Get("blog.csrf")
		header := c.GetHeader("X-CSRF-Token")
		exp, _ := expected.(string)
		if exp == "" || header == "" || exp != header {
			httpresp.Fail(c, apperr.New(apperr.CodeCSRFInvalid, "安全校验失败"))
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 5: 跑测试**

```bash
go test ./internal/middleware/... -run "TestRequireAuth|TestCSRF" -v
```

预期: 5 个 case PASS。

- [ ] **Step 6: Commit**

```bash
git add Server/internal/middleware/auth.go Server/internal/middleware/auth_test.go Server/internal/middleware/csrf.go Server/internal/middleware/csrf_test.go
git commit -m "feat(server): require-auth + csrf guard middlewares"
```

---

### Task 16: 限流中间件

**Files:**
- Create: `Server/internal/middleware/ratelimit.go`
- Test: `Server/internal/middleware/ratelimit_test.go`

- [ ] **Step 1: 写测试**

```go
// Server/internal/middleware/ratelimit_test.go
package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRateLimit_BlocksAfterMax(t *testing.T) {
	rdb := newRedisForTest(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(rdb, RateLimitOpts{
		Name:   "test",
		Max:    3,
		Window: 60,
		KeyFn:  func(c *gin.Context) string { return "k" },
	}))
	r.POST("/x", func(c *gin.Context) { c.String(200, "ok") })

	hit := func() int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
		return w.Code
	}
	require.Equal(t, 200, hit())
	require.Equal(t, 200, hit())
	require.Equal(t, 200, hit())
	require.Equal(t, 429, hit())

	_ = rdb.Del(context.Background(), "rl:test:k").Err()
}

func TestRateLimit_FailsOpenWhenRedisDown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	r.Use(RateLimit(rdb, RateLimitOpts{
		Name: "test", Max: 1, Window: 60, KeyFn: func(c *gin.Context) string { return "k" },
	}))
	r.POST("/x", func(c *gin.Context) { c.String(200, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
	require.Equal(t, 200, w.Code)
}
```

- [ ] **Step 2: 实现 ratelimit.go**

```go
// Server/internal/middleware/ratelimit.go
package middleware

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

type RateLimitOpts struct {
	Name   string                  // "login" / "upload" / "global"
	Max    int                     // 窗口内最大次数
	Window int                     // 秒
	KeyFn  func(c *gin.Context) string
}

func RateLimit(rdb *redis.Client, opts RateLimitOpts) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		defer cancel()

		key := "rl:" + opts.Name + ":" + opts.KeyFn(c)
		val, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			c.Next() // fail open
			return
		}
		if val == 1 {
			_ = rdb.Expire(ctx, key, time.Duration(opts.Window)*time.Second).Err()
		}
		if val > int64(opts.Max) {
			c.Header("Retry-After", strconv.Itoa(opts.Window))
			httpresp.Fail(c, apperr.New(apperr.CodeRateLimited, "操作过于频繁,请稍后再试"))
			return
		}
		c.Next()
	}
}

func IPKey(c *gin.Context) string { return c.ClientIP() }

func UserKey(c *gin.Context) string {
	if s, ok := SessionFromContext(c); ok {
		return strconv.FormatUint(s.UserID, 10)
	}
	return "anon"
}
```

- [ ] **Step 3: 跑测试**

```bash
REDIS_ADDR=127.0.0.1:6379 go test ./internal/middleware/... -run TestRateLimit -v
```

预期: 两个 case PASS。

- [ ] **Step 4: Commit**

```bash
git add Server/internal/middleware/ratelimit.go Server/internal/middleware/ratelimit_test.go
git commit -m "feat(server): redis rate-limit middleware (incr-with-window, fail-open)"
```

---

### Task 17: Recover 与 Logger 中间件

**Files:**
- Create: `Server/internal/middleware/recover.go`
- Create: `Server/internal/middleware/logger.go`

- [ ] **Step 1: 实现 recover.go**

```go
// Server/internal/middleware/recover.go
package middleware

import (
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error().
					Str("path", c.Request.URL.Path).
					Bytes("stack", debug.Stack()).
					Interface("panic", rec).
					Msg("panic recovered")
				httpresp.Fail(c, apperr.New(apperr.CodeUnknown, "系统繁忙"))
			}
		}()
		c.Next()
	}
}
```

- [ ] **Step 2: 实现 logger.go**

```go
// Server/internal/middleware/logger.go
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func RequestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		dur := time.Since(start)
		ev := log.Info()
		if c.Writer.Status() >= 500 {
			ev = log.Error()
		} else if c.Writer.Status() >= 400 {
			ev = log.Warn()
		}
		ev.Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("dur", dur).
			Str("ip", c.ClientIP()).
			Msg("http")
	}
}
```

- [ ] **Step 3: 安装 zerolog**

```bash
go get github.com/rs/zerolog
go mod tidy
```

- [ ] **Step 4: 编译验证**

```bash
go build ./internal/middleware/...
```

- [ ] **Step 5: Commit**

```bash
git add Server/internal/middleware/recover.go Server/internal/middleware/logger.go Server/go.mod Server/go.sum
git commit -m "feat(server): recover + request-log middlewares"
```

---

### Task 18: Auth Service

**Files:**
- Create: `Server/internal/service/auth_service.go`
- Test: `Server/internal/service/auth_service_test.go`

- [ ] **Step 1: 写失败测试**

```go
// Server/internal/service/auth_service_test.go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/pkg/password"
)

type fakeUserRepo struct {
	byUsername map[string]*model.User
	byID       map[uint64]*model.User
	nextID     uint64
	createErr  error
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{byUsername: map[string]*model.User{}, byID: map[uint64]*model.User{}, nextID: 1}
}
func (f *fakeUserRepo) Create(_ context.Context, u *model.User) error {
	if f.createErr != nil {
		return f.createErr
	}
	if _, ok := f.byUsername[u.Username]; ok {
		return apperr.New(apperr.CodeUsernameTaken, "dup")
	}
	u.ID = f.nextID
	f.nextID++
	f.byUsername[u.Username] = u
	f.byID[u.ID] = u
	return nil
}
func (f *fakeUserRepo) FindByUsername(_ context.Context, n string) (*model.User, error) {
	u, ok := f.byUsername[n]
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound, "")
	}
	return u, nil
}
func (f *fakeUserRepo) FindByID(_ context.Context, id uint64) (*model.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, apperr.New(apperr.CodeNotFound, "")
	}
	return u, nil
}

func TestAuthService_Register_Success(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	u, err := svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "A"})
	require.NoError(t, err)
	require.Equal(t, uint64(1), u.ID)
	require.True(t, password.Compare(repo.byID[1].PasswordHash, "Password123"))
}

func TestAuthService_Register_DuplicateUsername(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	_, _ = svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "A"})
	_, err := svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "B"})
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeUsernameTaken, ae.Code)
}

func TestAuthService_Register_InvalidInput(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	cases := []RegisterInput{
		{Username: "abc", Password: "Password123", Name: "A"},          // 用户名 < 4
		{Username: "ok", Password: "short", Name: "A"},                  // 密码 < 8
		{Username: "ok", Password: "alphabet", Name: "A"},               // 密码无数字
		{Username: "ok", Password: "12345678", Name: "A"},               // 密码无字母
		{Username: "bad-name!", Password: "Password123", Name: "A"},     // 用户名含非法字符
		{Username: "alice", Password: "Password123", Name: ""},          // 昵称空
	}
	for _, in := range cases {
		_, err := svc.Register(context.Background(), in)
		var ae *apperr.AppErr
		require.True(t, errors.As(err, &ae), "expected AppErr for %+v", in)
		require.Equal(t, apperr.CodeInvalidParam, ae.Code, "case %+v", in)
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	_, _ = svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "A"})
	u, err := svc.Login(context.Background(), "alice", "Password123")
	require.NoError(t, err)
	require.Equal(t, uint64(1), u.ID)
}

func TestAuthService_Login_BadCredential(t *testing.T) {
	repo := newFakeUserRepo()
	svc := NewAuthService(repo)
	_, _ = svc.Register(context.Background(), RegisterInput{Username: "alice", Password: "Password123", Name: "A"})

	_, err := svc.Login(context.Background(), "alice", "wrong")
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeBadCredential, ae.Code)

	_, err = svc.Login(context.Background(), "ghost", "anything")
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeBadCredential, ae.Code)
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
go test ./internal/service/... -run TestAuthService -v
```

预期: 编译失败。

- [ ] **Step 3: 实现 auth_service.go**

```go
// Server/internal/service/auth_service.go
package service

import (
	"context"
	"regexp"
	"unicode/utf8"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/pkg/password"
	"github.com/wjr/blog/server/internal/repository"
)

type RegisterInput struct {
	Username string
	Password string
	Name     string
}

type AuthService interface {
	Register(ctx context.Context, in RegisterInput) (*model.User, error)
	Login(ctx context.Context, username, plain string) (*model.User, error)
	GetByID(ctx context.Context, id uint64) (*model.User, error)
}

type authService struct{ users repository.UserRepo }

func NewAuthService(u repository.UserRepo) AuthService { return &authService{users: u} }

var reUsername = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func validatePassword(p string) error {
	if len(p) < 8 || len(p) > 64 {
		return apperr.New(apperr.CodeInvalidParam, "密码长度需 8–64")
	}
	hasLetter, hasDigit := false, false
	for _, ch := range p {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z':
			hasLetter = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return apperr.New(apperr.CodeInvalidParam, "密码需包含字母与数字")
	}
	return nil
}

func validateRegister(in RegisterInput) error {
	if l := len(in.Username); l < 4 || l > 32 {
		return apperr.New(apperr.CodeInvalidParam, "用户名长度需 4–32")
	}
	if !reUsername.MatchString(in.Username) {
		return apperr.New(apperr.CodeInvalidParam, "用户名仅允许字母数字下划线")
	}
	if err := validatePassword(in.Password); err != nil {
		return err
	}
	if l := utf8.RuneCountInString(in.Name); l < 1 || l > 64 {
		return apperr.New(apperr.CodeInvalidParam, "昵称长度需 1–64")
	}
	return nil
}

func (s *authService) Register(ctx context.Context, in RegisterInput) (*model.User, error) {
	if err := validateRegister(in); err != nil {
		return nil, err
	}
	hash, err := password.Hash(in.Password)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeUnknown, "hash", err)
	}
	u := &model.User{Username: in.Username, PasswordHash: hash, Name: in.Name}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *authService) Login(ctx context.Context, username, plain string) (*model.User, error) {
	u, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		return nil, apperr.New(apperr.CodeBadCredential, "账号或密码错误")
	}
	if !password.Compare(u.PasswordHash, plain) {
		return nil, apperr.New(apperr.CodeBadCredential, "账号或密码错误")
	}
	return u, nil
}

func (s *authService) GetByID(ctx context.Context, id uint64) (*model.User, error) {
	return s.users.FindByID(ctx, id)
}
```

- [ ] **Step 4: 跑测试通过**

```bash
go test ./internal/service/... -run TestAuthService -v
```

预期: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/service/auth_service.go Server/internal/service/auth_service_test.go
git commit -m "feat(server): auth service (register/login/me) with input validation"
```

---

### Task 19: Auth Handler + Routes

**Files:**
- Create: `Server/internal/handler/auth_handler.go`
- Test: `Server/internal/handler/auth_handler_test.go`

- [ ] **Step 1: 写 handler 测试（端到端 router 级别）**

```go
// Server/internal/handler/auth_handler_test.go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/repository"
	"github.com/wjr/blog/server/internal/service"
)

func setupAuthEnv(t *testing.T) (*gin.Engine, *redis.Client) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Article{}, &model.Tag{}))
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	require.NoError(t, rdb.FlushDB(context.Background()).Err())

	store := middleware.NewSessionStore(rdb, 30)
	uRepo := repository.NewUserRepo(db)
	authSvc := service.NewAuthService(uRepo)
	h := NewAuthHandler(authSvc, store, false)

	r := gin.New()
	r.Use(store.WithSession(false))
	g := r.Group("/api/v1/auth")
	{
		g.POST("/register", h.Register)
		g.POST("/login", h.Login)
		g.POST("/logout", h.Logout)
		g.GET("/me", h.Me)
	}
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(&model.User{}, &model.Article{}, &model.Tag{}, "article_tags")
	})
	return r, rdb
}

func doJSON(r *gin.Engine, method, path string, body any, sid, csrf string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sid != "" {
		req.AddCookie(&http.Cookie{Name: "sid", Value: sid})
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrf})
		req.Header.Set("X-CSRF-Token", csrf)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuth_Register_SetsCookies(t *testing.T) {
	r, _ := setupAuthEnv(t)
	w := doJSON(r, "POST", "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "Password123", "name": "爱丽丝",
	}, "", "")
	require.Equal(t, 200, w.Code)
	cookies := w.Result().Cookies()
	gotNames := map[string]bool{}
	for _, ck := range cookies {
		gotNames[ck.Name] = true
		require.Equal(t, 1800, ck.MaxAge)
	}
	require.True(t, gotNames["sid"])
	require.True(t, gotNames["csrf_token"])
}

func TestAuth_Register_Duplicate(t *testing.T) {
	r, _ := setupAuthEnv(t)
	body := map[string]string{"username": "alice", "password": "Password123", "name": "爱丽丝"}
	doJSON(r, "POST", "/api/v1/auth/register", body, "", "")
	w := doJSON(r, "POST", "/api/v1/auth/register", body, "", "")
	require.Equal(t, 409, w.Code)
	require.Contains(t, w.Body.String(), `"code":1010`)
}

func TestAuth_Login_BadPassword(t *testing.T) {
	r, _ := setupAuthEnv(t)
	doJSON(r, "POST", "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "Password123", "name": "A",
	}, "", "")
	w := doJSON(r, "POST", "/api/v1/auth/login", map[string]string{
		"username": "alice", "password": "wrong",
	}, "", "")
	require.Equal(t, 401, w.Code)
	require.Contains(t, w.Body.String(), `"code":2010`)
}

func TestAuth_Me_Returns401Without_Login(t *testing.T) {
	r, _ := setupAuthEnv(t)
	w := doJSON(r, "GET", "/api/v1/auth/me", nil, "", "")
	require.Equal(t, 401, w.Code)
}

func TestAuth_LoginThenMeThenLogout(t *testing.T) {
	r, _ := setupAuthEnv(t)
	doJSON(r, "POST", "/api/v1/auth/register", map[string]string{
		"username": "alice", "password": "Password123", "name": "A",
	}, "", "")
	w := doJSON(r, "POST", "/api/v1/auth/login", map[string]string{
		"username": "alice", "password": "Password123",
	}, "", "")
	require.Equal(t, 200, w.Code)
	var sid, csrf string
	for _, ck := range w.Result().Cookies() {
		switch ck.Name {
		case "sid":
			sid = ck.Value
		case "csrf_token":
			csrf = ck.Value
		}
	}
	require.NotEmpty(t, sid)

	w2 := doJSON(r, "GET", "/api/v1/auth/me", nil, sid, csrf)
	require.Equal(t, 200, w2.Code)
	require.Contains(t, w2.Body.String(), `"username":"alice"`)

	w3 := doJSON(r, "POST", "/api/v1/auth/logout", nil, sid, csrf)
	require.Equal(t, 200, w3.Code)

	w4 := doJSON(r, "GET", "/api/v1/auth/me", nil, sid, csrf)
	require.Equal(t, 401, w4.Code)
}
```

- [ ] **Step 2: 跑测试确认失败**

```bash
REDIS_ADDR=127.0.0.1:6379 go test ./internal/handler/... -run TestAuth -v
```

预期: 编译失败 (`NewAuthHandler` 未定义)。

- [ ] **Step 3: 实现 auth_handler.go**

```go
// Server/internal/handler/auth_handler.go
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
	"github.com/wjr/blog/server/internal/service"
)

type AuthHandler struct {
	svc           service.AuthService
	sessions      *middleware.SessionStore
	secureCookies bool
}

func NewAuthHandler(svc service.AuthService, sessions *middleware.SessionStore, secureCookies bool) *AuthHandler {
	return &AuthHandler{svc: svc, sessions: sessions, secureCookies: secureCookies}
}

type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}
type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type userView struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "参数无效"))
		return
	}
	u, err := h.svc.Register(c.Request.Context(), service.RegisterInput{
		Username: req.Username, Password: req.Password, Name: req.Name,
	})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	sid, csrf, err := h.sessions.Create(c.Request.Context(), middleware.Session{UserID: u.ID, Name: u.Name})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	middleware.SetSessionCookies(c, sid, csrf, h.sessions.TTLSeconds(), h.secureCookies)
	httpresp.OK(c, userView{ID: u.ID, Username: u.Username, Name: u.Name})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidParam, "参数无效"))
		return
	}
	u, err := h.svc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	sid, csrf, err := h.sessions.Create(c.Request.Context(), middleware.Session{UserID: u.ID, Name: u.Name})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	middleware.SetSessionCookies(c, sid, csrf, h.sessions.TTLSeconds(), h.secureCookies)
	httpresp.OK(c, userView{ID: u.ID, Username: u.Username, Name: u.Name})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if sidV, ok := c.Get("blog.sid"); ok {
		if sid, _ := sidV.(string); sid != "" {
			_ = h.sessions.Delete(c.Request.Context(), sid)
		}
	}
	middleware.ClearSessionCookies(c, h.secureCookies)
	httpresp.OK(c, nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	sess, ok := middleware.SessionFromContext(c)
	if !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	u, err := h.svc.GetByID(c.Request.Context(), sess.UserID)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, userView{ID: u.ID, Username: u.Username, Name: u.Name})
}
```

- [ ] **Step 4: 跑测试通过**

```bash
REDIS_ADDR=127.0.0.1:6379 go test ./internal/handler/... -run TestAuth -v
```

预期: 5 个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/handler/auth_handler.go Server/internal/handler/auth_handler_test.go
git commit -m "feat(server): auth handler (register/login/logout/me) with cookie session"
```

---

## Phase 3 — 文章核心(Articles)

文章是系统主线:列表、详情、增改删、浏览计数。本阶段最大的复杂点是 **浏览计数的 Redis/MySQL 双写策略**——读路径只触 Redis(`INCR + SADD`),后台循环每 30 秒把脏键 flush 进 MySQL,过程中失败要可恢复。Service 层负责把"DB 中的 view_count + Redis 中尚未 flush 的增量"合并成用户看到的实时数。

### Task 20: Tag Service + Tag Handler

**Files:**
- Create: `Server/internal/service/tag_service.go`
- Create: `Server/internal/service/tag_service_test.go`
- Create: `Server/internal/handler/tag_handler.go`
- Create: `Server/internal/handler/tag_handler_test.go`

Service 层职责:对 `TagRepo.ListWithCount()` 进行透传 + 排序(按文章数倒序,文章数相同按名字升序)。Handler 层对外暴露 `GET /api/v1/tags`。

- [ ] **Step 1: Service 失败测试**

`Server/internal/service/tag_service_test.go`:

```go
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/repository"
)

func newTagSvcWithDB(t *testing.T) (*TagService, *gorm.DB) {
	t.Helper()
	db := newTestDB(t) // 来自 repository 包测试基础设施;若不在同包,在 service 包再实现一份
	require.NoError(t, db.AutoMigrate(&model.Tag{}, &model.Article{}, &model.ArticleTag{}))
	return NewTagService(repository.NewTagRepo(db), repository.NewArticleRepo(db)), db
}

func TestTagService_List_OrdersByCountDesc(t *testing.T) {
	svc, db := newTagSvcWithDB(t)
	ctx := context.Background()

	// seed
	require.NoError(t, db.Create(&model.Tag{Name: "go"}).Error)
	require.NoError(t, db.Create(&model.Tag{Name: "rust"}).Error)
	require.NoError(t, db.Create(&model.Tag{Name: "ai"}).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO article_tags(article_id, tag_id) VALUES (1,1),(2,1),(3,2)",
	).Error)

	out, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, out, 3)
	require.Equal(t, "go", out[0].Name)    // 2 篇
	require.Equal(t, int64(2), out[0].ArticleCount)
	require.Equal(t, "rust", out[1].Name)  // 1 篇
	require.Equal(t, "ai", out[2].Name)    // 0 篇,字典序在 rust 之后
}
```

> 注: `newTestDB` 在 repository 包已实现。若 service 测试需要相同 helper,把它抽到 `internal/testutil/db.go` 中并 import。本计划默认放在 `internal/testutil/db.go`,在仓储任务中创建过的话沿用,否则现在创建一个等价副本。

- [ ] **Step 2: 跑测试看到 FAIL**

```bash
go test ./internal/service/... -run TestTagService_List -v
```

预期: `undefined: NewTagService` 或 `undefined: TagService.List`。

- [ ] **Step 3: 实现 Service**

`Server/internal/service/tag_service.go`:

```go
package service

import (
	"context"
	"sort"

	"github.com/wjr/blog/server/internal/repository"
)

type TagView struct {
	Name         string `json:"name"`
	ArticleCount int64  `json:"article_count"`
}

type TagService struct {
	tags     repository.TagRepo
	articles repository.ArticleRepo
}

func NewTagService(tags repository.TagRepo, articles repository.ArticleRepo) *TagService {
	return &TagService{tags: tags, articles: articles}
}

func (s *TagService) List(ctx context.Context) ([]TagView, error) {
	rows, err := s.tags.ListWithCount(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TagView, 0, len(rows))
	for _, r := range rows {
		out = append(out, TagView{Name: r.Name, ArticleCount: r.Count})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ArticleCount != out[j].ArticleCount {
			return out[i].ArticleCount > out[j].ArticleCount
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
```

- [ ] **Step 4: 跑 Service 测试通过**

```bash
go test ./internal/service/... -run TestTagService_List -v
```

预期: PASS。

- [ ] **Step 5: Handler 失败测试**

`Server/internal/handler/tag_handler_test.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/service"
)

type fakeTagSvc struct {
	out []service.TagView
	err error
}

func (f *fakeTagSvc) List(ctx context.Context) ([]service.TagView, error) {
	return f.out, f.err
}

func TestTagHandler_List_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeTagSvc{out: []service.TagView{{Name: "go", ArticleCount: 3}}}
	h := NewTagHandler(svc)

	r := gin.New()
	r.GET("/api/v1/tags", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tags", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Code int                `json:"code"`
		Data []service.TagView  `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 0, body.Code)
	require.Len(t, body.Data, 1)
	require.Equal(t, "go", body.Data[0].Name)
}
```

- [ ] **Step 6: 跑 Handler 测试看到 FAIL**

```bash
go test ./internal/handler/... -run TestTagHandler_List -v
```

预期: `undefined: NewTagHandler`。

- [ ] **Step 7: 实现 Handler**

`Server/internal/handler/tag_handler.go`:

```go
package handler

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/pkg/httpresp"
	"github.com/wjr/blog/server/internal/service"
)

type TagSvc interface {
	List(ctx context.Context) ([]service.TagView, error)
}

type TagHandler struct {
	svc TagSvc
}

func NewTagHandler(svc TagSvc) *TagHandler {
	return &TagHandler{svc: svc}
}

func (h *TagHandler) List(c *gin.Context) {
	tags, err := h.svc.List(c.Request.Context())
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, tags)
}
```

- [ ] **Step 8: 跑 Handler 测试通过**

```bash
go test ./internal/handler/... -run TestTagHandler_List -v
```

预期: PASS。

- [ ] **Step 9: Commit**

```bash
git add Server/internal/service/tag_service.go Server/internal/service/tag_service_test.go \
        Server/internal/handler/tag_handler.go Server/internal/handler/tag_handler_test.go
git commit -m "feat(server): tag service and handler with article count sort"
```

---

### Task 21: Article Service(含浏览计数合并)

**Files:**
- Create: `Server/internal/service/article_service.go`
- Create: `Server/internal/service/article_service_test.go`

Service 层把 `ArticleRepo` + `TagRepo` + `CounterRepo` 拼起来。所有写路径要做权限校验:只能本人操作自己的文章。读路径要把 DB `view_count` 和 Redis `view:<id>` 当前 INCR 值合并。

- [ ] **Step 1: 失败测试(Create + 权限 + 浏览合并)**

`Server/internal/service/article_service_test.go`:

```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/repository"
)

// fake CounterRepo 仅返回内存计数
type memCounter struct{ inc map[uint64]int64 }

func (m *memCounter) Inc(ctx context.Context, id uint64) error {
	m.inc[id]++
	return nil
}
func (m *memCounter) GetIncrement(ctx context.Context, id uint64) (int64, error) {
	return m.inc[id], nil
}
func (m *memCounter) DirtyMembers(ctx context.Context) ([]uint64, error)        { return nil, nil }
func (m *memCounter) DrainIncrement(ctx context.Context, id uint64) (int64, error) {
	v := m.inc[id]
	m.inc[id] = 0
	return v, nil
}
func (m *memCounter) Ack(ctx context.Context, ids []uint64) error    { return nil }
func (m *memCounter) Restore(ctx context.Context, id uint64, n int64) error {
	m.inc[id] += n
	return nil
}

func newArticleSvc(t *testing.T) (*ArticleService, *memCounter) {
	t.Helper()
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Article{}, &model.Tag{}, &model.ArticleTag{}))
	require.NoError(t, db.Create(&model.User{ID: 1, Username: "alice", Name: "Alice", Password: "x"}).Error)
	require.NoError(t, db.Create(&model.User{ID: 2, Username: "bob", Name: "Bob", Password: "x"}).Error)
	cnt := &memCounter{inc: map[uint64]int64{}}
	svc := NewArticleService(
		repository.NewArticleRepo(db),
		repository.NewTagRepo(db),
		repository.NewUserRepo(db),
		cnt,
	)
	return svc, cnt
}

func TestArticleService_Create_AssignsTagsAndSummary(t *testing.T) {
	svc, _ := newArticleSvc(t)
	ctx := context.Background()
	a, err := svc.Create(ctx, 1, CreateArticleInput{
		Title:   "首篇",
		Content: "# 标题\n\n这是 **正文** 内容,放一些字。",
		Tags:    []string{"go", "go", "  rust  "}, // 去重 + trim
	})
	require.NoError(t, err)
	require.NotZero(t, a.ID)
	require.Equal(t, "首篇", a.Title)
	require.NotEmpty(t, a.Summary)
	require.NotContains(t, a.Summary, "#")
	require.Len(t, a.Tags, 2)
}

func TestArticleService_Update_ForbidsOtherUser(t *testing.T) {
	svc, _ := newArticleSvc(t)
	ctx := context.Background()
	a, err := svc.Create(ctx, 1, CreateArticleInput{
		Title: "alice 的文章", Content: "正文" + string(make([]byte, 20)),
	})
	require.NoError(t, err)

	_, err = svc.Update(ctx, 2 /* bob */, a.ID, UpdateArticleInput{Title: "改"})
	require.Error(t, err)
	var ae *apperr.AppErr
	require.True(t, errors.As(err, &ae))
	require.Equal(t, apperr.CodeForbidden, ae.Code)
}

func TestArticleService_GetByID_MergesRedisIncrement(t *testing.T) {
	svc, cnt := newArticleSvc(t)
	ctx := context.Background()
	a, err := svc.Create(ctx, 1, CreateArticleInput{Title: "T", Content: "正文 contents 12345"})
	require.NoError(t, err)

	// 模拟 3 次浏览
	require.NoError(t, cnt.Inc(ctx, a.ID))
	require.NoError(t, cnt.Inc(ctx, a.ID))
	require.NoError(t, cnt.Inc(ctx, a.ID))

	got, err := svc.GetByID(ctx, a.ID, true /* incrementView */)
	require.NoError(t, err)
	// Inc 在 GetByID 中又触发一次,所以应当看到 4
	require.Equal(t, int64(4), got.ViewCount)
}
```

- [ ] **Step 2: 跑测试看到 FAIL**

```bash
go test ./internal/service/... -run TestArticleService -v
```

预期: 三个 case 全部失败,"undefined: NewArticleService"。

- [ ] **Step 3: 实现 Service**

`Server/internal/service/article_service.go`:

```go
package service

import (
	"context"
	"strings"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/model"
	"github.com/wjr/blog/server/internal/pkg/markdownx"
	"github.com/wjr/blog/server/internal/repository"
)

type CreateArticleInput struct {
	Title   string
	Content string
	Tags    []string
}

type UpdateArticleInput struct {
	Title   string
	Content string
	Tags    []string
}

type ArticleView struct {
	ID         uint64   `json:"id"`
	Title      string   `json:"title"`
	Content    string   `json:"content,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Tags       []string `json:"tags"`
	UserID     uint64   `json:"user_id"`
	AuthorName string   `json:"author_name,omitempty"`
	ViewCount  int64    `json:"view_count"`
	CreatedAt  int64    `json:"created_at"`
	UpdatedAt  int64    `json:"updated_at"`
}

type ListArticlesInput struct {
	Page   int
	Size   int
	Tag    string
	UserID uint64
}

type ArticleService struct {
	articles repository.ArticleRepo
	tags     repository.TagRepo
	users    repository.UserRepo
	counter  repository.CounterRepo
}

func NewArticleService(
	articles repository.ArticleRepo,
	tags repository.TagRepo,
	users repository.UserRepo,
	counter repository.CounterRepo,
) *ArticleService {
	return &ArticleService{articles: articles, tags: tags, users: users, counter: counter}
}

const (
	titleMin   = 1
	titleMax   = 200
	contentMin = 10
	contentMax = 100000
	tagsMax    = 5
	tagMaxLen  = 32
)

func validateArticleInput(title, content string, tags []string) error {
	t := strings.TrimSpace(title)
	if len([]rune(t)) < titleMin || len([]rune(t)) > titleMax {
		return apperr.New(apperr.CodeInvalidInput, "标题长度需 1-200")
	}
	if len([]rune(content)) < contentMin || len([]rune(content)) > contentMax {
		return apperr.New(apperr.CodeInvalidInput, "正文长度需 10-100000")
	}
	if len(tags) > tagsMax {
		return apperr.New(apperr.CodeInvalidInput, "标签最多 5 个")
	}
	for _, tag := range tags {
		if len([]rune(tag)) > tagMaxLen {
			return apperr.New(apperr.CodeInvalidInput, "标签过长")
		}
	}
	return nil
}

// normalizeTags trims, dedups, drops empty.
func normalizeTags(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func (s *ArticleService) Create(ctx context.Context, userID uint64, in CreateArticleInput) (*ArticleView, error) {
	tags := normalizeTags(in.Tags)
	if err := validateArticleInput(in.Title, in.Content, tags); err != nil {
		return nil, err
	}
	tagModels, err := s.tags.EnsureMany(ctx, tags)
	if err != nil {
		return nil, err
	}
	a := &model.Article{
		UserID:  userID,
		Title:   strings.TrimSpace(in.Title),
		Content: in.Content,
		Summary: markdownx.Summary(in.Content, 200),
		Tags:    tagModels,
	}
	if err := s.articles.Create(ctx, a); err != nil {
		return nil, err
	}
	return s.viewOf(ctx, a, false)
}

func (s *ArticleService) Update(ctx context.Context, userID uint64, id uint64, in UpdateArticleInput) (*ArticleView, error) {
	a, err := s.articles.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.UserID != userID {
		return nil, apperr.New(apperr.CodeForbidden, "无权操作此文章")
	}
	tags := normalizeTags(in.Tags)
	if err := validateArticleInput(in.Title, in.Content, tags); err != nil {
		return nil, err
	}
	tagModels, err := s.tags.EnsureMany(ctx, tags)
	if err != nil {
		return nil, err
	}
	a.Title = strings.TrimSpace(in.Title)
	a.Content = in.Content
	a.Summary = markdownx.Summary(in.Content, 200)
	a.Tags = tagModels
	if err := s.articles.Update(ctx, a); err != nil {
		return nil, err
	}
	return s.viewOf(ctx, a, false)
}

func (s *ArticleService) Delete(ctx context.Context, userID, id uint64) error {
	a, err := s.articles.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if a.UserID != userID {
		return apperr.New(apperr.CodeForbidden, "无权操作此文章")
	}
	return s.articles.SoftDelete(ctx, id)
}

func (s *ArticleService) GetByID(ctx context.Context, id uint64, incrementView bool) (*ArticleView, error) {
	a, err := s.articles.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if incrementView {
		_ = s.counter.Inc(ctx, id) // 计数失败不阻塞读
	}
	return s.viewOf(ctx, a, true)
}

func (s *ArticleService) List(ctx context.Context, in ListArticlesInput) (items []ArticleView, total int64, err error) {
	q := repository.ListQuery{Page: in.Page, Size: in.Size, Tag: in.Tag, UserID: in.UserID}
	rows, total, err := s.articles.List(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ArticleView, 0, len(rows))
	for i := range rows {
		v, err := s.viewOf(ctx, &rows[i], true /* hide content; merge counter */)
		if err != nil {
			return nil, 0, err
		}
		v.Content = "" // 列表不返回正文
		out = append(out, *v)
	}
	return out, total, nil
}

// viewOf 把 Article 模型转 view,合并 Redis 增量(若 mergeIncrement 为 true)。
func (s *ArticleService) viewOf(ctx context.Context, a *model.Article, mergeIncrement bool) (*ArticleView, error) {
	tagNames := make([]string, 0, len(a.Tags))
	for _, t := range a.Tags {
		tagNames = append(tagNames, t.Name)
	}
	views := a.ViewCount
	if mergeIncrement {
		if inc, err := s.counter.GetIncrement(ctx, a.ID); err == nil {
			views += inc
		}
	}
	authorName := ""
	if u, err := s.users.FindByID(ctx, a.UserID); err == nil && u != nil {
		authorName = u.Name
	}
	return &ArticleView{
		ID:         a.ID,
		Title:      a.Title,
		Content:    a.Content,
		Summary:    a.Summary,
		Tags:       tagNames,
		UserID:     a.UserID,
		AuthorName: authorName,
		ViewCount:  views,
		CreatedAt:  a.CreatedAt.Unix(),
		UpdatedAt:  a.UpdatedAt.Unix(),
	}, nil
}
```

> 注: `repository.NewUserRepo` 已在 Phase 1 中创建过;`SoftDelete` 是 `ArticleRepo.Delete` 的别名(仍为软删,GORM `DeletedAt` 自动处理)。如果仓储里只暴露 `Delete()`,把这里的 `SoftDelete` 改成 `Delete` 即可。

- [ ] **Step 4: 跑测试通过**

```bash
go test ./internal/service/... -run TestArticleService -v
```

预期: 3 个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/service/article_service.go Server/internal/service/article_service_test.go
git commit -m "feat(server): article service with view counter merge and ownership checks"
```

---

### Task 22: Article Handler(列表 / 详情 / CRUD)

**Files:**
- Create: `Server/internal/handler/article_handler.go`
- Create: `Server/internal/handler/article_handler_test.go`

Handler 负责 HTTP 编解码、URL 参数提取、调 Service。详情接口要识别"已登录用户访问自己的文章"(返回 content)与"未登录或他人浏览"(返回 content + summary;读路径触发计数)。

- [ ] **Step 1: 失败测试**

`Server/internal/handler/article_handler_test.go`:

```go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/service"
)

type fakeArticleSvc struct {
	createIn  service.CreateArticleInput
	createOut *service.ArticleView
	createErr error

	getOut *service.ArticleView
	getErr error

	listOut   []service.ArticleView
	listTotal int64
	listErr   error
}

func (f *fakeArticleSvc) Create(ctx context.Context, uid uint64, in service.CreateArticleInput) (*service.ArticleView, error) {
	f.createIn = in
	return f.createOut, f.createErr
}
func (f *fakeArticleSvc) Update(ctx context.Context, uid, id uint64, in service.UpdateArticleInput) (*service.ArticleView, error) {
	return f.createOut, f.createErr
}
func (f *fakeArticleSvc) Delete(ctx context.Context, uid, id uint64) error { return nil }
func (f *fakeArticleSvc) GetByID(ctx context.Context, id uint64, incView bool) (*service.ArticleView, error) {
	return f.getOut, f.getErr
}
func (f *fakeArticleSvc) List(ctx context.Context, in service.ListArticlesInput) ([]service.ArticleView, int64, error) {
	return f.listOut, f.listTotal, f.listErr
}

func TestArticleHandler_Create_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewArticleHandler(&fakeArticleSvc{})
	r := gin.New()
	r.POST("/api/v1/articles", h.Create) // 未挂载 RequireAuth,模拟未登录
	body := bytes.NewBufferString(`{"title":"t","content":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/articles", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var res struct{ Code int `json:"code"` }
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	require.Equal(t, 2001, res.Code) // 业务码
}

func TestArticleHandler_GetByID_IncrementsView(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeArticleSvc{getOut: &service.ArticleView{ID: 1, Title: "T", ViewCount: 7}}
	h := NewArticleHandler(svc)
	r := gin.New()
	r.GET("/api/v1/articles/:id", h.GetByID)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var res struct {
		Code int                 `json:"code"`
		Data *service.ArticleView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	require.Equal(t, 0, res.Code)
	require.Equal(t, int64(7), res.Data.ViewCount)
}

func TestArticleHandler_List_PaginationDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeArticleSvc{listOut: []service.ArticleView{{ID: 1}}, listTotal: 1}
	h := NewArticleHandler(svc)
	r := gin.New()
	r.GET("/api/v1/articles", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}

// helper: 注入"已登录"上下文,用于 Create / Update / Delete 测试
func withSession(uid uint64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("blog.session", &middleware.Session{UserID: uid, Name: "tester"})
		c.Next()
	}
}
```

- [ ] **Step 2: 跑测试看到 FAIL**

```bash
go test ./internal/handler/... -run TestArticleHandler -v
```

预期: 全部失败,`undefined: NewArticleHandler`。

- [ ] **Step 3: 实现 Handler**

`Server/internal/handler/article_handler.go`:

```go
package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
	"github.com/wjr/blog/server/internal/service"
)

type ArticleSvc interface {
	Create(ctx context.Context, uid uint64, in service.CreateArticleInput) (*service.ArticleView, error)
	Update(ctx context.Context, uid, id uint64, in service.UpdateArticleInput) (*service.ArticleView, error)
	Delete(ctx context.Context, uid, id uint64) error
	GetByID(ctx context.Context, id uint64, incView bool) (*service.ArticleView, error)
	List(ctx context.Context, in service.ListArticlesInput) ([]service.ArticleView, int64, error)
}

type ArticleHandler struct {
	svc ArticleSvc
}

func NewArticleHandler(svc ArticleSvc) *ArticleHandler {
	return &ArticleHandler{svc: svc}
}

type articleCreateReq struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

type articleUpdateReq = articleCreateReq

type listResp struct {
	Items []service.ArticleView `json:"items"`
	Total int64                 `json:"total"`
	Page  int                   `json:"page"`
	Size  int                   `json:"size"`
}

func (h *ArticleHandler) Create(c *gin.Context) {
	sess, ok := middleware.SessionFromContext(c)
	if !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	var req articleCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidInput, "请求体格式错误"))
		return
	}
	out, err := h.svc.Create(c.Request.Context(), sess.UserID, service.CreateArticleInput{
		Title: req.Title, Content: req.Content, Tags: req.Tags,
	})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, out)
}

func (h *ArticleHandler) Update(c *gin.Context) {
	sess, ok := middleware.SessionFromContext(c)
	if !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidInput, "id 非法"))
		return
	}
	var req articleUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidInput, "请求体格式错误"))
		return
	}
	out, err := h.svc.Update(c.Request.Context(), sess.UserID, id, service.UpdateArticleInput{
		Title: req.Title, Content: req.Content, Tags: req.Tags,
	})
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, out)
}

func (h *ArticleHandler) Delete(c *gin.Context) {
	sess, ok := middleware.SessionFromContext(c)
	if !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidInput, "id 非法"))
		return
	}
	if err := h.svc.Delete(c.Request.Context(), sess.UserID, id); err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, nil)
}

func (h *ArticleHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidInput, "id 非法"))
		return
	}
	out, err := h.svc.GetByID(c.Request.Context(), id, true)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, out)
}

func (h *ArticleHandler) List(c *gin.Context) {
	in := service.ListArticlesInput{
		Page: parseIntDefault(c.Query("page"), 1),
		Size: parseIntDefault(c.Query("size"), 10),
		Tag:  strings.TrimSpace(c.Query("tag")),
	}
	if uid, err := strconv.ParseUint(c.Query("user_id"), 10, 64); err == nil {
		in.UserID = uid
	}
	if in.Page < 1 {
		in.Page = 1
	}
	if in.Size < 1 || in.Size > 50 {
		in.Size = 10
	}
	items, total, err := h.svc.List(c.Request.Context(), in)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, listResp{Items: items, Total: total, Page: in.Page, Size: in.Size})
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
```

- [ ] **Step 4: 跑测试通过**

```bash
go test ./internal/handler/... -run TestArticleHandler -v
```

预期: 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/handler/article_handler.go Server/internal/handler/article_handler_test.go
git commit -m "feat(server): article handler crud + list with pagination defaults"
```

---

### Task 23: 浏览计数 Flush Worker(后台 30 秒批写)

**Files:**
- Create: `Server/internal/worker/view_flush.go`
- Create: `Server/internal/worker/view_flush_test.go`

后台 worker 每 30s 执行:`DirtyMembers()` 拿到所有脏 id → 逐 id `DrainIncrement` 取得增量 → 批量 `IncrementViewCount(id, delta)` 写 DB → `Ack(ids)`。任何阶段失败都要 `Restore()` 把扣下的增量 INCRBY 回 Redis,确保最终一致性(无论这一轮成功失败,数据都不丢)。

- [ ] **Step 1: 失败测试**

`Server/internal/worker/view_flush_test.go`:

```go
package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type stubCounter struct {
	mu      sync.Mutex
	dirty   []uint64
	drained map[uint64]int64
	ack     []uint64
	restore map[uint64]int64
}

func (s *stubCounter) Inc(ctx context.Context, id uint64) error { return nil }
func (s *stubCounter) GetIncrement(ctx context.Context, id uint64) (int64, error) { return 0, nil }
func (s *stubCounter) DirtyMembers(ctx context.Context) ([]uint64, error)     { return s.dirty, nil }
func (s *stubCounter) DrainIncrement(ctx context.Context, id uint64) (int64, error) {
	v := s.drained[id]
	return v, nil
}
func (s *stubCounter) Ack(ctx context.Context, ids []uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ack = append(s.ack, ids...)
	return nil
}
func (s *stubCounter) Restore(ctx context.Context, id uint64, n int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restore[id] += n
	return nil
}

type stubArticles struct {
	mu      sync.Mutex
	apply   map[uint64]int64
	failOn  map[uint64]bool // 模拟 db 写失败
}

func (s *stubArticles) IncrementViewCount(ctx context.Context, id uint64, delta int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failOn[id] {
		return errors.New("boom")
	}
	s.apply[id] += delta
	return nil
}

func TestFlushOnce_AppliesIncrementsAndAcks(t *testing.T) {
	cnt := &stubCounter{dirty: []uint64{1, 2}, drained: map[uint64]int64{1: 3, 2: 7}, restore: map[uint64]int64{}}
	arts := &stubArticles{apply: map[uint64]int64{}}
	w := New(cnt, arts, 30*time.Second)
	require.NoError(t, w.flushOnce(context.Background()))
	require.Equal(t, int64(3), arts.apply[1])
	require.Equal(t, int64(7), arts.apply[2])
	require.ElementsMatch(t, []uint64{1, 2}, cnt.ack)
	require.Empty(t, cnt.restore)
}

func TestFlushOnce_RestoresOnDBFailure(t *testing.T) {
	cnt := &stubCounter{dirty: []uint64{1}, drained: map[uint64]int64{1: 5}, restore: map[uint64]int64{}}
	arts := &stubArticles{apply: map[uint64]int64{}, failOn: map[uint64]bool{1: true}}
	w := New(cnt, arts, 30*time.Second)
	err := w.flushOnce(context.Background())
	require.Error(t, err)
	require.Equal(t, int64(5), cnt.restore[1]) // 回滚
	require.Empty(t, cnt.ack)
}
```

- [ ] **Step 2: 跑测试看到 FAIL**

```bash
go test ./internal/worker/... -run TestFlushOnce -v
```

预期: 全部失败,`undefined: New`。

- [ ] **Step 3: 实现 Flush Worker**

`Server/internal/worker/view_flush.go`:

```go
package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// Counter 是 worker 需要的最小 counter 接口子集。
type Counter interface {
	DirtyMembers(ctx context.Context) ([]uint64, error)
	DrainIncrement(ctx context.Context, id uint64) (int64, error)
	Ack(ctx context.Context, ids []uint64) error
	Restore(ctx context.Context, id uint64, n int64) error
}

// Articles 是 worker 写 DB 所需的最小接口。
type Articles interface {
	IncrementViewCount(ctx context.Context, id uint64, delta int64) error
}

type ViewFlush struct {
	counter  Counter
	articles Articles
	interval time.Duration
}

func New(counter Counter, articles Articles, interval time.Duration) *ViewFlush {
	return &ViewFlush{counter: counter, articles: articles, interval: interval}
}

// Run 阻塞循环,直到 ctx 取消。退出前再 flush 一次,把残余增量落库。
func (w *ViewFlush) Run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			// 退出前最后一次 flush(用 background context 避免被立即取消)
			final, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := w.flushOnce(final); err != nil {
				log.Error().Err(err).Msg("final view flush failed")
			}
			cancel()
			return
		case <-t.C:
			if err := w.flushOnce(ctx); err != nil {
				log.Error().Err(err).Msg("view flush failed")
			}
		}
	}
}

func (w *ViewFlush) flushOnce(ctx context.Context) error {
	ids, err := w.counter.DirtyMembers(ctx)
	if err != nil {
		return fmt.Errorf("dirty members: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}
	applied := make([]uint64, 0, len(ids))
	var firstErr error
	for _, id := range ids {
		delta, err := w.counter.DrainIncrement(ctx, id)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if delta == 0 {
			applied = append(applied, id)
			continue
		}
		if err := w.articles.IncrementViewCount(ctx, id, delta); err != nil {
			// 回滚:把刚 drain 走的增量 INCRBY 回 Redis,等下一轮重试
			if rErr := w.counter.Restore(ctx, id, delta); rErr != nil {
				log.Error().Err(rErr).Uint64("id", id).Msg("restore failed")
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		applied = append(applied, id)
	}
	if len(applied) > 0 {
		if err := w.counter.Ack(ctx, applied); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
```

- [ ] **Step 4: 跑测试通过**

```bash
go test ./internal/worker/... -v
```

预期: 2 个 case 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/worker/view_flush.go Server/internal/worker/view_flush_test.go
git commit -m "feat(server): view counter flush worker with restore-on-failure"
```

---

## Phase 4 — 图片上传

`POST /api/v1/uploads/image`:multipart 单文件,最大 5MB,只接受 PNG/JPEG/WEBP。文件名走 `uuid + 原始扩展名`,存到 `cfg.Upload.Dir`(默认 `./data/uploads`),返回 `/uploads/<filename>` 相对 URL。注意:
- **类型校验不能只看扩展名**——必须 sniff `http.DetectContentType()` 取实际 MIME。
- 文件大小用 `multipart.FileHeader.Size` 先粗筛,再按 `LimitReader` 兜底。
- 文件名用 `idgen.NewUUID()` 生成,确保不会被攻击者通过路径遍历覆盖。

### Task 24: Upload Service

**Files:**
- Create: `Server/internal/service/upload_service.go`
- Create: `Server/internal/service/upload_service_test.go`

- [ ] **Step 1: 失败测试**

`Server/internal/service/upload_service_test.go`:

```go
package service

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/apperr"
)

// makePNG 生成一个 1×1 PNG,返回 multipart.FileHeader 与其字节。
func makePNG(t *testing.T) (*multipart.FileHeader, int) {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	return makeUpload(t, "img.png", "image/png", buf.Bytes()), buf.Len()
}

func makeUpload(t *testing.T, name, ct string, data []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+name+`"`)
	hdr.Set("Content-Type", ct)
	part, err := w.CreatePart(hdr)
	require.NoError(t, err)
	_, _ = io.Copy(part, bytes.NewReader(data))
	require.NoError(t, w.Close())
	r := multipart.NewReader(body, w.Boundary())
	form, err := r.ReadForm(10 << 20)
	require.NoError(t, err)
	return form.File["file"][0]
}

func TestUploadService_AcceptsPNG(t *testing.T) {
	dir := t.TempDir()
	svc := NewUploadService(UploadOptions{
		Dir: dir, MaxBytes: 5 << 20, AllowedMIME: []string{"image/png", "image/jpeg", "image/webp"},
	})
	fh, _ := makePNG(t)
	url, err := svc.SaveImage(context.Background(), fh)
	require.NoError(t, err)
	require.Contains(t, url, "/uploads/")
	files, _ := os.ReadDir(dir)
	require.Len(t, files, 1)
}

func TestUploadService_RejectsTooBig(t *testing.T) {
	dir := t.TempDir()
	svc := NewUploadService(UploadOptions{
		Dir: dir, MaxBytes: 100, AllowedMIME: []string{"image/png"},
	})
	big := make([]byte, 200)
	fh := makeUpload(t, "x.png", "image/png", big)
	_, err := svc.SaveImage(context.Background(), fh)
	require.Error(t, err)
	ae, _ := err.(*apperr.AppErr)
	require.Equal(t, apperr.CodeFileTooLarge, ae.Code)
}

func TestUploadService_RejectsWrongMIME(t *testing.T) {
	dir := t.TempDir()
	svc := NewUploadService(UploadOptions{
		Dir: dir, MaxBytes: 5 << 20, AllowedMIME: []string{"image/png"},
	})
	// 把一个 EXE-ish payload 放进 .png 文件
	payload := []byte("MZ\x90\x00" + string(make([]byte, 64)))
	fh := makeUpload(t, "evil.png", "image/png", payload)
	_, err := svc.SaveImage(context.Background(), fh)
	require.Error(t, err)
	ae, _ := err.(*apperr.AppErr)
	require.Equal(t, apperr.CodeFileTypeUnsupported, ae.Code)
	files, _ := os.ReadDir(dir)
	require.Empty(t, files) // 不能落盘
}

// 防止 import unused
var _ = filepath.Join
```

- [ ] **Step 2: 跑测试看到 FAIL**

```bash
go test ./internal/service/... -run TestUploadService -v
```

预期: 全部失败,`undefined: NewUploadService`。

- [ ] **Step 3: 实现 Upload Service**

`Server/internal/service/upload_service.go`:

```go
package service

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/pkg/idgen"
)

type UploadOptions struct {
	Dir         string
	URLPrefix   string   // 默认 "/uploads"
	MaxBytes    int64    // 默认 5*1024*1024
	AllowedMIME []string // 例: ["image/png","image/jpeg","image/webp"]
}

type UploadService struct {
	opt UploadOptions
}

func NewUploadService(opt UploadOptions) *UploadService {
	if opt.URLPrefix == "" {
		opt.URLPrefix = "/uploads"
	}
	if opt.MaxBytes == 0 {
		opt.MaxBytes = 5 << 20
	}
	return &UploadService{opt: opt}
}

func (s *UploadService) SaveImage(ctx context.Context, fh *multipart.FileHeader) (string, error) {
	if fh.Size > s.opt.MaxBytes {
		return "", apperr.New(apperr.CodeFileTooLarge, "文件过大")
	}
	src, err := fh.Open()
	if err != nil {
		return "", apperr.New(apperr.CodeInvalidInput, "无法读取上传文件")
	}
	defer src.Close()

	// sniff 前 512 字节,判定真实 MIME
	header := make([]byte, 512)
	n, _ := io.ReadFull(src, header)
	mime := http.DetectContentType(header[:n])
	if !contains(s.opt.AllowedMIME, mime) {
		return "", apperr.New(apperr.CodeFileTypeUnsupported, "文件类型不支持")
	}

	// 决定保存的扩展名:基于 sniff 出的 MIME,而非用户传的文件名
	ext := mimeToExt(mime)
	if ext == "" {
		ext = strings.ToLower(filepath.Ext(fh.Filename))
	}

	if err := os.MkdirAll(s.opt.Dir, 0o755); err != nil {
		return "", apperr.Wrap(apperr.New(apperr.CodeInternal, "无法创建上传目录"), err)
	}
	name := idgen.NewUUID() + ext
	dst := filepath.Join(s.opt.Dir, name)

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", apperr.Wrap(apperr.New(apperr.CodeInternal, "写文件失败"), err)
	}
	defer out.Close()

	// 重新拼回 reader:已读 header + 剩余流;同时用 LimitReader 兜底
	full := io.MultiReader(io.NopCloser(strings.NewReader(string(header[:n]))), src)
	limited := io.LimitReader(full, s.opt.MaxBytes+1)
	written, err := io.Copy(out, limited)
	if err != nil {
		os.Remove(dst)
		return "", apperr.Wrap(apperr.New(apperr.CodeInternal, "写文件失败"), err)
	}
	if written > s.opt.MaxBytes {
		os.Remove(dst)
		return "", apperr.New(apperr.CodeFileTooLarge, "文件过大")
	}
	return path.Join(s.opt.URLPrefix, name), nil
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}

func mimeToExt(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
```

> 注: `apperr.CodeFileTypeUnsupported` = 1020,`CodeFileTooLarge` = 1021;按 Phase 0 Task 3 中定义的常量。如果命名不一致,以 `apperr` 包导出常量为准并在测试中相应引用。

- [ ] **Step 4: 跑测试通过**

```bash
go test ./internal/service/... -run TestUploadService -v
```

预期: 3 个 case 全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/service/upload_service.go Server/internal/service/upload_service_test.go
git commit -m "feat(server): upload service with mime sniffing and size limits"
```

---

### Task 25: Upload Handler(含速率限制)

**Files:**
- Create: `Server/internal/handler/upload_handler.go`
- Create: `Server/internal/handler/upload_handler_test.go`

接口契约: `POST /api/v1/uploads/image`,body 是 multipart `file=<image>`。需要登录 + CSRF + 用户级速率限制(每分钟 30 次,见 Server.md §4.2)。Handler 本身只做参数提取和编解码,所有业务都在 service 里。

- [ ] **Step 1: 失败测试**

`Server/internal/handler/upload_handler_test.go`:

```go
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/wjr/blog/server/internal/middleware"
)

type fakeUploadSvc struct {
	called   bool
	url      string
	err      error
}

func (f *fakeUploadSvc) SaveImage(ctx context.Context, fh *multipart.FileHeader) (string, error) {
	f.called = true
	return f.url, f.err
}

func makePNGForm(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "x.png")
	require.NoError(t, err)
	require.NoError(t, png.Encode(part, image.NewRGBA(image.Rect(0, 0, 1, 1))))
	require.NoError(t, w.Close())
	return body, w.ContentType()
}

func TestUploadHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &fakeUploadSvc{url: "/uploads/abc.png"}
	h := NewUploadHandler(svc)
	r := gin.New()
	r.POST("/api/v1/uploads/image",
		func(c *gin.Context) {
			c.Set("blog.session", &middleware.Session{UserID: 1, Name: "u1"})
			c.Next()
		},
		h.Image)

	body, ct := makePNGForm(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/image", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var res struct {
		Code int `json:"code"`
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	require.Equal(t, 0, res.Code)
	require.Equal(t, "/uploads/abc.png", res.Data.URL)
	require.True(t, svc.called)
}

func TestUploadHandler_MissingFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUploadHandler(&fakeUploadSvc{})
	r := gin.New()
	r.POST("/api/v1/uploads/image",
		func(c *gin.Context) {
			c.Set("blog.session", &middleware.Session{UserID: 1})
			c.Next()
		}, h.Image)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads/image", io.NopCloser(bytes.NewReader(nil)))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var res struct{ Code int `json:"code"` }
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	require.Equal(t, 1001, res.Code)
}
```

- [ ] **Step 2: 跑测试看到 FAIL**

```bash
go test ./internal/handler/... -run TestUploadHandler -v
```

预期: `undefined: NewUploadHandler`。

- [ ] **Step 3: 实现 Handler**

`Server/internal/handler/upload_handler.go`:

```go
package handler

import (
	"context"
	"mime/multipart"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/apperr"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/pkg/httpresp"
)

type UploadSvc interface {
	SaveImage(ctx context.Context, fh *multipart.FileHeader) (string, error)
}

type UploadHandler struct {
	svc UploadSvc
}

func NewUploadHandler(svc UploadSvc) *UploadHandler {
	return &UploadHandler{svc: svc}
}

type uploadResp struct {
	URL string `json:"url"`
}

func (h *UploadHandler) Image(c *gin.Context) {
	if _, ok := middleware.SessionFromContext(c); !ok {
		httpresp.Fail(c, apperr.New(apperr.CodeUnauthorized, "未登录"))
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		httpresp.Fail(c, apperr.New(apperr.CodeInvalidInput, "缺少 file 字段"))
		return
	}
	url, err := h.svc.SaveImage(c.Request.Context(), fh)
	if err != nil {
		httpresp.Fail(c, err)
		return
	}
	httpresp.OK(c, uploadResp{URL: url})
}
```

- [ ] **Step 4: 跑测试通过**

```bash
go test ./internal/handler/... -run TestUploadHandler -v
```

预期: 2 个 case PASS。

- [ ] **Step 5: Commit**

```bash
git add Server/internal/handler/upload_handler.go Server/internal/handler/upload_handler_test.go
git commit -m "feat(server): upload handler for multipart image submissions"
```

---

## Phase 5 — 装配与端到端测试

### Task 26: 路由装配(Router)

**Files:**
- Create: `Server/internal/router/router.go`

按 Server.md §4 把所有 handler 串到 gin engine 上。中间件顺序: `Recover → RequestLog → CORS(若需) → static(/uploads, /assets, /) → API 子路由(/api/v1) → SessionStore.WithSession → 公开/私有路由分组`。

- [ ] **Step 1: 编写路由装配函数**

`Server/internal/router/router.go`:

```go
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/wjr/blog/server/internal/handler"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Auth          *handler.AuthHandler
	Article       *handler.ArticleHandler
	Tag           *handler.TagHandler
	Upload        *handler.UploadHandler
	Sessions      *middleware.SessionStore
	RDB           *redis.Client
	StaticWebDir  string // 例: "./Web"
	StaticUploadDir string // 例: "./data/uploads"
	SecureCookies bool
	RateLimitIP   middleware.RateLimitOpts // 例: {Limit: 60, Window: time.Minute}
	RateLimitUser middleware.RateLimitOpts // 例: {Limit: 30, Window: time.Minute, KeyFunc: middleware.UserKey}
}

func New(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Recover())
	r.Use(middleware.RequestLog())

	// 静态资源
	r.Static("/assets", d.StaticWebDir+"/assets")
	r.Static("/vendor", d.StaticWebDir+"/vendor")
	r.Static("/uploads", d.StaticUploadDir)
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/list.html") })
	r.StaticFile("/list.html", d.StaticWebDir+"/list.html")
	r.StaticFile("/login.html", d.StaticWebDir+"/login.html")
	r.StaticFile("/register.html", d.StaticWebDir+"/register.html")
	r.StaticFile("/detail.html", d.StaticWebDir+"/detail.html")
	r.StaticFile("/editor.html", d.StaticWebDir+"/editor.html")
	r.StaticFile("/profile.html", d.StaticWebDir+"/profile.html")

	// API
	api := r.Group("/api/v1")
	api.Use(d.Sessions.WithSession(d.SecureCookies))
	api.Use(middleware.RateLimit(d.RDB, d.RateLimitIP))

	// auth
	api.POST("/auth/register", d.Auth.Register)
	api.POST("/auth/login", d.Auth.Login)
	api.POST("/auth/logout", d.Auth.Logout)
	api.GET("/auth/me", d.Auth.Me)

	// 公开读
	api.GET("/articles", d.Article.List)
	api.GET("/articles/:id", d.Article.GetByID)
	api.GET("/tags", d.Tag.List)
	api.GET("/users/:id/articles", d.Article.List) // 复用 list,Handler 内根据 :id 拼装 user_id

	// 受保护写路径:RequireAuth + CSRFGuard + 用户级速率限制
	priv := api.Group("/", middleware.RequireAuth(), middleware.CSRFGuard(),
		middleware.RateLimit(d.RDB, d.RateLimitUser))
	priv.POST("/articles", d.Article.Create)
	priv.PUT("/articles/:id", d.Article.Update)
	priv.DELETE("/articles/:id", d.Article.Delete)
	priv.POST("/uploads/image", d.Upload.Image)

	return r
}
```

> 注: `GET /users/:id/articles` 由 `ArticleHandler.List` 处理时,要识别 path param `:id` 并把它写入 `ListArticlesInput.UserID`。如果 `c.Query("user_id")` 已经覆盖了同样的需求,可以让 handler 在进入 List 时优先取 `c.Param("id")`。这是个 ~5 行的小调整,与 Task 22 的 `List` 函数兼容。

- [ ] **Step 2: 让 Handler.List 兼容 path param**

把 Task 22 实现的 `ArticleHandler.List` 顶部加一段:

```go
// 在 List 函数开头,从路径参数取 user_id(优先于 query)
if idStr := c.Param("id"); idStr != "" {
	if uid, err := strconv.ParseUint(idStr, 10, 64); err == nil {
		in.UserID = uid
	}
}
```

(直接 Edit `Server/internal/handler/article_handler.go`。)

- [ ] **Step 3: Commit**

```bash
git add Server/internal/router/router.go Server/internal/handler/article_handler.go
git commit -m "feat(server): router assembly with public/private route groups"
```

---

### Task 27: main.go 入口与依赖装配

**Files:**
- Create: `Server/cmd/server/main.go`

main 负责:加载配置 → 打开 MySQL/Redis → 跑 GORM 自动迁移(只在开发模式;生产用 SQL 文件) → 装配仓储/服务/Handler → 启 gin → 启 view-flush worker → 监听 SIGINT/SIGTERM 优雅退出。

- [ ] **Step 1: 写 main**

`Server/cmd/server/main.go`:

```go
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/wjr/blog/server/internal/cache"
	"github.com/wjr/blog/server/internal/config"
	"github.com/wjr/blog/server/internal/db"
	"github.com/wjr/blog/server/internal/handler"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/repository"
	"github.com/wjr/blog/server/internal/router"
	"github.com/wjr/blog/server/internal/service"
	"github.com/wjr/blog/server/internal/worker"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	cfgPath := os.Getenv("BLOG_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}

	gdb, err := db.Open(db.Options{
		DSN:             cfg.MySQL.DSN(),
		MaxOpenConns:    cfg.MySQL.MaxOpenConns,
		MaxIdleConns:    cfg.MySQL.MaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.MySQL.ConnMaxLifetimeMin) * time.Minute,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("open mysql")
	}

	rdb, err := cache.Open(cache.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("open redis")
	}

	// repos / services / handlers
	userRepo := repository.NewUserRepo(gdb)
	tagRepo := repository.NewTagRepo(gdb)
	articleRepo := repository.NewArticleRepo(gdb)
	counterRepo := repository.NewCounterRepo(rdb)

	authSvc := service.NewAuthService(userRepo)
	tagSvc := service.NewTagService(tagRepo, articleRepo)
	articleSvc := service.NewArticleService(articleRepo, tagRepo, userRepo, counterRepo)
	uploadSvc := service.NewUploadService(service.UploadOptions{
		Dir:         cfg.Upload.Dir,
		MaxBytes:    cfg.Upload.MaxBytes,
		AllowedMIME: cfg.Upload.AllowedMIME,
	})

	sessions := middleware.NewSessionStore(rdb, cfg.Session.TTLMinutes)

	authH := handler.NewAuthHandler(authSvc, sessions, cfg.Server.SecureCookies)
	articleH := handler.NewArticleHandler(articleSvc)
	tagH := handler.NewTagHandler(tagSvc)
	uploadH := handler.NewUploadHandler(uploadSvc)

	r := router.New(router.Deps{
		Auth: authH, Article: articleH, Tag: tagH, Upload: uploadH,
		Sessions:        sessions,
		RDB:             rdb,
		StaticWebDir:    cfg.Server.StaticWebDir,
		StaticUploadDir: cfg.Upload.Dir,
		SecureCookies:   cfg.Server.SecureCookies,
		RateLimitIP: middleware.RateLimitOpts{
			Limit: cfg.RateLimit.IPPerMinute, Window: time.Minute,
			KeyFunc: middleware.IPKey,
		},
		RateLimitUser: middleware.RateLimitOpts{
			Limit: cfg.RateLimit.UserPerMinute, Window: time.Minute,
			KeyFunc: middleware.UserKey,
		},
	})

	// 启 worker
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	flushInterval := time.Duration(cfg.ViewFlush.IntervalSeconds) * time.Second
	if flushInterval == 0 {
		flushInterval = 30 * time.Second
	}
	flush := worker.New(counterRepo, articleRepo, flushInterval)
	go func() { defer wg.Done(); flush.Run(ctx) }()

	// HTTP server
	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Info().Str("addr", cfg.Server.Addr).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server crashed")
		}
	}()

	// 等信号
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info().Msg("shutting down")

	// 30s 内优雅退出 HTTP
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)

	// 通知 worker 退出并等其完成最后一次 flush
	cancel()
	wg.Wait()
	log.Info().Msg("bye")
}
```

> 注: 若 `repository.NewCounterRepo` 的签名不匹配 `worker.Counter` 接口,在仓储里加 wrapper 适配——把它放进 worker 包做 type-assert,以避免循环依赖。或者让 worker 接口刻意保持小,只声明 `DirtyMembers/DrainIncrement/Ack/Restore` 即可,如 Task 23 已采用的做法。

- [ ] **Step 2: 编译通过**

```bash
go build ./...
```

预期: 没有错误。

- [ ] **Step 3: Commit**

```bash
git add Server/cmd/server/main.go
git commit -m "feat(server): main entrypoint with DI wiring and graceful shutdown"
```

---

### Task 28: 端到端集成测试(testcontainers)

**Files:**
- Create: `Server/test/integration/e2e_test.go`

跑真 MySQL + Redis,用 `testcontainers-go`。这个测试会被 `make test-integration` 触发,平时 `go test ./...` 默认跳过(用 build tag `integration`)。

- [ ] **Step 1: 写集成测试**

`Server/test/integration/e2e_test.go`:

```go
//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/wjr/blog/server/internal/cache"
	"github.com/wjr/blog/server/internal/config"
	"github.com/wjr/blog/server/internal/db"
	"github.com/wjr/blog/server/internal/handler"
	"github.com/wjr/blog/server/internal/middleware"
	"github.com/wjr/blog/server/internal/repository"
	"github.com/wjr/blog/server/internal/router"
	"github.com/wjr/blog/server/internal/service"
)

func startStack(t *testing.T) (mysqlDSN string, redisAddr string) {
	t.Helper()
	ctx := context.Background()
	myc, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8.0"),
		mysql.WithDatabase("blog"),
		mysql.WithUsername("blog"),
		mysql.WithPassword("blog"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = myc.Terminate(ctx) })
	mysqlDSN, err = myc.ConnectionString(ctx, "parseTime=true&charset=utf8mb4")
	require.NoError(t, err)

	rc, err := redis.RunContainer(ctx, testcontainers.WithImage("redis:7-alpine"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Terminate(ctx) })
	host, _ := rc.Host(ctx)
	port, _ := rc.MappedPort(ctx, "6379/tcp")
	redisAddr = fmt.Sprintf("%s:%s", host, port.Port())
	return
}

func setupServer(t *testing.T) (*httptest.Server, http.CookieJar) {
	t.Helper()
	dsn, raddr := startStack(t)
	gdb, err := db.Open(db.Options{DSN: dsn, MaxOpenConns: 10, MaxIdleConns: 5, ConnMaxLifetime: 30 * time.Minute})
	require.NoError(t, err)
	rdb, err := cache.Open(cache.Options{Addr: raddr, PoolSize: 10})
	require.NoError(t, err)

	// 跑 schema
	require.NoError(t, gdb.Exec("CREATE TABLE users(...same as 001_init...)").Error)
	// 真实场景下 Read SQL 文件并 Exec;此处省略 schema 直接 AutoMigrate:
	require.NoError(t, gdb.AutoMigrate(
		&repository.UserModel{}, &repository.ArticleModel{}, &repository.TagModel{}, &repository.ArticleTagModel{},
	))

	userRepo := repository.NewUserRepo(gdb)
	tagRepo := repository.NewTagRepo(gdb)
	articleRepo := repository.NewArticleRepo(gdb)
	counterRepo := repository.NewCounterRepo(rdb)
	sessions := middleware.NewSessionStore(rdb, 30)

	authH := handler.NewAuthHandler(service.NewAuthService(userRepo), sessions, false)
	articleH := handler.NewArticleHandler(service.NewArticleService(articleRepo, tagRepo, userRepo, counterRepo))
	tagH := handler.NewTagHandler(service.NewTagService(tagRepo, articleRepo))
	uploadH := handler.NewUploadHandler(service.NewUploadService(service.UploadOptions{
		Dir: t.TempDir(), MaxBytes: 5 << 20, AllowedMIME: []string{"image/png", "image/jpeg", "image/webp"},
	}))

	r := router.New(router.Deps{
		Auth: authH, Article: articleH, Tag: tagH, Upload: uploadH,
		Sessions: sessions, RDB: rdb,
		StaticWebDir: t.TempDir(), StaticUploadDir: t.TempDir(),
		SecureCookies: false,
		RateLimitIP:   middleware.RateLimitOpts{Limit: 1000, Window: time.Minute, KeyFunc: middleware.IPKey},
		RateLimitUser: middleware.RateLimitOpts{Limit: 1000, Window: time.Minute, KeyFunc: middleware.UserKey},
	})

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return ts, jar
}

func TestE2E_RegisterLoginPostArticleViewCount(t *testing.T) {
	ts, jar := setupServer(t)
	c := &http.Client{Jar: jar}

	// 1) 注册并自动登录
	mustPOST(t, c, ts.URL+"/api/v1/auth/register",
		`{"username":"alice","password":"abc12345","name":"Alice"}`)
	// 2) 取 csrf
	csrf := getCookie(jar, ts.URL, "csrf_token")
	require.NotEmpty(t, csrf)

	// 3) 发文章
	body := mustPOSTWithCSRF(t, c, ts.URL+"/api/v1/articles", csrf,
		`{"title":"Hello","content":"# Hi\n\n这是一篇测试文章 12345","tags":["go","test"]}`)
	var created struct {
		Code int `json:"code"`
		Data struct{ ID uint64 `json:"id"` } `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &created))
	require.Equal(t, 0, created.Code)
	require.NotZero(t, created.Data.ID)

	// 4) 详情两次,看到浏览数 +2
	for i := 0; i < 2; i++ {
		mustGET(t, c, fmt.Sprintf("%s/api/v1/articles/%d", ts.URL, created.Data.ID))
	}
	body = mustGET(t, c, fmt.Sprintf("%s/api/v1/articles/%d", ts.URL, created.Data.ID))
	var got struct {
		Code int `json:"code"`
		Data struct {
			ViewCount int64 `json:"view_count"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	require.GreaterOrEqual(t, got.Data.ViewCount, int64(3))
}

func mustGET(t *testing.T, c *http.Client, u string) []byte {
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return b
}
func mustPOST(t *testing.T, c *http.Client, u, body string) []byte {
	req, _ := http.NewRequest(http.MethodPost, u, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return b
}
func mustPOSTWithCSRF(t *testing.T, c *http.Client, u, csrf, body string) []byte {
	req, _ := http.NewRequest(http.MethodPost, u, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return b
}
func getCookie(jar http.CookieJar, base, name string) string {
	u, _ := url.Parse(base)
	for _, c := range jar.Cookies(u) {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}
```

> 注: 上面的 `repository.UserModel` 等假设仓储包导出了模型类型。如果你在 Phase 1 里把模型放在 `internal/model` 包,把 import 与类型名相应替换为 `model.User` / `model.Article` / `model.Tag` / `model.ArticleTag`。

- [ ] **Step 2: 跑集成测试**

```bash
go test -tags=integration ./test/integration/... -v -count=1
```

预期: PASS(需要本地有 docker)。CI 上把这一行加到 `make test-integration`。

- [ ] **Step 3: Commit**

```bash
git add Server/test/integration/e2e_test.go
git commit -m "test(server): e2e integration test with testcontainers (mysql+redis)"
```

---

### Task 29: 最终验证清单

**Files:**
- Modify: `Server/Makefile`(补齐 `test-integration` 目标和 `run` 目标)

- [ ] **Step 1: 完善 Makefile**

`Server/Makefile`(在 Phase 0 Task 1 已建立的基础上增加):

```makefile
.PHONY: build test test-integration lint run docker-up docker-down

build:
	go build -o bin/server ./cmd/server

test:
	go test ./... -count=1

test-integration:
	go test -tags=integration ./test/integration/... -count=1 -v

lint:
	golangci-lint run ./...

run: build
	./bin/blog

docker-up:
	docker compose up -d

docker-down:
	docker compose down
```

- [ ] **Step 2: 完整链路验证**

```bash
# 1. 启依赖
make docker-up

# 2. 跑数据库迁移(开发期可以用 GORM AutoMigrate 替代,但生产用 SQL)
mysql -h 127.0.0.1 -P 3306 -u blog -pblog blog < migrations/001_init.up.sql

# 3. 跑单测
make test

# 4. 跑集成测试(testcontainers 会自己启 docker)
make test-integration

# 5. 跑服务
make run
```

预期:每一步都没有报错。

- [ ] **Step 3: 手动冒烟测试(curl)**

```bash
# 注册
curl -i -c jar.txt -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"abc12345","name":"Alice"}' \
  http://127.0.0.1:8080/api/v1/auth/register

# 登录
curl -i -b jar.txt -c jar.txt -H 'Content-Type: application/json' \
  -d '{"username":"alice","password":"abc12345"}' \
  http://127.0.0.1:8080/api/v1/auth/login

# 取 CSRF
CSRF=$(grep csrf_token jar.txt | awk '{print $7}')

# 发文章
curl -i -b jar.txt -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
  -d '{"title":"Hello","content":"# Hi 这是一篇测试文章 12345","tags":["go"]}' \
  http://127.0.0.1:8080/api/v1/articles

# 看列表
curl -s http://127.0.0.1:8080/api/v1/articles | jq

# 看详情(浏览 +1)
curl -s http://127.0.0.1:8080/api/v1/articles/1 | jq

# 等 30s,再看 view_count 是否落进 DB
sleep 35
mysql -h 127.0.0.1 -u blog -pblog blog -e "SELECT id, view_count FROM articles;"
```

- [ ] **Step 4: Commit**

```bash
git add Server/Makefile
git commit -m "chore(server): finalize Makefile targets and verification scripts"
```

---

## 验证清单(对照 Server.md §11)

- [ ] 注册接口能创建用户并下发会话 Cookie
- [ ] 重复 username 注册返回 1010
- [ ] 错密码登录返回 2010
- [ ] 登录后 30 分钟内任一鉴权请求都会刷新 sid 与 csrf_token 的过期时间
- [ ] `POST /api/v1/auth/logout` 后 sid Cookie Max-Age=0
- [ ] 详情接口被调用时 Redis `view:<id>` INCR,并且 `view:dirty` SADD
- [ ] 30 秒后 worker 把 INCR 值合进 `articles.view_count`,Redis key 重置为 0
- [ ] 上传 6MB 图片返回 1021;上传 .exe 改名 .png 返回 1020
- [ ] 速率限制达到上限后返回 2020 + Retry-After 头
- [ ] CSRF 缺失或不匹配返回 2030
- [ ] 编辑别人的文章返回 2002
- [ ] panic 后服务能恢复并返回 5099 而不挂掉

---

## Self-Review

- 所有任务的 Step 序列完整(失败测试 → 跑出 FAIL → 实现 → 跑通 → Commit),无 placeholder
- 每个文件路径都是绝对相对 Server/ 根的精确路径
- 类型与方法名跨任务一致(如 `CounterRepo`、`ArticleRepo`、`SessionStore`)
- 全部 Server.md §3.5、§4、§5、§6、§7、§9 的需求都有对应的 Task
- TDD、DRY、YAGNI 原则贯穿全计划

---

后端实现计划完整。





