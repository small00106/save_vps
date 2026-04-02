# CloudNest

VPS 监控 + 分布式存储一体化管理平台。基于 Master-Agent 架构，将多台 VPS 组成统一管理面板。

## 功能

### 监控
- CPU / RAM / Swap / Disk / 网络 / 负载 实时监控与历史图表
- 节点在线状态追踪（30s 心跳超时自动标记离线）
- 流量统计
- 指标自动压缩（4h 内保留原始数据，之后压缩为 15min 粒度）

### 存储
- Agent 定期上报指定目录的文件树，网页端按节点浏览
- 签名 URL 直传下载（不经过 Master）
- 用户选择目标节点上传文件
- 跨节点文件搜索

### 远程操作
- WebSocket 远程终端
- 远程命令执行
- Ping 探测任务（ICMP / TCP / HTTP）

### 告警
- 自定义告警规则（CPU / 内存 / 磁盘 / 离线）
- 多通知渠道（Telegram / Webhook / Email / Bark）
- 审计日志

### 其他
- 节点标签与筛选
- Agent 传输限速
- 单管理员认证

## 架构

```
               ┌──────────────────────────┐
               │   Master Server (:8800)  │
               │  监控 / 存储 / 告警 / 面板 │
               │  SQLite · Go · React     │
               └────┬──────┬──────┬───────┘
                    │ WS   │ WS   │ WS
             ┌──────┘      │      └──────┐
             ▼             ▼             ▼
       ┌──────────┐  ┌──────────┐  ┌──────────┐
       │ Agent A  │  │ Agent B  │  │ Agent C  │
       │  :8801   │  │  :8801   │  │  :8801   │
       └──────────┘  └──────────┘  └──────────┘
             ▲             ▲             ▲
             └──── HTTPS 签名URL直传 ────┘
               ┌────────────────────────┐
               │      用户 / 浏览器      │
               └────────────────────────┘
```

- **控制面**：Agent ↔ Master 通过 WebSocket + JSON-RPC 2.0 通信（心跳、指令、文件树上报）
- **数据面**：用户通过签名 URL 直接与 Agent 传输文件，Master 不代理数据

## 技术栈

| 组件 | 技术 |
|------|------|
| Master 后端 | Go · Gin · GORM · gorilla/websocket · Cobra |
| Agent | Go · Gin · gopsutil |
| 前端 | React · TypeScript · Tailwind CSS · Recharts |
| 数据库 | SQLite（默认）/ MySQL |
| 部署 | Docker · systemd |

## 快速开始

### 方式一：Docker 部署 Master（推荐）

```bash
git clone https://github.com/yourname/cloudnest.git
cd cloudnest

# 修改注册密钥和签名密钥
vim docker-compose.yml

# 一键启动
docker compose up -d --build
```

启动后访问 `http://your-server-ip:8800`

默认账号：`admin` / `admin`（请立即修改密码）

### 方式二：二进制部署 Master

```bash
# 构建前端
cd cloudnest-web && npm ci && npm run build
cp -r dist/* ../cloudnest/public/dist/

# 编译 Master（需要 CGO，因为 SQLite）
cd ../cloudnest
CGO_ENABLED=1 go build -o cloudnest .

# 启动
./cloudnest server -l 0.0.0.0:8800
```

### 部署 Agent（每台 VPS 执行一条命令）

```bash
curl -sSL http://master-ip:8800/install.sh | bash -s -- --token my-secret-token
```

脚本会自动：下载 Agent 二进制 → 注册到 Master → 创建 systemd 服务 → 启动。

**可选参数：**

```bash
curl -sSL http://master-ip:8800/install.sh | bash -s -- \
  --token my-secret-token \
  --port 8801 \
  --scan-dirs /data
```

**手动部署 Agent：**

```bash
# 交叉编译（无 CGO 依赖）
cd cloudnest-agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o cloudnest-agent .

# 上传到 VPS 后
./cloudnest-agent register --master http://master-ip:8800 --token my-secret-token
./cloudnest-agent run
```

## 环境变量

### Master

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CLOUDNEST_LISTEN` | `0.0.0.0:8800` | 监听地址 |
| `CLOUDNEST_DB_TYPE` | `sqlite` | 数据库类型（sqlite / mysql） |
| `CLOUDNEST_DB_DSN` | `./data/cloudnest.db` | 数据库连接（SQLite 文件路径或 MySQL DSN） |
| `CLOUDNEST_REG_TOKEN` | `cloudnest-register` | Agent 注册密钥 |
| `CLOUDNEST_SIGNING_SECRET` | `cloudnest-default-secret` | 签名 URL 的 HMAC 密钥 |

### Agent

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CLOUDNEST_SIGNING_SECRET` | `cloudnest-default-secret` | 签名 URL 的 HMAC 密钥（需与 Master 一致） |

## 项目结构

```
cloudnest/                  # Master 服务端 (28 Go files)
├── cmd/                    # Cobra CLI
├── internal/
│   ├── api/                # REST API (agent/auth/files/nodes/alerts/ping/command/terminal/admin)
│   ├── database/           # GORM 模型 + 初始化
│   ├── ws/                 # WebSocket Hub + Dashboard 推送
│   ├── scheduler/          # 后台任务 (健康检查/指标压缩/告警评估)
│   ├── transfer/           # HMAC 签名 URL
│   ├── notify/             # 通知发送 (Telegram/Webhook/Email/Bark)
│   └── cache/              # go-cache 缓冲
├── public/                 # go:embed 前端静态文件
└── Dockerfile

cloudnest-agent/            # Agent 客户端 (13 Go files)
├── cmd/                    # register / run 命令
├── internal/
│   ├── agent/              # 主循环 + 配置 + 注册
│   ├── ws/                 # WebSocket 客户端 + 重连
│   ├── storage/            # 文件读写
│   ├── server/             # HTTP 数据面 + 限速 + 签名验证
│   ├── terminal/           # 远程终端
│   └── reporter/           # 系统指标采集 + 文件树扫描
└── install.sh

cloudnest-web/              # 前端 (16 TS/TSX files)
├── src/
│   ├── pages/              # Dashboard/NodeDetail/FileBrowser/Alerts/PingTasks/AuditLog/Login
│   ├── components/         # Layout/Sparkline
│   ├── hooks/              # useAuth/useWebSocket
│   └── api/                # 类型化 API 客户端
└── package.json

Dockerfile                  # 多阶段构建 (前端 + Agent交叉编译 + Master)
docker-compose.yml          # 一键部署
ARCHITECTURE.md             # 详细架构设计文档
```

## API 概览

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 登录 |
| GET | `/api/nodes` | 节点列表（含实时指标） |
| GET | `/api/nodes/:uuid/metrics?range=1h` | 历史指标 |
| GET | `/api/nodes/:uuid/files?path=/data` | 浏览节点文件目录 |
| GET | `/api/nodes/:uuid/download?path=...` | 获取文件下载签名 URL |
| POST | `/api/files/upload` | 初始化上传（用户选节点） |
| GET | `/api/files/search?q=keyword` | 跨节点文件搜索 |
| POST | `/api/nodes/:uuid/exec` | 远程命令执行 |
| WS | `/api/ws/dashboard` | 实时监控数据推送 |
| WS | `/api/ws/terminal/:uuid` | 远程终端 |
| GET/POST | `/api/ping/tasks` | Ping 探测任务 |
| GET/POST | `/api/alerts/rules` | 告警规则 |
| GET/POST | `/api/alerts/channels` | 通知渠道 |
| GET | `/api/admin/audit` | 审计日志 |
| GET | `/install.sh` | Agent 一键安装脚本 |

## 生产部署建议

1. **修改默认密钥** — `CLOUDNEST_REG_TOKEN` 和 `CLOUDNEST_SIGNING_SECRET` 务必改为随机字符串
2. **修改 admin 密码** — 首次登录后立即修改
3. **HTTPS** — 使用 nginx / caddy 反代并配置 SSL 证书
4. **防火墙** — Master 开放 8800，Agent 开放 8801
5. **备份** — 定期备份 `data/cloudnest.db`

### Nginx 反代示例

```nginx
server {
    listen 443 ssl;
    server_name cloudnest.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8800;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location ~ ^/api/ws/ {
        proxy_pass http://127.0.0.1:8800;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## License

MIT
