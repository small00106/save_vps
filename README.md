# CloudNest

VPS 监控 + 分布式存储一体化管理平台。基于 Master-Agent 架构，将多台 VPS 组成统一管理面板。

## 功能

### 监控
- CPU / RAM / Swap / Disk / 网络 / 负载 实时监控与历史图表
- 节点在线状态追踪（30s 心跳超时自动标记离线）
- 流量统计
- 指标自动压缩（4h 内保留原始数据，之后压缩为 15min 粒度）

### 存储
- Agent 每 10s 上报受管目录（默认 `~/data_save/files`）的文件树（首次全量，后续增量），网页端按节点浏览
- 文件上传/下载通过 Master 代理转发（浏览器 → Master → Agent），HTTPS 环境无 mixed content 问题
- 节点详情页支持按当前目录上传文件，保留原名并支持同名覆盖确认
- 节点目录支持下载文件和文件夹（文件夹打包为 zip）
- 全局 `/files` 页面保留已托管文件搜索；节点上手动放入但未入库的文件不纳入搜索
- 托管文件元数据接口（列表/新建目录/移动/删除）仍保留兼容
- 文件副本复制与 SHA-256 校验协议已预留（Master 暂无调度入口）

### 远程操作
- WebSocket 远程终端（Master 代理 ↔ Agent shell）
- 远程命令执行（60s 超时）
- Ping 探测任务（ICMP / TCP / HTTP，多节点分布式执行）

### 告警
- 自定义告警规则（CPU / 内存 / 磁盘 / 离线）
- 持续时间窗口判定 + 冷却机制（非单点触发，防重复告警）
- 多通知渠道（Telegram / Webhook / Email / Bark / ServerChan）
- 审计日志

### 其他
- 节点标签（展示 + API 更新；前端筛选待补）
- Agent 传输限速（读写双向）
- 单管理员认证（Cookie + Bearer 双模式，30 天会话）
- 前端遇到会话过期的 `401` 会统一清理本地状态并回到登录页

## 架构

```
               ┌──────────────────────────┐
               │   Master Server (:8800)  │
               │  监控 / 存储 / 告警 / 面板 │
               │  SQLite · Go · React     │
               └────┬──────┬──────┬───────┘
                    │ WS   │ WS   │ WS       ← 控制面 (JSON-RPC 2.0)
             ┌──────┘      │      └──────┐
             ▼             ▼             ▼
       ┌──────────┐  ┌──────────┐  ┌──────────┐
       │ Agent A  │  │ Agent B  │  │ Agent C  │
       │  :8801   │  │  :8801   │  │  :8801   │
       └──────────┘  └──────────┘  └──────────┘
```

- **控制面**：Agent ↔ Master 通过 WebSocket + JSON-RPC 2.0 通信（心跳、指令、文件树上报），连接启用 ping/pong 保活与读取超时
- **数据面**：文件上传/下载通过 Master 代理转发（浏览器 ↔ Master ↔ Agent），流式传输不缓存整个文件，关键 HTTP 请求带固定超时，Agent 无需暴露端口到公网

## 技术栈

| 组件 | 技术 |
|------|------|
| Master 后端 | Go · Gin · GORM · gorilla/websocket · Cobra |
| Agent | Go · Gin · gopsutil |
| 前端 | React · TypeScript · Tailwind CSS · Recharts |
| 数据库 | SQLite（默认）/ MySQL |
| 部署 | GitHub Releases · systemd · Docker |

## 快速开始

### 方式一：GitHub Release + systemd（推荐）

直接安装最新版：

```bash
curl -fsSL https://raw.githubusercontent.com/small00106/save_vps/main/install-master.sh | sudo bash
```

如果你已经拉了仓库，也可以直接执行：

```bash
git clone https://github.com/small00106/save_vps.git
cd save_vps
sudo bash install-master.sh
```

安装脚本会：

- 从 GitHub Releases 下载 `cloudnest-master-linux-<arch>.tar.gz`
- 安装到 `/opt/cloudnest`
- 创建数据目录 `/var/lib/cloudnest`
- 写入环境文件 `/etc/cloudnest/cloudnest.env`
- 创建并启动 `cloudnest.service`

首次启动后访问 `http://your-server-ip:8800`。

默认账号：`admin` / `admin`。这是个人项目保留的初始入口；首次用默认密码登录后，前端会弹出一次修改提醒。

如果未显式设置 `CLOUDNEST_REG_TOKEN` / `CLOUDNEST_SIGNING_SECRET`，Master 会按以下优先级取值：

1. 读取环境变量
2. 读取数据目录下的 `secrets/reg_token` 与 `secrets/signing_secret`
3. 首次启动自动生成并写入 `secrets/`

`install-master.sh` 默认把数据目录放在 `/var/lib/cloudnest`，因此首次启动生成的 secrets 路径是：

- `/var/lib/cloudnest/secrets/reg_token`
- `/var/lib/cloudnest/secrets/signing_secret`

如果 Master 通过反向代理对外暴露，建议安装时同时写入公开地址：

```bash
curl -fsSL https://raw.githubusercontent.com/small00106/save_vps/main/install-master.sh | \
  sudo bash -s -- --public-base-url https://ops.example.com
```

这样 Master 生成的 Agent 安装脚本会优先使用 `CLOUDNEST_PUBLIC_BASE_URL`，而不是依赖请求头推断地址。

### 部署 Agent（每台 VPS 执行一条命令）

```bash
curl -sSL http://master-ip:8800/install.sh | bash -s -- \
  --token "$(sudo cat /var/lib/cloudnest/secrets/reg_token)" \
  --secret "$(sudo cat /var/lib/cloudnest/secrets/signing_secret)"
```

脚本会自动：检测 OS/架构 → 下载 Agent 二进制 → 注册到 Master → 创建 systemd 服务 → 启动。

如果 Master 已通过域名暴露（例如 `https://ops.example.com`），直接替换地址即可，脚本会自动检测协议：

```bash
curl -sSL https://ops.example.com/install.sh | bash -s -- \
  --token <reg_token> \
  --secret <signing_secret>
```

> `--token` 读取自 Master 数据目录的 `secrets/reg_token`；`--secret` 读取自 `secrets/signing_secret`。

> `--secret` 必须与 Master 当前使用的签名密钥一致，用于 Master 代理请求到 Agent 时的签名验证。

> 支持架构：linux/amd64、linux/arm64。

**可选参数：**

```bash
curl -sSL http://master-ip:8800/install.sh | bash -s -- \
  --token <reg_token> \
  --secret <signing_secret> \
  --port 8801
```

> 当前版本 Agent 运行时固定使用受管目录 `~/data_save/files`；在节点上手动放入该目录的新文件，约 10 秒内会出现在节点详情页 `Files` 中。

**手动部署 Agent：**

```bash
# 交叉编译（无 CGO 依赖）
cd cloudnest-agent
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o cloudnest-agent .

# 上传到 VPS 后
./cloudnest-agent register --master http://master-ip:8800 --token <reg_token>
CLOUDNEST_SIGNING_SECRET=<signing_secret> ./cloudnest-agent run
```

Agent 配置存储在 `~/.cloudnest/agent.json`，注册后自动生成。`run` 阶段要求显式提供 `CLOUDNEST_SIGNING_SECRET`；未设置或仍使用旧公开默认值会直接启动失败。

### 方式二：Docker 部署 Master（可选）

```bash
git clone https://github.com/small00106/save_vps.git
cd save_vps
docker compose up -d --build
```

Docker 路径仍然可用，镜像会把前端和 Agent 二进制一起打包进 Master。默认数据目录在容器内 `/app/data`，因此 secrets 默认位于：

- `/app/data/secrets/reg_token`
- `/app/data/secrets/signing_secret`

### 方式三：源码构建（开发者）

```bash
bash release/build-release.sh
```

构建完成后，发布产物会出现在 `release/dist/`：

- `cloudnest-master-linux-amd64.tar.gz`
- `cloudnest-master-linux-arm64.tar.gz`
- `cloudnest-agent-linux-amd64`
- `cloudnest-agent-linux-arm64`
- `checksums.txt`

## 环境变量

### Master

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CLOUDNEST_LISTEN` | `0.0.0.0:8800` | 监听地址 |
| `CLOUDNEST_DB_TYPE` | `sqlite` | 数据库类型（sqlite / mysql） |
| `CLOUDNEST_DB_DSN` | `./data/cloudnest.db` | 数据库连接（SQLite 文件路径或 MySQL DSN） |
| `CLOUDNEST_PUBLIC_BASE_URL` | 空 | 公开访问地址；`/install.sh` 生成 Agent 安装脚本时优先使用它 |
| `CLOUDNEST_REG_TOKEN` | 首启自动生成并持久化 | Agent 注册密钥；环境变量优先于本地 secrets 文件 |
| `CLOUDNEST_SIGNING_SECRET` | 首启自动生成并持久化 | 代理请求的 HMAC 签名密钥；环境变量优先于本地 secrets 文件 |

### Agent

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CLOUDNEST_SIGNING_SECRET` | 无（必填） | HMAC 签名密钥（需与 Master 一致；缺失或使用旧公开默认值时拒绝启动） |

## 项目结构

```
cloudnest/                  # Master 服务端
├── cmd/                    # Cobra CLI (root, server)
├── internal/
│   ├── api/
│   │   ├── agent/          # Agent 注册 + WS 消息处理
│   │   ├── auth/           # 登录/会话/默认管理员
│   │   ├── files/          # 文件上传/下载/CRUD/搜索 + 代理转发
│   │   ├── nodes/          # 节点列表/详情/指标/流量/标签
│   │   ├── alerts/         # 告警规则 + 通知渠道
│   │   ├── ping/           # Ping 探测任务
│   │   ├── command/        # 远程命令执行
│   │   ├── terminal/       # 远程终端 WS 代理
│   │   └── admin/          # 系统设置 + 审计日志
│   ├── database/
│   │   ├── dbcore/         # GORM 初始化 (sync.Once)
│   │   └── models/         # 所有数据模型
│   ├── ws/                 # WebSocket Hub + Dashboard 推送 + SafeConn
│   ├── scheduler/          # 后台任务 (指标落库/健康检查/压缩/告警/GC)
│   ├── transfer/           # HMAC 签名 URL
│   ├── notify/             # 通知发送 (Telegram/Webhook/Email/Bark/ServerChan)
│   ├── cache/              # go-cache 指标+文件树缓存
│   └── server/             # Gin 路由 + 中间件 + 安装脚本生成
├── public/                 # go:embed 前端静态文件
└── Dockerfile

cloudnest-agent/            # Agent 客户端
├── cmd/                    # register / run 命令
├── internal/
│   ├── agent/              # 主循环 + 配置 + 注册 + 命令/Ping/文件操作
│   ├── ws/                 # WebSocket 客户端 + 指数退避重连
│   ├── server/             # HTTP 数据面 + 限速 + 签名验证
│   ├── terminal/           # 远程终端
│   └── reporter/           # 系统指标采集 + 文件树扫描
└── go.mod

cloudnest-web/              # 前端
├── src/
│   ├── pages/              # Dashboard/NodeDetail/FileBrowser/Terminal/PingTasks/Alerts/AuditLog/Login
│   ├── components/         # Layout/Sparkline
│   ├── hooks/              # useAuth/useWebSocket
│   └── api/                # 类型化 API 客户端
└── package.json

Dockerfile                  # 多阶段构建 (前端 + Agent交叉编译 + Master)
docker-compose.yml          # 一键部署
install-master.sh           # Master 一键安装脚本（从 GitHub Releases 下载）
release/                    # 发布脚本与说明（dist/ 默认忽略）
ARCHITECTURE.md             # 详细架构设计文档
```

## API 概览

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 登录（同一 IP 5 分钟内最多 5 次失败，第 6 次返回 `429`） |
| POST | `/api/auth/logout` | 登出 |
| GET | `/api/auth/me` | 获取当前用户 |
| POST | `/api/auth/default-password-notice/ack` | 确认默认密码提醒（全系统仅一次） |

### 节点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/nodes` | 节点列表（含实时指标） |
| GET | `/api/nodes/:uuid` | 节点详情 |
| GET | `/api/nodes/:uuid/metrics?range=1h` | 历史指标（1h/4h/24h/7d） |
| GET | `/api/nodes/:uuid/traffic` | 流量统计 |
| PUT | `/api/nodes/:uuid/tags` | 设置节点标签 |

### 文件 — 节点浏览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/nodes/:uuid/files?path=/` | 浏览节点受管文件目录 |
| GET | `/api/nodes/:uuid/download?path=...` | 获取文件/文件夹下载代理 URL |

### 文件 — 托管文件元数据 / 兼容接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/files/upload` | 初始化上传（单节点、当前目录），返回代理 URL |
| GET | `/api/files/download/:id` | 获取下载代理 URL |
| GET | `/api/files?path=/folder` | 列出虚拟目录 |
| POST | `/api/files/mkdir` | 创建虚拟目录 |
| DELETE | `/api/files/:id` | 删除文件 |
| PUT | `/api/files/:id/move` | 移动/重命名 |
| GET | `/api/files/search?q=keyword` | 搜索已托管文件（不含仅存在于节点目录树的手动文件） |

### 文件 — 代理转发（浏览器 ↔ Master ↔ Agent）

| 方法 | 路径 | 说明 |
|------|------|------|
| PUT | `/api/proxy/upload/:file_id?node=uuid` | 上传代理（流式转发到 Agent） |
| GET | `/api/proxy/download/:file_id?node=uuid` | 兼容旧文件下载代理 |
| GET | `/api/proxy/browse?node=uuid&path=...` | 节点文件/文件夹下载代理 |

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

### 告警

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/POST | `/api/alerts/rules` | 告警规则 |
| PUT/DELETE | `/api/alerts/rules/:id` | 更新/删除规则 |
| GET/POST | `/api/alerts/channels` | 通知渠道 |
| PUT | `/api/alerts/channels/:id` | 更新渠道 |

### 管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/PUT | `/api/admin/settings` | 系统设置 |
| GET | `/api/admin/audit` | 审计日志 |
| GET | `/install.sh` | Agent 一键安装脚本（动态生成） |
| GET | `/download/agent/:os/:arch` | Agent 二进制下载 |

## 通信协议

### Agent ↔ Master: WebSocket + JSON-RPC 2.0

连接地址：`ws(s)://master:8800/api/agent/ws`，认证头 `Authorization: Bearer <agent_token>`

连接保活规则：

- Master 和 Agent 都会定时发送 `ping`，并通过 `pong` 刷新读取超时
- 连接超过保活窗口未响应时会主动断开并走现有重连流程
- Agent 本地在无连接状态下发送结果会返回显式错误，不再静默当作成功

**Agent → Master:**

| 方法 | 用途 | 频率 |
|------|------|------|
| `agent.heartbeat` | 系统指标 | 每 10s |
| `agent.fileTree` | 文件树（首次全量，后续增量） | 每 10s |
| `agent.fileStored` | 文件写入确认 | 事件触发 |
| `agent.fileDeleted` | 文件删除确认 | 事件触发 |
| `agent.pingResult` | Ping 探测结果 | 事件触发 |
| `agent.commandResult` | 命令执行结果 | 事件触发 |
| `agent.verifyResult` | 文件校验结果（预留） | 事件触发 |
| `agent.replicateResult` | 副本复制结果（预留） | 事件触发 |

**Master → Agent:**

| 方法 | 用途 |
|------|------|
| `master.deleteFile` | 删除文件 |
| `master.replicateFile` | 拉取文件副本（预留） |
| `master.verifyFile` | 校验文件 SHA-256（预留） |
| `master.execCommand` | 执行远程命令 |
| `master.startPing` | 启动 Ping 探测 |
| `master.stopPing` | 停止 Ping 探测 |

### 签名算法

Master 代理请求到 Agent 时使用 HMAC-SHA256 签名，5 分钟有效期，签名绑定 HTTP 方法防跨方法重放：

```go
payload := fmt.Sprintf("%s:%s:%d", method, resourceID, expires)
// 普通下载示例: "GET:/docs/report.txt:1712345678"
// 上传示例: "PUT:file-uuid|/docs|report.txt|false:1712345678"
mac := hmac.New(sha256.New, []byte(sharedSecret))
mac.Write([]byte(payload))
sig := hex.EncodeToString(mac.Sum(nil))
```

## 数据链路

1. **监控链路**
   - Agent 每 10s 上报 `agent.heartbeat`
   - Master 写入缓存并每 60s 批量落库到 `NodeMetric`
   - 每 30min 将 4h 前原始数据压缩到 `NodeMetricCompact`（15min 粒度）
   - Dashboard 通过 `/api/ws/dashboard` 接收实时推送

2. **文件树链路**
   - Agent 首次全量、后续每 10s 增量上报 `agent.fileTree`
   - Master 在内存缓存每个节点文件树（go-cache）
   - 前端通过 `/api/nodes/:uuid/files` 浏览节点目录

3. **上传链路**
   - 前端在节点详情页调用 `/api/files/upload`（必须是在线节点）获取代理 URL
   - 浏览器 PUT 到 `/api/proxy/upload/:file_id?node=uuid&path=...&name=...&overwrite=...`
   - Master 生成签名 URL，流式转发到 Agent `PUT /api/files/:file_id`
   - Agent 以原名写入受管目录 `~/data_save/files`
   - Agent 上传完成后发 `agent.fileStored`
   - Master 将副本状态更新为 `stored`，全部完成后文件状态改为 `ready`

4. **下载链路**
   - 节点目录下载：`/api/nodes/:uuid/download?path=...` → 返回代理 URL `/api/proxy/browse?...`
   - 节点文件保持原名下载，目录以 zip 下载
   - 托管文件下载：`/api/files/download/:id` → 选择在线副本 → 返回代理 URL `/api/proxy/browse?...`
   - 浏览器通过 Master 代理从 Agent 流式拉取文件

5. **删除链路**
   - `/api/files/:id` 将文件标记为 `deleting` 并下发 `master.deleteFile`
   - Agent 删除后回报 `agent.fileDeleted`
   - GC 定时重试未完成删除，并在副本清零后清理文件记录

6. **Ping 链路**
   - 创建任务：Master 广播 `master.startPing`
   - Agent 按任务执行（Linux 下 `icmp` 为真实 ICMP，需 root 或 CAP_NET_RAW）
   - 结果上报 `agent.pingResult`
   - 删除任务：Master 广播 `master.stopPing`，Agent 按 `task_id` 停止

7. **告警链路**
   - 调度器每 10s 评估规则（cpu/mem/disk/offline）
   - 持续时间窗口内所有采样点都超阈值才触发
   - 命中后按渠道发送通知并写入审计日志
   - 冷却机制防止重复告警

## 生产部署建议

1. **妥善保管 Master secrets** — 默认会自动生成 `secrets/reg_token` 和 `secrets/signing_secret`；如果你改用环境变量覆盖，也请使用随机值并妥善保管
2. **修改 admin 密码** — 个人项目保留 `admin/admin` 作为初始入口；首次使用默认密码登录后会弹出一次提醒，仍建议尽快修改
3. **HTTPS** — 使用 nginx / caddy 反代 Master 并配置 SSL 证书（数据面通过 Master 代理，无 mixed content 问题）
4. **防火墙** — Master 开放 8800；Agent 8801 端口仅需对 Master IP 可达（不需要对公网开放）
5. **备份** — `install-master.sh` 默认把 SQLite 放在 `/var/lib/cloudnest/cloudnest.db`；Docker 默认在 `/app/data/cloudnest.db`；MySQL 部署请定期备份数据库
6. **公开地址** — 如果你走反代域名，请在环境文件里设置 `CLOUDNEST_PUBLIC_BASE_URL=https://your-domain`，让 Agent 安装脚本始终生成正确地址

### Nginx 反代示例

```nginx
server {
    listen 443 ssl;
    server_name cloudnest.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    client_max_body_size 0;  # 不限制上传大小

    location / {
        proxy_pass http://127.0.0.1:8800;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
    }

    location ~ ^/api/ws/ {
        proxy_pass http://127.0.0.1:8800;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400s;
    }

    location ~ ^/api/proxy/ {
        proxy_pass http://127.0.0.1:8800;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 600s;
        proxy_request_buffering off;  # 流式转发，不缓冲请求体
    }
}
```

## License

MIT
