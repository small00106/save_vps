# CloudNest: VPS 监控 + 分布式存储一体化系统

## 1. 项目概述

CloudNest 是一个基于 Master-Agent 架构的 VPS 统一管理平台，兼具 **Komari Monitor 级别的服务器监控** 和 **分布式文件存储** 能力。仿照 [Komari Monitor](https://github.com/komari-monitor/komari) 的架构模式，将 4-6 台 VPS 组成统一管理平台。

**设计原则**：
- 文件不分块，每个节点保存完整文件副本
- Master 管理元数据和调度，并代理浏览器到 Agent 的文件数据流
- 浏览器始终访问 Master；Master 再通过短时签名请求 Agent
- Agent 定期上报指定目录的文件树，网页端浏览下载
- 用户手动选择目标节点上传

---

## 2. 功能全景

### A. 监控功能 (继承 Komari)

| 功能 | 说明 |
|------|------|
| 实时指标 | CPU / RAM / Swap / Disk / 网络速率 / 负载 / 连接数 |
| 历史图表 | 指标时序存储, 前端可查历史趋势 |
| 流量统计 | 上传/下载累计流量, 可设流量限额 |
| 在线状态 | 节点在线/离线实时追踪 |
| 远程终端 | WebSocket Shell, 网页端直接操作 VPS |
| 远程命令 | 批量执行 Shell 命令, 返回结果 |
| Ping 探测 | ICMP/TCP/HTTP 探测任务, 多节点分布式执行 |
| 审计日志 | 所有管理操作留痕 |

### B. 存储功能 (新增)

| 功能 | 说明 |
|------|------|
| 文件目录浏览 | Agent 每 10s 上报默认数据目录 `~/data_save/files` 的文件树；可追加额外扫描目录，网页端按节点浏览 |
| 文件下载 | 节点目录和托管文件都返回 Master 代理 URL，由 Master 流式转发 Agent 数据 |
| 文件上传 | 当前实现为单目标节点上传：浏览器先向 Master 初始化，再 PUT 到 Master 代理地址，由 Master 转发写入 Agent；Agent 确认成功后 Master 提交元数据 |
| 已托管文件搜索 | `/api/files/search` 查询 `files` 元数据表；仅存在于节点目录树中的手动文件不纳入搜索 |
| 文件副本协议 | `replicate/verify` 协议与结果回执已预留，当前 Master 暂无调度入口 |

### C. 增强功能 (新增)

| 功能 | 说明 |
|------|------|
| 节点标签 | 给节点打自定义标签 (如 "日本"、"备份")；后端支持展示与更新，前端标签筛选待补 |
| 存储告警 | 磁盘使用超阈值 → 触发通知 (Telegram/Webhook/Email/Bark) |
| 传输限速 | Agent 可配置上传/下载带宽上限, 避免影响 VPS 其他服务 |
| 多通知渠道 | Telegram / Webhook / Email / Bark / ServerChan |

---

## 3. 系统架构

```
                 ┌───────────────────────────────┐
                 │      Master Server (8800)      │
                 │  监控 / 存储调度 / 告警 / 面板   │
                 │  SQLite (GORM)                 │
                 └────┬──────┬──────┬─────────────┘
                      │ WS   │ WS   │ WS  ← 控制面 (JSON-RPC 2.0)
               ┌──────┘      │      └──────┐
               ▼             ▼             ▼
         ┌──────────┐  ┌──────────┐  ┌──────────┐
         │ Agent A  │  │ Agent B  │  │ Agent C  │  ... (4-6台)
         │ VPS-东京 │  │ VPS-美西 │  │ VPS-香港 │
         │ :8801    │  │ :8801    │  │ :8801    │
         └──────────┘  └──────────┘  └──────────┘
                 ┌────────────────────────┐
                 │       用户 / 浏览器      │
                 └──────────┬─────────────┘
                            │ HTTPS
                            ▼
                 ┌────────────────────────┐
                 │         Master          │  ← 数据面代理 + 短时签名
                 └──────────┬─────────────┘
                            │ HTTP/HTTPS
                            ▼
                        Agent 数据面
```

---

## 4. 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.24+ / Gin / GORM / gorilla/websocket / Cobra |
| 前端 | React + TypeScript + Tailwind CSS + Recharts |
| 数据库 | SQLite (默认) / MySQL |
| 缓存 | patrickmn/go-cache |
| 部署 | GitHub Releases / systemd / GHCR / Docker Compose |

---

## 5. 通信协议

### 5.1 Agent ↔ Master: WebSocket + JSON-RPC 2.0 (控制面)

```
连接: wss://master:8800/api/agent/ws
认证: Authorization: Bearer <agent_token>
```

连接约束：

- Master 与 Agent 双向使用 ping/pong 保活，并设置读取超时，避免僵尸连接长期滞留
- Hub 广播时先复制连接快照，再在锁外逐个发送，避免慢 Agent 阻塞整个控制面
- Agent 在本地 WebSocket 未连接时发送结果会返回显式错误，不再把断链误判为发送成功

**Agent → Master:**

| 方法 | 用途 | 频率 |
|------|------|------|
| `agent.heartbeat` | 系统指标 (CPU/RAM/Swap/Disk/网络/负载/连接数/进程数/Uptime) | 每 10s |
| `agent.fileTree` | 文件树 (首次全量, 后续增量；增量字段为 `added/removed`) | 每 10s |
| `agent.fileStored` | 文件写入完成确认；Master 在这里提交 `File/FileReplica` 元数据 | 事件触发 |
| `agent.fileDeleted` | 文件删除完成确认 | 事件触发 |
| `agent.pingResult` | Ping 探测结果 | 事件触发 |
| `agent.commandResult` | 远程命令执行结果 | 事件触发 |

**Master → Agent:**

| 方法 | 用途 |
|------|------|
| `master.deleteFile` | 删除文件 |
| `master.replicateFile` | 从另一节点拉取文件副本 |
| `master.verifyFile` | 校验文件 SHA-256 |
| `master.execCommand` | 执行远程命令 |
| `master.startPing` | 启动 Ping 探测任务 |
| `master.stopPing` | 停止 Ping 探测任务 |

### 5.2 用户 ↔ Master: REST API + WebSocket

```
# 认证
POST   /api/auth/login                  # 按 IP 登录失败限流: 5 次/5 分钟，第 6 次 429
POST   /api/auth/logout

# 存储 - 文件管理
POST   /api/files/upload                    # 初始化上传 (当前实现为单节点), 返回 Master 代理上传地址
GET    /api/files/download/:id              # 获取 Master 下载代理地址
GET    /api/files?path=/folder              # 列出虚拟目录
POST   /api/files/mkdir                     # 创建目录
DELETE /api/files/:id                       # 删除文件
PUT    /api/files/:id/move                  # 移动/重命名

# 存储 - 节点文件浏览 (基于心跳上报的文件树)
GET    /api/nodes/:uuid/files?path=/data    # 浏览某节点的真实文件目录
GET    /api/nodes/:uuid/download?path=...   # 获取节点目录下载的 Master 代理地址
GET    /api/files/search?q=keyword          # 搜索已托管文件元数据

# 监控
GET    /api/nodes                           # 节点列表 + 实时状态
GET    /api/nodes/:uuid                     # 节点详情
GET    /api/nodes/:uuid/metrics             # 历史指标
GET    /api/nodes/:uuid/traffic             # 流量统计
WS     /api/ws/dashboard                    # 实时推送监控数据到前端

# 远程操作
POST   /api/nodes/:uuid/exec               # 执行远程命令
GET    /api/commands/:id                   # 查询命令结果
WS     /api/ws/terminal/:uuid              # WebSocket 终端

# Ping 探测
GET    /api/ping/tasks                      # Ping 任务列表
POST   /api/ping/tasks                      # 创建 Ping 任务
GET    /api/ping/tasks/:id/results          # Ping 结果
DELETE /api/ping/tasks/:id                  # 删除 Ping 任务

# 告警与通知
GET    /api/alerts/rules                    # 告警规则列表
POST   /api/alerts/rules                    # 创建告警规则
PUT    /api/alerts/rules/:id               # 更新告警规则
DELETE /api/alerts/rules/:id               # 删除告警规则
GET    /api/alerts/channels                 # 通知渠道配置
POST   /api/alerts/channels                 # 创建通知渠道
PUT    /api/alerts/channels/:id             # 更新通知渠道

# 管理
GET    /api/admin/settings                  # 系统设置
PUT    /api/admin/settings                  # 更新设置
GET    /api/admin/audit                     # 审计日志
PUT    /api/nodes/:uuid/tags               # 设置节点标签
```

前端认证行为：

- `AuthProvider` 启动时仍会执行一次 `/api/auth/me`
- 后续任意业务 API 返回 `401` 时，前端统一清理认证态并跳回 `/login`
- 登录接口自身的凭证错误不触发这条全局退登逻辑

### 5.3 Master ↔ Agent: 签名 URL + HTTP 代理 (数据面)

```
PUT  https://agent:8801/api/files/:file_id?expires=<ts>&sig=<hmac>   # 上传
GET  https://agent:8801/api/files/:file_id?expires=<ts>&sig=<hmac>   # 下载
GET  https://agent:8801/api/browse?path=...&expires=<ts>&sig=<hmac>  # 按路径下载
POST https://agent:8801/api/files/move?from=...&to=...&expires=<ts>&sig=<hmac>  # 单副本文件移动
```

这些 URL 由 Master 在代理请求 Agent 时内部生成，浏览器不直接持有 Agent 的签名 URL。签名算法为 HMAC-SHA256，5 分钟有效期。

控制面/数据面 HTTP 可靠性约束：

- 文件代理与复制链路使用带固定超时的 HTTP 客户端，避免半开连接无限挂起
- Agent 注册请求使用超时和 context 控制，启动阶段不会永久卡住
- 通知发送统一检查非 `2xx` 响应并按失败处理

```go
payload := fmt.Sprintf("%s:%s:%d", strings.ToUpper(method), resourceID, expires)
mac := hmac.New(sha256.New, []byte(sharedSecret))
mac.Write([]byte(payload))
sig := hex.EncodeToString(mac.Sum(nil))
```

密钥来源规则：

- Master 启动时优先读取 `CLOUDNEST_REG_TOKEN` 和 `CLOUDNEST_SIGNING_SECRET`
- 如果环境变量为空，则读取数据目录下的 `secrets/reg_token` 和 `secrets/signing_secret`
- 两者都没有时，Master 首次启动自动生成并持久化
- Agent 运行时优先读取 `CLOUDNEST_SIGNING_SECRET_FILE`，其次兼容 `CLOUDNEST_SIGNING_SECRET`；缺失或仍使用旧公开默认值时直接拒绝启动

部署与分发规则：

- Release + systemd 是默认主路径：
  - `install-master.sh` 从 Release 下载 `cloudnest-master-linux-{amd64,arm64}.tar.gz`，默认安装到 `/opt/cloudnest`
  - 安装脚本会先下载同版本 `checksums.txt` 校验 SHA-256，再启动 systemd 并轮询本地 `/healthz`
  - 对个人项目来说，这条路径比 Docker 更接近“一键部署”
- Docker 是可选路径：
  - 默认通过 `docker-compose.yml` 拉取 `ghcr.io/small00106/save_vps:<tag>`
  - `.env.example` 提供 `CLOUDNEST_IMAGE_TAG`、`CLOUDNEST_PORT`、`CLOUDNEST_DB_TYPE`、`CLOUDNEST_DB_DSN`、`CLOUDNEST_PUBLIC_BASE_URL`
  - 如需本地源码构建镜像，使用 `docker-compose.build.yml` 作为覆盖层
- 发布 workflow 同时产出两类正式产物：
  - GitHub Release 资产：Master tarball、Agent 二进制、`checksums.txt`
  - GHCR 多架构镜像：`ghcr.io/small00106/save_vps:<tag>` 与 `latest`
- 正式发布产物统一放在 GitHub Releases，不再提交编译后二进制到源码仓库
- 发布包内自带 `data/binaries/cloudnest-agent-linux-amd64` 与 `cloudnest-agent-linux-arm64`，Master 运行后继续通过 `/download/agent/:os/:arch` 分发 Agent
- `GET /install.sh` 生成安装脚本时，优先读取 `CLOUDNEST_PUBLIC_BASE_URL`；未配置时才回退到当前请求的 `scheme + host`
- Master 公开 `GET /healthz`，用于 Docker `HEALTHCHECK`、安装脚本就绪探测和 smoke test

上传/移动一致性约束：

- `InitUpload` 只做冲突检查并返回代理上传地址，不预写 `uploading/pending` 记录
- Agent 上传先写同目录临时文件，完整写入后再替换正式文件，避免覆盖上传把旧文件截断
- `MoveFile` 目前只支持单个在线 `stored` 副本；Agent 真实移动成功后，Master 才更新 `files` 和 `file_replicas.store_path`

---

## 6. 数据模型

```go
// ==================== 节点相关 ====================

// Node — 存储/监控节点
type Node struct {
    UUID      string   `gorm:"primaryKey;size:36"`
    Token     string   `gorm:"size:64;uniqueIndex"`
    Hostname  string
    IP        string
    Port      int      `gorm:"default:8801"`
    Region    string
    Tags      string   // JSON array: ["日本","备份","媒体"]
    OS        string   // 操作系统
    Arch      string   // CPU 架构
    CPUModel  string   // CPU 型号
    CPUCores  int
    DiskTotal int64
    DiskUsed  int64
    RAMTotal  int64
    Status    string   `gorm:"default:online"` // online/offline/draining
    Version   string   // Agent 版本
    RateLimit int64    // 传输限速 bytes/s, 0=不限
    LastSeen  time.Time
    CreatedAt time.Time
    UpdatedAt time.Time
}

// NodeMetric — 节点实时指标 (热数据, 保留 4h)
type NodeMetric struct {
    ID           uint    `gorm:"primaryKey"`
    NodeUUID     string  `gorm:"index"`
    CPUPercent   float64
    MemPercent   float64
    SwapUsed     int64
    SwapTotal    int64
    DiskPercent  float64
    Load1        float64
    Load5        float64
    Load15       float64
    NetInSpeed   int64   // bytes/s
    NetOutSpeed  int64   // bytes/s
    NetInTotal   int64   // 累计字节
    NetOutTotal  int64   // 累计字节
    TCPConns     int
    UDPConns     int
    ProcessCount int
    Uptime       int64   // 秒
    Timestamp    time.Time `gorm:"index"`
}

// NodeMetricCompact — 压缩后的长期指标 (15min 粒度)
type NodeMetricCompact struct {
    ID           uint    `gorm:"primaryKey"`
    NodeUUID     string  `gorm:"index"`
    CPUPercent   float64
    MemPercent   float64
    DiskPercent  float64
    NetInSpeed   int64
    NetOutSpeed  int64
    BucketTime   time.Time `gorm:"index"` // 15min 对齐
}

// ==================== 存储相关 ====================

// File — 文件元数据 (通过上传接口管理的文件)
type File struct {
    FileID    string `gorm:"primaryKey;size:36"`
    Name      string
    Path      string `gorm:"index"` // 虚拟路径
    Size      int64
    MimeType  string
    Checksum  string
    IsDir     bool   `gorm:"default:false"`
    Status    string `gorm:"default:uploading"` // uploading/ready/deleting
    CreatedAt time.Time
    DeletedAt gorm.DeletedAt `gorm:"index"`
}

// FileReplica — 文件副本位置
type FileReplica struct {
    ID         uint   `gorm:"primaryKey"`
    FileID     string `gorm:"index"`
    NodeUUID   string `gorm:"index"`
    Status     string // pending/stored/verified/lost
    StorePath  string
    VerifiedAt time.Time
}

// ==================== 认证 ====================

// User — 管理员 (单用户)
type User struct {
    ID           uint   `gorm:"primaryKey"`
    Username     string `gorm:"uniqueIndex"`
    PasswordHash string
    CreatedAt    time.Time
}

// Session — 登录会话
type Session struct {
    Token     string `gorm:"primaryKey;size:64"`
    UserID    uint
    ExpiresAt time.Time
}

// ==================== 告警 ====================

// AlertRule — 告警规则
type AlertRule struct {
    ID          uint   `gorm:"primaryKey"`
    Name        string
    NodeUUID    string `gorm:"index"` // 空=全部节点
    Metric      string // cpu/mem/disk/offline
    Operator    string // gt/lt
    Threshold   float64
    Duration    int    // 持续秒数
    ChannelID   uint   // 通知渠道
    Enabled     bool   `gorm:"default:true"`
    LastFiredAt time.Time
    CreatedAt   time.Time
}

// AlertChannel — 通知渠道
type AlertChannel struct {
    ID     uint   `gorm:"primaryKey"`
    Name   string
    Type   string // telegram/webhook/email/bark/serverchan
    Config string // JSON 配置 (token, url, etc.)
}

// ==================== Ping 探测 ====================

// PingTask — Ping 探测任务
type PingTask struct {
    ID       uint   `gorm:"primaryKey"`
    Name     string
    Type     string // icmp/tcp/http
    Target   string // IP/域名/URL
    Interval int    // 秒
    Enabled  bool   `gorm:"default:true"`
}

// PingResult — Ping 结果
type PingResult struct {
    ID         uint    `gorm:"primaryKey"`
    TaskID     uint    `gorm:"index"`
    NodeUUID   string  `gorm:"index"`
    Latency    float64 // ms
    Success    bool
    Timestamp  time.Time `gorm:"index"`
}

// ==================== 远程操作 ====================

// CommandTask — 远程命令执行记录
type CommandTask struct {
    ID        uint   `gorm:"primaryKey"`
    NodeUUID  string `gorm:"index"`
    Command   string
    Output    string
    ExitCode  int
    Status    string // pending/running/done/failed
    CreatedAt time.Time
}

// AuditLog — 审计日志
type AuditLog struct {
    ID         uint   `gorm:"primaryKey"`
    Action     string
    Actor      string
    Status     string
    TargetType string
    TargetID   string
    NodeUUID   string
    Detail     string
    IP         string
    CreatedAt  time.Time
}

// Setting — 系统设置键值
type Setting struct {
    Key   string `gorm:"primaryKey;size:64"`
    Value string
}
```

---

## 7. 核心数据流

### 7.1 监控数据流

```
Agent 每10s → WS agent.heartbeat → Master:
  CPU/RAM/Swap/Disk/网络/负载/连接数/进程数/Uptime
  → go-cache 缓存 → 每60s 刷入 NodeMetric 表
  → 每30min 压缩为 NodeMetricCompact (15min粒度)
  → WS 推送到前端 dashboard
```

### 7.2 文件目录数据流

```
Agent 每10s → WS agent.fileTree → Master:
  扫描默认数据目录 `${data_root}/files`，并可按配置追加 scan_dirs
  → 首次全量: {full: true, entries: [...]}
  → 后续增量: {full: false, added: [...], removed: [...]}
  → Master 缓存在 go-cache (key: filetree:<uuid>)
  → 前端 GET /api/nodes/:uuid/files?path= 查询
```

### 7.3 上传数据流

```
用户 → Master: POST /api/files/upload { name, size, path, node_uuid, overwrite }
  → Master 校验在线节点与重名冲突
  → 返回 Master 代理上传地址 /api/proxy/upload/:file_id?node=...
用户 → Master: PUT /api/proxy/upload/:file_id?node=...&path=...&name=...&overwrite=...
  → Master 生成 Agent 签名 URL，流式转发到 Agent PUT /api/files/:file_id
  → Agent 先写同目录临时文件，再替换正式文件
Agent → Master (WS): agent.fileStored
  → Master 更新 File / FileReplica 元数据并刷新文件树缓存
```

### 7.4 下载数据流 (两种模式)

**模式1: 从节点文件浏览器下载 (浏览 Agent 真实目录)**
```
用户浏览 /api/nodes/A/files?path=/data/videos
  → 看到文件列表 (来自 Agent 上报的文件树缓存)
  → 点击下载
  → GET /api/nodes/A/download?path=/data/videos/movie.mp4
  → Master 返回代理 URL /api/proxy/browse?node=A&path=...
  → 浏览器向 Master 发起下载
  → Master 签名并流式拉取 Agent A 数据
```

**模式2: 从虚拟文件管理器下载 (Master 管理的文件)**
```
用户浏览 /api/files?path=/photos
  → 看到通过上传接口管理的文件
  → 点击下载
  → GET /api/files/download/:file_id
  → Master 选一个在线 stored 副本, 返回代理 URL /api/proxy/browse?node=...
  → 浏览器向 Master 发起下载
  → Master 再从对应 Agent 流式拉取文件
```

### 7.5 告警数据流

```
Agent 心跳 → Master 检查告警规则:
  if node.disk_percent > rule.threshold 持续 rule.duration:
    → 查找对应 AlertChannel
    → 发送通知 (Telegram/Webhook/Email/Bark)
    → 记录 AuditLog
```

---

## 8. 项目目录结构

### Master (`cloudnest/`)
```
cloudnest/
├── main.go
├── cmd/
│   ├── root.go
│   └── server.go
├── internal/
│   ├── api/
│   │   ├── agent/                # Agent 注册 + WebSocket 控制面
│   │   ├── alerts/               # 告警规则 + 通知渠道
│   │   ├── admin/                # 系统设置 + 审计日志查询
│   │   ├── auth/                 # 登录、会话、默认密码提醒
│   │   ├── command/              # 远程命令执行
│   │   ├── files/                # 文件元数据、节点浏览、Master 代理
│   │   ├── nodes/                # 节点列表、详情、指标、流量、标签
│   │   ├── ping/                 # Ping 探测任务
│   │   └── terminal/             # 远程终端 WS 代理
│   ├── audit/                    # 审计日志写入与上下文提取
│   ├── cache/                    # 指标缓冲与文件树/实时指标缓存
│   ├── database/                 # GORM 初始化与模型
│   ├── notify/                   # Telegram/Webhook/Email/Bark/ServerChan
│   ├── scheduler/                # 指标落库、健康检查、压缩、告警、GC
│   ├── server/                   # Gin 路由、中间件、安装脚本生成
│   ├── transfer/                 # HMAC 签名 URL
│   └── ws/                       # Agent Hub + Dashboard 推送
├── public/                       # go:embed 前端静态文件
└── go.mod
```

### Agent (`cloudnest-agent/`)
```
cloudnest-agent/
├── main.go
├── cmd/
│   ├── root.go
│   ├── run.go
│   └── register.go
├── internal/
│   ├── agent/
│   │   ├── agent.go              # 主循环、回调与 Master 指令分发
│   │   └── config.go             # 本地配置 (port / scan_dirs / rate_limit)
│   ├── reporter/                 # 系统指标采集 + 文件树扫描
│   ├── server/                   # HTTP 数据面、签名验证、限速、上传/下载/移动
│   ├── storage/                  # 数据目录、路径校验、文件落盘
│   ├── terminal/                 # 远程终端
│   ├── testutil/                 # 测试辅助
│   └── ws/                       # WS 客户端 + 指数退避重连
└── go.mod
```

### 前端 (`cloudnest-web/`)
```
cloudnest-web/
├── src/
│   ├── api/                      # 类型化 API 客户端
│   ├── assets/                   # 静态资源
│   ├── components/               # Layout、默认密码提醒等 UI 组件
│   ├── contexts/                 # 偏好设置上下文
│   ├── hooks/                    # useAuth / useWebSocket
│   ├── i18n/                     # 中英文本地化
│   ├── lib/                      # 通用辅助代码
│   ├── pages/
│   │   ├── Dashboard.tsx         # 总览: 所有节点状态卡片
│   │   ├── NodeDetail.tsx        # 单节点: 指标图表 + 文件浏览
│   │   ├── FileBrowser.tsx       # 已托管文件搜索与下载
│   │   ├── Terminal.tsx          # 远程终端
│   │   ├── PingTasks.tsx         # Ping 探测
│   │   ├── Alerts.tsx            # 告警规则 + 通知渠道
│   │   ├── AuditLog.tsx          # 审计日志
│   │   ├── LoginPage.tsx         # 登录页
│   │   └── SettingsPage.tsx      # 设置与密码修改
│   └── utils/                    # 下载等前端辅助逻辑
└── package.json
```

---

## 9. 实施顺序

| 阶段 | 内容 | 交付 |
|------|------|------|
| 1. 基础框架 | 脚手架, Cobra CLI, 全部数据模型, Agent 注册, WS Hub | Master+Agent 能连接 |
| 2. 监控核心 | 心跳上报全部指标, 指标存储+压缩, 前端 Dashboard + 节点详情 + 图表 | 监控可用 |
| 3. 存储核心 | 文件目录扫描+上报, 节点文件浏览, 上传/下载, 已托管文件搜索 | 存储可用 |
| 4. 远程操作 | 远程终端, 远程命令, Ping 探测 | 运维功能完整 |
| 5. 告警通知 | 告警规则, 通知渠道 (Telegram/Webhook/Email/Bark), 存储告警 | 告警可用 |
| 6. 增强 | 节点标签, 传输限速, 审计日志, 副本复制/校验协议预留 | 功能完善 |
| 7. 部署 | GitHub Releases, systemd, install.sh, GHCR 镜像, Docker Compose | 一键部署 |

---

## 10. 验证方案

1. `docker compose up -d` 启动 Master，并通过 `/healthz` 确认服务就绪
2. 在 1-2 台 Linux 主机上执行 `install.sh`，或手动运行 `cloudnest-agent register/run`
3. Agent 注册成功后，Dashboard 显示节点在线并持续刷新实时指标
4. 节点详情页查看 CPU/RAM/Disk 历史图表与流量统计
5. 在节点详情页浏览 `Files`，验证目录列表来自 Agent 文件树缓存
6. 上传文件到某个节点当前目录，确认 Agent 本地落盘且前端可下载
7. 在 `/files` 页搜索已托管文件，确认只能搜到已入库文件元数据
8. 打开远程终端并执行命令；同时验证远程命令执行与结果查询
9. 配置告警规则与通知渠道，触发一次 Webhook / Telegram / Bark 等通知
10. 停止一个 Agent，确认约 30 秒后节点标记 offline，并可触发离线告警

补充：

- CI / Release 路径已有 `scripts/smoke-test-release.sh` 与 `scripts/smoke-test-docker.sh`
- 当前 `docker-compose.yml` 只负责拉起 Master；多 Agent 联调需额外启动 Agent 进程或单独主机
