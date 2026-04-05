# CloudNest

> VPS 监控 + 分布式存储一体化管理平台

基于 Master-Agent 架构，将多台 VPS 组成统一管理面板，提供实时监控、文件管理、远程终端和告警通知等功能。

## 目录

- [功能特性](#功能特性)
- [架构概览](#架构概览)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
  - [Docker 部署（推荐）](#方式一docker-部署推荐)
  - [二进制部署](#方式二二进制部署)
  - [部署 Agent](#部署-agent)
- [环境变量](#环境变量)
- [项目结构](#项目结构)
- [API 概览](#api-概览)
- [通信协议](#通信协议)
- [生产部署建议](#生产部署建议)
- [License](#license)

## 功能特性

### 监控

- CPU / RAM / Swap / Disk / 网络 / 负载实时监控与历史图表
- 节点在线状态追踪（30s 心跳超时自动标记离线）
- 流量统计（上行/下行）
- 指标自动压缩（4h 内保留原始数据，之后压缩为 15min 粒度）

### 文件管理

- Agent 每 10s 上报受管目录的文件树（首次全量，后续增量），网页端按节点浏览
- 文件上传/下载通过 Master 代理转发（浏览器 → Master → Agent），HTTPS 环境无 mixed-content 问题
- 节点详情页支持单文件上传到当前目录、同名覆盖确认、文件夹打包 zip 下载
- 全局文件搜索与托管文件元数据管理
- 文件副本复制与 SHA-256 校验协议已预留（Master 暂无调度入口）

### 远程操作

- WebSocket 远程终端（Master ↔ Agent shell）
- 远程命令执行（60s 超时）
- Ping 探测任务（ICMP / TCP / HTTP，多节点分布式执行）

### 告警

- 自定义告警规则（CPU / 内存 / 磁盘 / 离线）
- 持续时间窗口判定 + 冷却机制（防重复告警）
- 多通知渠道：Telegram / Webhook / Email / Bark / ServerChan
- 审计日志

### 其他

- 节点标签管理
- Agent 传输限速（读写双向）
- 单管理员认证（Cookie + Bearer 双模式，30 天会话）

## 架构概览

```
            ┌─────────────────────────────┐
            │    Master Server (:8800)    │
            │  监控 / 存储 / 告警 / 面板    │
            │  SQLite · Go · React        │
            └───┬─────────┬─────────┬─────┘
                │ WS      │ WS      │ WS       ← JSON-RPC 2.0
          ┌─────┘         │         └─────┐
          ▼               ▼               ▼
    ┌──────────┐    ┌──────────┐    ┌──────────┐
    │ Agent A  │    │ Agent B  │    │ Agent C  │
    │  :8801   │    │  :8801   │    │  :8801   │
    └──────────┘    └──────────┘    └──────────┘
```

- **控制面**：Agent ↔ Master 通过 WebSocket + JSON-RPC 2.0 通信（心跳、指令、文件树上报）
- **数据面**：文件上传/下载通过 Master 代理转发，流式传输不缓存整个文件，Agent 无需暴露端口到公网

> 详细架构设计请参阅 [ARCHITECTURE.md](./ARCHITECTURE.md)。

## 技术栈

| 组件 | 技术 |
|------|------|
| Master 后端 | Go · Gin · GORM · gorilla/websocket · Cobra |
| Agent | Go · Gin · gopsutil |
| 前端 | React · TypeScript · Tailwind CSS · Recharts |
| 数据库 | SQLite（默认）/ MySQL |
| 部署 | Docker · systemd |

## 快速开始

### 方式一：Docker 部署（推荐）

```bash
git clone https://github.com/small00106/save_vps.git
cd save_vps

# 编辑 docker-compose.yml，补齐数据库和密钥配置
vim docker-compose.yml

# 一键启动
docker compose up -d --build
```

如果你只是单机快速启动，建议在 `docker-compose.yml` 中显式覆盖为 SQLite：

```yaml
services:
  cloudnest:
    environment:
      - CLOUDNEST_DB_TYPE=sqlite
      - CLOUDNEST_DB_DSN=/app/data/cloudnest.db
      - CLOUDNEST_REG_TOKEN=my-secret-token
      - CLOUDNEST_SIGNING_SECRET=change-me-in-production
```

说明：

- 当前 `Dockerfile` 内置默认值是 `CLOUDNEST_DB_TYPE=mysql`
- 仓库里的 `docker-compose.yml` 模板没有自带 MySQL 服务
- 如果不显式改成 SQLite，就需要自行提供一个可访问的 MySQL 实例，并同步设置 `CLOUDNEST_DB_DSN`

启动后访问 `http://<your-server-ip>:8800`

默认账号：`admin` / `admin`（**请立即修改密码**）

> Docker 构建会自动编译前端、交叉编译 Agent 二进制（linux/amd64 + linux/arm64），并嵌入到 Master 镜像中。

### 方式二：二进制部署

```bash
# 1. 构建前端
cd cloudnest-web && npm ci && npm run build
cp -r dist/* ../cloudnest/public/dist/

# 2. 编译 Master（需要 CGO，因为 SQLite）
cd ../cloudnest
CGO_ENABLED=1 go build -o cloudnest .

# 3. 启动
./cloudnest server -l 0.0.0.0:8800
```

### 部署 Agent

每台 VPS 执行一条命令即可完成安装：

```bash
curl -sSL http://<master-ip>:8800/install.sh | bash -s -- \
  --token <your-reg-token> \
  --secret <your-signing-secret>
```

脚本会自动：检测 OS/架构 → 下载 Agent 二进制 → 注册到 Master → 创建 systemd 服务 → 启动。

如果 Master 已通过域名暴露（例如 `https://ops.example.com`），直接替换地址即可，脚本会自动检测协议。

**可选参数：**

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--token` | 注册密钥（对应 Master 的 `CLOUDNEST_REG_TOKEN`） | — |
| `--secret` | 签名密钥（需与 Master 的 `CLOUDNEST_SIGNING_SECRET` 一致） | — |
| `--port` | Agent 监听端口 | `8801` |

> **支持架构：** linux/amd64、linux/arm64

<details>
<summary>手动部署 Agent</summary>

```bash
# 交叉编译（无 CGO 依赖）
cd cloudnest-agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o cloudnest-agent .

# 上传到 VPS 后注册并启动
export CLOUDNEST_SIGNING_SECRET=<your-signing-secret>
./cloudnest-agent register --master http://<master-ip>:8800 --token <your-reg-token>
./cloudnest-agent run
```

Agent 配置存储在 `~/.cloudnest/agent.json`，注册后自动生成。

如果你使用 systemd 管理 Agent，也要把 `CLOUDNEST_SIGNING_SECRET` 写入 service 环境变量；否则 Master 代理到 Agent 的签名请求会校验失败。

</details>

## 环境变量

### Master

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CLOUDNEST_LISTEN` | `0.0.0.0:8800` | 监听地址 |
| `CLOUDNEST_DB_TYPE` | `sqlite` | 数据库类型（`sqlite` / `mysql`） |
| `CLOUDNEST_DB_DSN` | `./data/cloudnest.db` | 数据库连接字符串 |
| `CLOUDNEST_REG_TOKEN` | `cloudnest-register` | Agent 注册密钥 |
| `CLOUDNEST_SIGNING_SECRET` | `cloudnest-default-secret` | 代理请求的 HMAC 签名密钥 |

### Agent

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CLOUDNEST_SIGNING_SECRET` | `cloudnest-default-secret` | HMAC 签名密钥（需与 Master 一致） |

## 项目结构

```
save_vps/
├── cloudnest/                  # Master 服务端
│   ├── cmd/                    # Cobra CLI (root, server)
│   ├── internal/
│   │   ├── api/                # HTTP/WS 接口
│   │   │   ├── agent/          #   Agent 注册 + WS 消息处理
│   │   │   ├── auth/           #   登录 / 会话 / 默认管理员
│   │   │   ├── files/          #   文件 CRUD / 搜索 / 代理转发
│   │   │   ├── nodes/          #   节点列表 / 详情 / 指标 / 流量
│   │   │   ├── alerts/         #   告警规则 + 通知渠道
│   │   │   ├── ping/           #   Ping 探测任务
│   │   │   ├── command/        #   远程命令执行
│   │   │   ├── terminal/       #   远程终端 WS 代理
│   │   │   └── admin/          #   系统设置 + 审计日志
│   │   ├── database/           # GORM 初始化 + 数据模型
│   │   ├── ws/                 # WebSocket Hub + 实时推送
│   │   ├── scheduler/          # 后台任务（落库/健康检查/压缩/告警/GC）
│   │   ├── transfer/           # HMAC 签名 URL
│   │   ├── notify/             # 通知发送（Telegram/Webhook/Email/Bark/ServerChan）
│   │   ├── cache/              # go-cache 指标 + 文件树缓存
│   │   └── server/             # Gin 路由 + 中间件 + 安装脚本生成
│   ├── public/                 # go:embed 前端静态文件
│   └── Dockerfile
│
├── cloudnest-agent/            # Agent 客户端
│   ├── cmd/                    # register / run 命令
│   ├── internal/
│   │   ├── agent/              # 主循环 + 配置 + 注册
│   │   ├── ws/                 # WebSocket 客户端 + 指数退避重连
│   │   ├── server/             # HTTP 数据面 + 限速 + 签名验证
│   │   ├── terminal/           # 远程终端
│   │   └── reporter/           # 系统指标采集 + 文件树扫描
│   └── go.mod
│
├── cloudnest-web/              # 前端
│   ├── src/
│   │   ├── pages/              # Dashboard / NodeDetail / FileBrowser / Terminal / ...
│   │   ├── components/         # Layout / Sparkline
│   │   ├── hooks/              # useAuth / useWebSocket
│   │   └── api/                # 类型化 API 客户端
│   └── package.json
│
├── Dockerfile                  # 多阶段构建（前端 + Agent 交叉编译 + Master）
├── docker-compose.yml          # 一键部署模板
└── ARCHITECTURE.md             # 详细架构设计文档
```

## API 概览

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 登录 |
| POST | `/api/auth/logout` | 登出 |
| GET | `/api/auth/me` | 获取当前用户 |
| POST | `/api/auth/change-password` | 修改当前用户密码 |

### 节点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/nodes` | 节点列表（含实时指标） |
| GET | `/api/nodes/:uuid` | 节点详情 |
| GET | `/api/nodes/:uuid/metrics?range=` | 历史指标（`1h` / `4h` / `24h` / `7d`） |
| GET | `/api/nodes/:uuid/traffic` | 流量统计 |
| PUT | `/api/nodes/:uuid/tags` | 设置节点标签 |

### 文件管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/nodes/:uuid/files?path=/` | 浏览节点受管目录 |
| GET | `/api/nodes/:uuid/download?path=` | 文件/文件夹下载代理 URL |
| POST | `/api/files/upload` | 初始化上传，返回代理 URL |
| GET | `/api/files/download/:id` | 获取下载代理 URL |
| GET | `/api/files?path=` | 列出虚拟目录 |
| POST | `/api/files/mkdir` | 创建虚拟目录 |
| DELETE | `/api/files/:id` | 删除文件 |
| PUT | `/api/files/:id/move` | 移动/重命名 |
| GET | `/api/files/search?q=` | 搜索已托管文件 |

### 代理转发

| 方法 | 路径 | 说明 |
|------|------|------|
| PUT | `/api/proxy/upload/:file_id` | 上传代理（流式转发到 Agent） |
| GET | `/api/proxy/download/:file_id` | 下载代理（兼容旧接口） |
| GET | `/api/proxy/browse` | 节点文件/文件夹下载代理 |

### 远程操作

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/nodes/:uuid/exec` | 远程命令执行 |
| GET | `/api/commands/:id` | 查询命令结果 |
| WS | `/api/ws/terminal/:uuid` | 远程终端 |
| WS | `/api/ws/dashboard` | 实时监控数据推送 |

### Ping 探测

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/ping/tasks` | 任务列表 |
| POST | `/api/ping/tasks` | 创建任务 |
| GET | `/api/ping/tasks/:id/results` | 查看结果 |
| DELETE | `/api/ping/tasks/:id` | 删除任务 |

### 告警 & 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/alerts/rules` | 告警规则列表 / 创建 |
| PUT/DELETE | `/api/alerts/rules/:id` | 更新 / 删除告警规则 |
| GET/POST | `/api/alerts/channels` | 通知渠道列表 / 创建 |
| PUT | `/api/alerts/channels/:id` | 更新通知渠道 |
| GET/PUT | `/api/admin/settings` | 系统设置 |
| GET | `/api/admin/audit` | 审计日志 |
| GET | `/install.sh` | Agent 一键安装脚本 |
| GET | `/download/agent/:os/:arch` | Agent 二进制下载 |

## 通信协议

Agent 与 Master 通过 WebSocket + JSON-RPC 2.0 通信，连接地址：

```
ws(s)://<master>:8800/api/agent/ws
Authorization: Bearer <agent_token>
```

**Agent → Master：**

| 方法 | 用途 | 频率 |
|------|------|------|
| `agent.heartbeat` | 系统指标上报 | 每 10s |
| `agent.fileTree` | 文件树同步（首次全量，后续增量） | 每 10s |
| `agent.fileStored` | 文件写入确认 | 事件触发 |
| `agent.fileDeleted` | 文件删除确认 | 事件触发 |
| `agent.pingResult` | Ping 探测结果 | 事件触发 |
| `agent.commandResult` | 命令执行结果 | 事件触发 |

**Master → Agent：**

| 方法 | 用途 |
|------|------|
| `master.deleteFile` | 删除文件 |
| `master.execCommand` | 执行远程命令 |
| `master.startPing` | 启动 Ping 探测 |
| `master.stopPing` | 停止 Ping 探测 |
| `master.replicateFile` | 拉取文件副本（预留） |
| `master.verifyFile` | 校验文件 SHA-256（预留） |

**签名算法：** Master 代理请求到 Agent 时使用 HMAC-SHA256 签名（5 分钟有效期），签名绑定 HTTP 方法防跨方法重放。详见 [ARCHITECTURE.md](./ARCHITECTURE.md)。

## 生产部署建议

1. **修改默认密钥** — `CLOUDNEST_REG_TOKEN` 和 `CLOUDNEST_SIGNING_SECRET` 务必改为随机字符串
2. **修改 admin 密码** — 首次登录后立即修改
3. **HTTPS** — 使用 Nginx / Caddy 反代 Master 并配置 SSL 证书
4. **防火墙** — Master 开放 `8800`；Agent `8801` 端口仅需对 Master IP 可达（无需对公网开放）
5. **备份** — SQLite 部署备份 `data/cloudnest.db`；MySQL 部署定期 dump

<details>
<summary>Nginx 反代配置示例</summary>

```nginx
server {
    listen 443 ssl;
    server_name cloudnest.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    client_max_body_size 0;           # 不限制上传大小

    # 普通请求
    location / {
        proxy_pass http://127.0.0.1:8800;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
    }

    # WebSocket
    location ~ ^/api/ws/ {
        proxy_pass http://127.0.0.1:8800;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400s;
    }

    # 文件代理（流式转发）
    location ~ ^/api/proxy/ {
        proxy_pass http://127.0.0.1:8800;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 600s;
        proxy_request_buffering off;
    }
}
```

</details>

## License

[MIT](./LICENSE)
