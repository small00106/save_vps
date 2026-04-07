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
| 文件目录浏览 | Agent 定期上报指定目录的文件树, 网页端按节点浏览 |
| 文件下载 | 网页端选中文件 → Master 生成短时签名并代理 Agent 下载 |
| 文件上传 | 用户选择目标节点 → 浏览器上传到 Master, 再由 Master 代理写入 Agent；Agent 确认写入成功后 Master 才提交元数据 |
| 跨节点文件搜索 | 在 Master 缓存的所有节点文件树中搜索文件名 |
| 文件副本管理 | 手动/自动将文件复制到其他节点 |

### C. 增强功能 (新增)

| 功能 | 说明 |
|------|------|
| 节点标签 | 给节点打自定义标签 (如 "日本"、"备份"), 前端按标签筛选 |
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
| 前端 | React + TypeScript + Tailwind + shadcn/ui |
| 数据库 | SQLite (默认) / MySQL |
| 缓存 | patrickmn/go-cache |
| 部署 | GitHub Releases / systemd / Docker |

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
| `agent.fileTree` | 指定目录的文件树 (路径/大小/修改时间) | 每 60s (首次全量, 后续增量) |
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
| `master.openTerminal` | 建立终端会话 |

### 5.2 用户 ↔ Master: REST API + WebSocket

```
# 认证
POST   /api/auth/login                  # 按 IP 登录失败限流: 5 次/5 分钟，第 6 次 429
POST   /api/auth/logout

# 存储 - 文件管理
POST   /api/files/upload                    # 初始化上传 (用户选节点), 返回签名URL
GET    /api/files/download/:id              # 获取下载签名URL
GET    /api/files?path=/folder              # 列出虚拟目录
POST   /api/files/mkdir                     # 创建目录
DELETE /api/files/:id                       # 删除文件
PUT    /api/files/:id/move                  # 移动/重命名

# 存储 - 节点文件浏览 (基于心跳上报的文件树)
GET    /api/nodes/:uuid/files?path=/data    # 浏览某节点的真实文件目录
GET    /api/nodes/:uuid/download?path=...   # 从某节点下载指定文件 (返回签名URL)
GET    /api/files/search?q=keyword          # 跨节点文件搜索

# 监控
GET    /api/nodes                           # 节点列表 + 实时状态
GET    /api/nodes/:uuid                     # 节点详情
GET    /api/nodes/:uuid/metrics             # 历史指标
GET    /api/nodes/:uuid/traffic             # 流量统计
WS     /api/ws/dashboard                    # 实时推送监控数据到前端

# 远程操作
POST   /api/nodes/:uuid/exec               # 执行远程命令
WS     /api/ws/terminal/:uuid              # WebSocket 终端

# Ping 探测
GET    /api/ping/tasks                      # Ping 任务列表
POST   /api/ping/tasks                      # 创建 Ping 任务
GET    /api/ping/tasks/:id/results          # Ping 结果

# 告警与通知
GET    /api/alerts/rules                    # 告警规则列表
POST   /api/alerts/rules                    # 创建告警规则
PUT    /api/alerts/rules/:id               # 更新告警规则
GET    /api/alerts/channels                 # 通知渠道配置
PUT    /api/alerts/channels                 # 更新通知渠道

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
- Agent 运行时必须显式提供 `CLOUDNEST_SIGNING_SECRET`，缺失或仍使用旧公开默认值时直接拒绝启动

部署与分发规则：

- 正式发布产物统一放在 GitHub Releases，不再提交编译后二进制到源码仓库
- `install-master.sh` 从 Release 下载 `cloudnest-master-linux-{amd64,arm64}.tar.gz`，默认安装到 `/opt/cloudnest`
- 发布包内自带 `data/binaries/cloudnest-agent-linux-amd64` 与 `cloudnest-agent-linux-arm64`，Master 运行后继续通过 `/download/agent/:os/:arch` 分发 Agent
- `GET /install.sh` 生成安装脚本时，优先读取 `CLOUDNEST_PUBLIC_BASE_URL`；未配置时才回退到当前请求的 `scheme + host`

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
    ID        uint   `gorm:"primaryKey"`
    Action    string
    Detail    string
    IP        string
    CreatedAt time.Time
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
Agent 每60s → WS agent.fileTree → Master:
  扫描配置的根目录 (如 /data)
  → 首次全量: [{path, size, modTime, isDir}, ...]
  → 后续增量: {added: [...], removed: [...], modified: [...]}
  → Master 缓存在内存 (nodeFileTrees map[uuid][]FileEntry)
  → 前端 GET /api/nodes/:uuid/files?path= 查询
```

### 7.3 上传数据流

```
用户 → Master: POST /api/files/upload { name, size, node_uuids: ["A","B"] }
  (用户自选目标节点)
Master → 用户: { file_id, targets: [{ node, url (签名) }] }
用户 → Agent A: PUT 完整文件 (直传)
用户 → Agent B: PUT 完整文件 (直传, 并行)
Agent → Master (WS): fileStored 确认
```

### 7.4 下载数据流 (两种模式)

**模式1: 从节点文件浏览器下载 (浏览 Agent 真实目录)**
```
用户浏览 /api/nodes/A/files?path=/data/videos
  → 看到文件列表 (来自 Agent 上报的文件树缓存)
  → 点击下载
  → GET /api/nodes/A/download?path=/data/videos/movie.mp4
  → Master 返回签名 URL
  → 浏览器直接从 Agent A 下载
```

**模式2: 从虚拟文件管理器下载 (Master 管理的文件)**
```
用户浏览 /api/files?path=/photos
  → 看到通过上传接口管理的文件
  → 点击下载
  → GET /api/files/download/:file_id
  → Master 选一个在线节点, 返回签名 URL
  → 浏览器直接从 Agent 下载
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
│   ├── server/
│   │   ├── server.go             # Gin 路由 + 中间件
│   │   └── middleware/auth.go
│   ├── api/
│   │   ├── agent/                # Agent 注册, WS, 心跳
│   │   │   ├── register.go
│   │   │   ├── websocket.go
│   │   │   └── heartbeat.go
│   │   ├── files/                # 文件上传/下载/CRUD/搜索
│   │   │   ├── upload.go
│   │   │   ├── download.go
│   │   │   ├── browse.go        # 节点文件浏览
│   │   │   ├── search.go        # 跨节点搜索
│   │   │   └── manage.go
│   │   ├── nodes/                # 节点列表/详情/指标/标签
│   │   ├── monitor/              # 实时监控 WS 推送
│   │   ├── terminal/             # 远程终端 WS
│   │   ├── command/              # 远程命令执行
│   │   ├── ping/                 # Ping 探测任务
│   │   ├── alerts/               # 告警规则 + 通知渠道
│   │   ├── admin/                # 系统设置 + 审计日志
│   │   └── auth/                 # 登录/会话
│   ├── database/
│   │   ├── dbcore/init.go        # GORM 初始化 (sync.Once)
│   │   └── models/               # 所有数据模型
│   ├── ws/
│   │   ├── hub.go                # Agent WS 连接管理
│   │   ├── dashboard.go          # 前端 WS 推送
│   │   └── safeconn.go
│   ├── placement/selector.go     # 节点选择算法
│   ├── transfer/signer.go        # HMAC 签名 URL
│   ├── notify/                   # 通知发送器
│   │   ├── sender.go             # 统一接口
│   │   ├── telegram.go
│   │   ├── webhook.go
│   │   ├── email.go
│   │   └── bark.go
│   ├── scheduler/
│   │   ├── health.go             # 节点健康检查
│   │   ├── alerts.go             # 告警规则评估
│   │   ├── replication.go        # 补副本
│   │   ├── integrity.go          # 文件校验
│   │   ├── compaction.go         # 指标压缩
│   │   └── gc.go                 # 垃圾回收
│   └── cache/
│       ├── cache.go              # go-cache 指标缓冲
│       └── filetree.go           # 节点文件树缓存
├── public/dist/                  # go:embed 前端
├── Dockerfile
├── docker-compose.yml
├── install-master.sh
├── release/
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
│   │   ├── agent.go              # 主循环 + 优雅关闭
│   │   └── config.go             # 本地配置 (含 rate_limit, scan_dirs)
│   ├── ws/
│   │   ├── client.go             # WS 客户端 + 指数退避重连
│   │   └── handler.go            # 处理 Master 下发的指令
│   ├── storage/
│   │   ├── engine.go             # 文件读写删
│   │   ├── scanner.go            # 目录扫描 + 增量 diff
│   │   └── integrity.go          # SHA-256 校验
│   ├── server/
│   │   ├── server.go             # Gin HTTP 数据面
│   │   ├── upload.go             # PUT /api/files/:id
│   │   ├── download.go           # GET /api/files/:id + browse
│   │   ├── auth.go               # 签名 URL 验证
│   │   └── ratelimit.go          # 传输限速中间件
│   ├── terminal/
│   │   └── shell.go              # 终端 WS 处理
│   └── reporter/
│       ├── metrics.go            # 系统指标采集 (CPU/RAM/Disk/Net/Load)
│       └── filetree.go           # 文件树上报
├── Dockerfile
├── install.sh
└── go.mod
```

### 前端 (`cloudnest-web/`)
```
cloudnest-web/
├── src/
│   ├── pages/
│   │   ├── Dashboard.tsx         # 总览: 所有节点状态卡片
│   │   ├── NodeDetail.tsx        # 单节点: 指标图表 + 文件浏览
│   │   ├── FileBrowser.tsx       # 虚拟文件管理器
│   │   ├── FileSearch.tsx        # 跨节点搜索
│   │   ├── Terminal.tsx          # 远程终端
│   │   ├── PingTasks.tsx         # Ping 探测
│   │   ├── Alerts.tsx            # 告警规则 + 通知渠道
│   │   ├── AuditLog.tsx          # 审计日志
│   │   ├── Login.tsx
│   │   └── Settings.tsx
│   ├── components/
│   │   ├── NodeCard.tsx          # 节点状态卡片 (CPU/RAM/Disk/标签)
│   │   ├── MetricsChart.tsx      # 指标折线图
│   │   ├── FileList.tsx          # 文件/目录列表
│   │   ├── UploadDialog.tsx      # 上传对话框 (选节点)
│   │   ├── SearchBar.tsx         # 文件搜索栏
│   │   ├── TagFilter.tsx         # 标签筛选
│   │   ├── Breadcrumb.tsx
│   │   └── XTerm.tsx             # 终端组件 (xterm.js)
│   ├── hooks/
│   │   ├── useWebSocket.ts       # 实时监控数据
│   │   ├── useUpload.ts
│   │   └── useAuth.ts
│   └── api/client.ts
└── package.json
```

---

## 9. 实施顺序

| 阶段 | 内容 | 交付 |
|------|------|------|
| 1. 基础框架 | 脚手架, Cobra CLI, 全部数据模型, Agent 注册, WS Hub | Master+Agent 能连接 |
| 2. 监控核心 | 心跳上报全部指标, 指标存储+压缩, 前端 Dashboard + 节点详情 + 图表 | 监控可用 |
| 3. 存储核心 | 文件目录扫描+上报, 节点文件浏览, 上传/下载, 跨节点搜索 | 存储可用 |
| 4. 远程操作 | 远程终端, 远程命令, Ping 探测 | 运维功能完整 |
| 5. 告警通知 | 告警规则, 通知渠道 (Telegram/Webhook/Email/Bark), 存储告警 | 告警可用 |
| 6. 增强 | 节点标签, 传输限速, 副本管理, 审计日志 | 功能完善 |
| 7. 部署 | GitHub Releases, systemd, install.sh, Docker | 一键部署 |

---

## 10. 验证方案

1. `docker-compose up` 启动 Master + 2 Agent
2. Agent 自动注册 → Dashboard 显示两节点在线, 实时指标刷新
3. 节点详情页查看 CPU/RAM/Disk 历史图表
4. 浏览 Agent A 的 /data 目录 → 看到文件列表 → 点击下载
5. 上传文件到 Agent A → 验证 Agent A 本地有文件
6. 搜索文件名 → 跨节点返回结果
7. 打开远程终端 → 执行命令
8. 配置告警: 磁盘 > 80% → 收到 Webhook 通知
9. 停止一个 Agent → 30s 后标记 offline → 触发离线告警
10. 给节点打标签 → 前端按标签筛选
