# MyBlog

一个基于 Go 的多用户博客平台，支持文章管理、标签过滤、浏览计数等功能。

## 功能特性

- **用户认证**：注册、登录、退出，支持 Session 管理和 CSRF 保护
- **文章管理**：创建、编辑、删除文章，支持 Markdown 格式
- **标签系统**：文章标签管理和过滤
- **图片上传**：支持多种图片格式，大小限制 5MB
- **浏览计数**：基于 Redis 的高性能浏览量统计
- **限流保护**：登录和上传接口限流

## 技术栈

| 分类 | 技术 | 版本 |
|------|------|------|
| 语言 | Go | 1.20+ |
| Web 框架 | Gin | 1.9.1 |
| ORM | GORM | 1.31.1 |
| 数据库 | PostgreSQL | 16+ |
| 缓存 | Redis | 7.0+ |

## 快速开始

### 环境要求

- Go 1.20+
- MySQL 8.0+
- Redis 7.0+
- Docker & Docker Compose（可选，用于快速启动依赖服务）

### 使用 Docker Compose 启动依赖

```bash
cd Server
make docker-up
```

这将启动 MySQL（端口 3306）和 Redis（端口 6379）。

### 手动启动依赖

**MySQL**:
```bash
# 创建数据库
CREATE DATABASE blog CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'blog'@'localhost' IDENTIFIED BY 'blog';
GRANT ALL PRIVILEGES ON blog.* TO 'blog'@'localhost';
```

**Redis**:
```bash
redis-server
```

### 运行项目

```bash
cd Server
make deps
make migrate-up
make run
```

服务将在 `http://localhost:8080` 启动。

### 构建生产版本

```bash
cd Server
make build
./bin/server
```

## 项目结构

```
├── Server/                    # 后端代码
│   ├── cmd/server/           # 入口文件
│   ├── internal/             # 内部模块
│   │   ├── handler/          # HTTP 处理器
│   │   ├── service/          # 业务逻辑层
│   │   ├── repository/       # 数据访问层
│   │   ├── model/            # 数据库模型
│   │   ├── middleware/       # 中间件
│   │   ├── config/           # 配置管理
│   │   ├── db/               # 数据库连接
│   │   ├── cache/            # Redis 缓存
│   │   └── pkg/              # 工具包
│   ├── migrations/           # 数据库迁移脚本
│   ├── test/integration/     # 集成测试
│   ├── config.yaml           # 配置文件
│   ├── docker-compose.yml    # Docker Compose 配置
│   └── Makefile              # 构建脚本
├── Web/                      # 前端代码
│   ├── assets/               # 静态资源
│   ├── vendor/               # 第三方依赖
│   └── *.html                # 页面模板
├── ServerRPD.md              # 后端设计文档
├── WebPRD.md                 # 前端设计文档
└── README.md                 # 项目说明
```

## API 接口

### 认证

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/auth/register` | POST | 注册新用户 |
| `/api/v1/auth/login` | POST | 用户登录 |
| `/api/v1/auth/logout` | POST | 用户退出 |
| `/api/v1/auth/me` | GET | 获取当前用户 |

### 文章

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/articles` | GET | 获取文章列表 |
| `/api/v1/articles/:id` | GET | 获取文章详情 |
| `/api/v1/articles` | POST | 创建文章 |
| `/api/v1/articles/:id` | PUT | 更新文章 |
| `/api/v1/articles/:id` | DELETE | 删除文章 |

### 标签

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/tags` | GET | 获取所有标签 |

### 上传

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/uploads/image` | POST | 上传图片 |

## 测试

### 单元测试

```bash
cd Server
make test
```

### 集成测试

```bash
cd Server
make integration
```

## 配置说明

配置文件位于 `Server/config.yaml`，支持环境变量覆盖：

```yaml
server:
  addr: ":8080"           # 服务端口
  static_dir: "../Web"    # 前端静态文件目录
  upload_dir: "./uploads" # 上传文件目录

postgres:
  dsn: "host=127.0.0.1 port=5432 user=blog password=blog dbname=blog sslmode=disable TimeZone=Asia/Shanghai"

redis:
  addr: "127.0.0.1:6379"

session:
  ttl_minutes: 30         # Session 过期时间

upload:
  max_bytes: 5242880      # 最大上传大小（5MB）
```

## 许可证

MIT License
