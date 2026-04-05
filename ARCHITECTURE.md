# CloudNest 架构说明

> **范围声明**：本文描述的是 `save_vps` 仓库**当前代码已经实现**的架构，而非最早的目标设计稿。凡是未完全落地、仅有数据结构、或前后端尚未接线的能力，均会明确标注。

## 1. 系统定位

CloudNest 当前是一个基于 Master-Agent 的 VPS 管理系统，主要覆盖三类能力：

- 节点监控：心跳、实时状态、历史指标、流量统计
- 文件能力：节点真实目录浏览、单节点上传、托管文件元数据、托管文件搜索与下载
- 运维能力：远程命令、Web 终端、Ping 任务、告警与通知、基础审计/设置

当前实现状态可以概括为：

| 状态         | 能力                                                                                                                                   |
| ------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| 已实现       | Agent 注册、心跳、节点上下线、Dashboard 实时推送、节点详情图表、节点目录浏览/下载、单节点上传、托管文件搜索、远程命令、Web 终端、Ping 任务、告警规则/通知渠道、设置页、登录认证 |
| 部分实现     | 文件副本元数据、`verify`/`replicate` 结果处理、文件管理 API（部分前端未使用）、节点标签编辑                                             |
| 未完整落地   | 自动副本调度、多节点并行上传、前端标签筛选、完整审计留痕、独立的托管文件管理器 UI                                                       |

---

## 2. 总体架构

当前代码的真实拓扑如下：

```text
Browser
  │
  │ REST + WebSocket
  ▼
Master (`cloudnest`)
  ├─ `/api/*` 业务 API
  ├─ `/api/ws/dashboard` 实时监控推送
  ├─ `/api/ws/terminal/:uuid` 终端桥接
  ├─ `/api/proxy/*` 文件上传/下载代理
  ├─ `/install.sh` Agent 安装脚本
  ├─ `/download/agent/:os/:arch` Agent 二进制分发
  └─ 嵌入式前端静态资源（SPA）
  │
  ├─ WebSocket 控制面：`/api/agent/ws`
  ▼
Agent (`cloudnest-agent`)
  ├─ 心跳与文件树上报
  ├─ 执行命令 / Ping / 删除文件
  ├─ 数据面 HTTP：`/api/files/:id`、`/api/browse`
  └─ 数据面 WebSocket：`/api/terminal`
```

这里有两个关键事实：

- 当前浏览器并不会直接访问 Agent。上传、下载、目录下载都先打到 Master 的 `/api/proxy/*`，再由 Master 生成签名请求并转发到 Agent。
- 终端也不是通过 Master-Agent 的 JSON-RPC 建立，而是 Master 先连接 Agent 的签名 WebSocket，再桥接到浏览器的 WebSocket。

---

## 3. 运行组件

### 3.1 Master

Master 二进制入口是 Cobra CLI：

- 启动命令：`cloudnest server`
- 入口链路：`main.go` -> `cmd.Execute()` -> `server` 子命令

`server` 子命令启动时会完成以下动作：

1. 解析 `listen/db-type/db-dsn` 参数与环境变量。
2. 初始化数据库。
3. 初始化内存缓存。
4. 设置签名密钥。
5. 启动后台调度器。
6. 构建 Gin Router。
7. 启动 HTTP 服务并处理优雅退出。

Master 的主要职责：

- 用户认证与会话管理
- Agent 注册与 WS 连接管理
- 节点、指标、文件元数据、Ping、告警等数据持久化
- Dashboard 实时广播
- 文件上传/下载代理
- 终端桥接
- 前端静态资源与安装脚本分发

### 3.2 Agent

Agent 也是 Cobra CLI：

- `cloudnest-agent register`
- `cloudnest-agent run`

本地配置保存在：

- `~/.cloudnest/agent.json`

配置项只有：

- `master_url`
- `uuid`
- `token`
- `port`
- `scan_dirs`
- `rate_limit`

运行时职责：

- 启动本地数据面 HTTP 服务
- 连接 Master 的 `/api/agent/ws`
- 每 10 秒发送一次心跳
- 首次全量、随后每 10 秒增量上报文件树
- 接收并执行 Master 下发的命令、Ping、删除文件指令
- 提供签名保护的数据面下载、上传、目录打包下载、终端入口

Agent 具备单实例锁，默认锁文件在：

- `~/.cloudnest/agent.run.lock`

### 3.3 前端

前端是一个嵌入到 Master 中的 React SPA，路由如下：

| 路由               | 页面         |
| ------------------ | ------------ |
| `/login`           | 登录页       |
| `/`                | Dashboard    |
| `/nodes/:uuid`     | 节点详情     |
| `/files`           | 文件搜索页   |
| `/terminal/:uuid`  | 终端页       |
| `/ping`            | Ping 任务    |
| `/alerts`          | 告警与通知渠道 |
| `/audit`           | 审计日志     |
| `/settings`        | 设置页       |

认证方式不是前端持久化 token，而是：

- `fetch(..., { credentials: "include" })`
- 依赖 Master 下发的 `session` Cookie
- `GET /api/auth/me` 用于恢复登录态

---

## 4. 启动与部署

### 4.1 Master 配置

Master 当前使用以下环境变量：

| 变量                       | 说明             | 默认值                      |
| -------------------------- | ---------------- | --------------------------- |
| `CLOUDNEST_LISTEN`         | 监听地址         | `0.0.0.0:8800`              |
| `CLOUDNEST_DB_TYPE`        | 数据库类型       | `sqlite`                    |
| `CLOUDNEST_DB_DSN`         | 数据库连接串     | `./data/cloudnest.db`       |
| `CLOUDNEST_REG_TOKEN`      | Agent 注册令牌   | `cloudnest-register`        |
| `CLOUDNEST_SIGNING_SECRET` | 数据面签名密钥   | `cloudnest-default-secret`  |

数据库只支持：

- `sqlite`
- `mysql`

SQLite 模式下会自动创建目录并启用 WAL。

### 4.2 镜像构建

顶层 `Dockerfile` 是多阶段构建：

1. 构建前端产物。
2. 交叉编译 Agent（`linux/amd64`、`linux/arm64`）。
3. 编译 Master，并把前端产物嵌入 `public/dist`。
4. 在最终镜像内放入：Master 二进制 + Agent 下载二进制。

因此运行中的 Master 同时承担：

- Web 控制台服务
- Agent 安装脚本分发
- Agent 二进制下载源

---

## 5. 通信模型

### 5.1 浏览器 ↔ Master

#### REST API

当前主要接口如下：

| 模块           | 接口                                                                                                                                                  |
| -------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| Auth           | `POST /api/auth/login` `POST /api/auth/logout` `GET /api/auth/me` `POST /api/auth/change-password`                                                   |
| Agent          | `POST /api/agent/register` `GET /api/agent/ws`                                                                                                        |
| Nodes          | `GET /api/nodes` `GET /api/nodes/:uuid` `GET /api/nodes/:uuid/metrics` `GET /api/nodes/:uuid/traffic` `PUT /api/nodes/:uuid/tags`                    |
| Node Files     | `GET /api/nodes/:uuid/files` `GET /api/nodes/:uuid/download`                                                                                          |
| Managed Files  | `POST /api/files/upload` `GET /api/files/download/:id` `GET /api/files` `POST /api/files/mkdir` `DELETE /api/files/:id` `PUT /api/files/:id/move` `GET /api/files/search` |
| Proxy          | `PUT /api/proxy/upload/:file_id` `GET /api/proxy/download/:file_id` `GET /api/proxy/browse`                                                          |
| Command        | `POST /api/nodes/:uuid/exec` `GET /api/commands/:id`                                                                                                  |
| Ping           | `GET /api/ping/tasks` `POST /api/ping/tasks` `GET /api/ping/tasks/:id/results` `DELETE /api/ping/tasks/:id`                                           |
| Alerts         | `GET/POST/PUT/DELETE /api/alerts/rules` `GET/POST /api/alerts/channels` `PUT /api/alerts/channels/:id`                                               |
| Admin          | `GET /api/admin/settings` `PUT /api/admin/settings` `GET /api/admin/audit`                                                                            |

#### WebSocket

浏览器只使用两个 WebSocket 端点：

| 端点                          | 用途                             |
| ----------------------------- | -------------------------------- |
| `GET /api/ws/dashboard`       | Dashboard 实时心跳/状态推送      |
| `GET /api/ws/terminal/:uuid`  | 浏览器终端与 Agent shell 的桥接通道 |

### 5.2 Agent ↔ Master 控制面

控制面走 WebSocket：

- 地址：`/api/agent/ws`
- 认证：`Authorization: Bearer <agent_token>`
- 协议格式：JSON-RPC 风格消息结构

### Agent → Master

| 方法                      | 说明                                           |
| ------------------------- | ---------------------------------------------- |
| `agent.heartbeat`         | 上报节点指标、磁盘信息、运行状态               |
| `agent.fileTree`          | 上报文件树全量或增量                           |
| `agent.fileStored`        | 上传完成回执                                   |
| `agent.fileDeleted`       | 删除完成回执                                   |
| `agent.pingResult`        | Ping 结果回传                                  |
| `agent.commandResult`     | 远程命令执行结果                               |
| `agent.verifyResult`      | 校验结果回传（当前仅有接收端）                 |
| `agent.replicateResult`   | 副本复制结果回传（当前仅有接收端）             |

### Master → Agent

当前 Master 真实发出的控制消息只有四类：

| 方法                  | 说明               |
| --------------------- | ------------------ |
| `master.execCommand`  | 执行远程命令       |
| `master.startPing`    | 启动 Ping 任务     |
| `master.stopPing`     | 停止 Ping 任务     |
| `master.deleteFile`   | 删除托管文件副本   |

注意：

- 文档草稿里出现过的 `master.openTerminal` 在当前代码中并不存在。
- `master.verifyFile`、`master.replicateFile` 在 Agent 有处理分支，但 Master 当前没有实际发送逻辑。

### 5.3 Master ↔ Agent 数据面

Agent 当前暴露的数据面路由如下：

| 路由                      | 用途                               |
| ------------------------- | ---------------------------------- |
| `PUT /api/files/:file_id` | 上传文件                           |
| `GET /api/files/:file_id` | 下载按 `file_id` 存储的文件        |
| `GET /api/browse`         | 按真实受管路径下载文件或打包目录   |
| `GET /api/terminal`       | 终端 WebSocket                     |
| `GET /api/health`         | 本地就绪探针                       |

这些路由全部通过签名校验保护，`/api/health` 除外。

签名算法实际是：

```text
payload = UPPER(method) + ":" + resourceID + ":" + expires
sig = HMAC_SHA256(signing_secret, payload)
```

不同场景下的 `resourceID`：

| 场景           | resourceID                          |
| -------------- | ----------------------------------- |
| 普通下载       | `file_id`                           |
| 浏览路径下载   | `path`                              |
| 终端 WS        | `id`                                |
| 上传           | `file_id\|path\|name\|overwrite`    |

这意味着当前实现不是简单的 `fileID:expires` 签名模型。

---

## 6. 核心数据流

### 6.1 Agent 注册

```text
Agent register
  -> POST /api/agent/register (Bearer reg token)
  -> Master 生成 node uuid + agent token
  -> Agent 保存到 ~/.cloudnest/agent.json
```

### 6.2 心跳与实时监控

```text
Agent 每 10s 发送 agent.heartbeat
  -> Master 更新 nodes.last_seen/status/disk_*
  -> 指标写入内存缓冲
  -> 最新指标缓存到 metric:<uuid>
  -> 广播给 /api/ws/dashboard
  -> 调度器每 60s 批量落库到 node_metrics
```

离线判定：

- `healthChecker` 每 10 秒运行一次
- 超过 30 秒无心跳且 WS 也已断开时，节点会被标记为 `offline`

### 6.3 文件树与节点目录浏览

```text
Agent 启动时先发一次全量 fileTree
Agent 之后每 10s 重新扫描 scan_dirs
  -> 只发送 added / removed
  -> Master 缓存 filetree:<uuid>
  -> 前端通过 /api/nodes/:uuid/files?path=... 浏览某个节点当前目录
```

说明：

- 当前没有独立的 `modified` 字段。
- 文件变化会被折叠进 `added`。
- Agent 运行时会强制把托管存储根目录加入 `scan_dirs`。

### 6.4 托管文件上传

当前实现是“单节点上传”，不是多节点并行上传：

```text
Browser
  -> POST /api/files/upload
  -> Master 创建 file + file_replica(status=pending)
  -> 返回 /api/proxy/upload/:file_id?node=...
  -> Browser PUT 到 Master proxy
  -> Master 生成签名 Agent URL 并流式转发
  -> Agent 写入文件
  -> Agent 发送 agent.fileStored
  -> Master 把 replica.status 更新为 stored
  -> 若无 pending replica，则把 file.status 更新为 ready
```

当前限制：

- `node_uuid` 才是真正使用的上传目标。
- 即使请求体里给了 `node_uuids`，当前也只会使用第一个。
- 前端没有“选多个目标节点上传”的界面。

### 6.5 下载

有两种下载入口，但两者最终都经过 Master proxy：

#### 节点真实目录下载

```text
Browser -> GET /api/nodes/:uuid/download?path=...
        -> Master 返回 /api/proxy/browse?... 
        -> Browser 请求 proxy
        -> Master 生成签名 Agent /api/browse 请求并转发
```

#### 托管文件下载

```text
Browser -> GET /api/files/download/:id
        -> Master 选择一个 online + stored 的 replica
        -> 返回 /api/proxy/browse?... 
        -> Browser 请求 proxy
        -> Master 转发给 Agent
```

说明：

- 当前实现里 `GET /api/proxy/download/:file_id` 路由存在，但托管文件下载主路径实际更常走 `/api/proxy/browse`。
- 浏览目录下载时，如果目标是目录，Agent 会实时打包为 zip 返回。

### 6.6 搜索

当前的“跨节点文件搜索”并不是搜索 file tree 缓存，而是：

- 查询 `files` 表
- 条件：`status = ready` 且 `is_dir = false`
- 因此它搜索的是“通过托管上传接口进入数据库的文件”，不是所有节点真实目录中的所有文件

### 6.7 删除

```text
Browser -> DELETE /api/files/:id
        -> Master 把 file.status 标记为 deleting
        -> 向每个 replica 所在 Agent 发送 master.deleteFile
        -> Agent 删除本地文件并回发 agent.fileDeleted
        -> Master 删除对应 file_replica
        -> GC 周期任务在 replica 清空后软删除 file 记录
```

### 6.8 远程命令

```text
Browser -> POST /api/nodes/:uuid/exec
        -> Master 创建 command_task(status=pending)
        -> 发送 master.execCommand
        -> Agent 用 shell 执行，默认 60s 超时
        -> Agent 回发 agent.commandResult
        -> Master 更新 output/exit_code/status
        -> 前端轮询 GET /api/commands/:id
```

### 6.9 Web 终端

```text
Browser WS -> /api/ws/terminal/:uuid
           -> Master 校验节点在线
           -> Master 生成签名 Agent WS URL `/api/terminal`
           -> Master 与 Agent 建立 WS
           -> Master 在 Browser WS 与 Agent WS 之间双向转发
           -> Agent 启动本地 shell 并把 stdin/stdout 绑定到 WS
```

### 6.10 Ping 任务

```text
Browser -> POST /api/ping/tasks
        -> Master 落库 ping_task
        -> 广播 master.startPing 给所有在线 Agent
        -> Agent 按 interval 执行 icmp/tcp/http 探测
        -> Agent 发送 agent.pingResult
        -> Master 落库 ping_results
```

补充：

- 删除任务时，Master 会广播 `master.stopPing`。
- Agent 会把小于 5 秒的间隔强制提升为 60 秒。
- ICMP 当前只有 Linux 真正实现，非 Linux 是 stub。

### 6.11 告警

```text
调度器每 10s 扫描 enabled alert rules
  -> 读取节点状态或指定时间窗口内的指标
  -> 判断 sustained threshold / offline
  -> 命中后调用 notify sender
  -> 更新 last_fired_at
  -> 写入一条 audit_log(alert_fired)
```

支持的通知渠道类型：

- `telegram`
- `webhook`
- `email`
- `bark`
- `serverchan`

---

## 7. 数据存储

启动时自动迁移以下表：

| 分组           | 表                                                  |
| -------------- | --------------------------------------------------- |
| 节点与指标     | `nodes` `node_metrics` `node_metric_compacts`       |
| 文件           | `files` `file_replicas`                             |
| 认证           | `users` `sessions`                                  |
| 告警           | `alert_rules` `alert_channels`                      |
| Ping           | `ping_tasks` `ping_results`                         |
| 运维           | `command_tasks` `audit_logs`                        |
| 系统设置       | `settings`                                          |

几个容易混淆的点：

- `Setting` 表是真实存在的，不应遗漏。
- `FileReplica` 表已经存在，但当前没有完整的自动副本编排流程。
- `Node.Status` 字段当前主要看到 `online/offline`，文档里常出现的 `draining` 并没有业务流使用。
- `Node.RateLimit` 会在注册配置里保存，但 Master 侧没有动态调度逻辑去使用它。

### 7.1 内存缓存

Master 使用内存缓存保存两类热数据：

- `metric:<uuid>`：最新实时指标
- `filetree:<uuid>`：节点文件树

此外还有一个内存缓冲区用于聚合心跳指标，供调度器批量落库。

---

## 8. 后台任务

当前真实存在的后台任务只有 5 个：

| 任务                  | 周期   | 作用                                           |
| --------------------- | ------ | ---------------------------------------------- |
| `metricFlusher`       | 60s    | 把内存中的 `NodeMetric` 批量写入数据库         |
| `healthChecker`       | 10s    | 检测 30 秒未更新的节点并标记离线               |
| `startCompaction`     | 30min  | 把 4 小时前的原始指标压缩到 15 分钟粒度        |
| `startAlertEvaluator` | 10s    | 评估告警规则并发送通知                         |
| `startGC`             | 2min   | 重试 deleting 文件删除、清理离线节点缓存       |

说明：

- 当前不存在 `replication.go`、`integrity.go`、`health.go` 这类独立调度文件。
- 指标查询中，`1h/4h` 使用原始指标；`24h/7d` 使用“压缩指标 + 最近 4 小时原始指标”的合并结果。

---

## 9. 前端结构

### 9.1 技术栈

当前前端技术栈是：

- React 19
- TypeScript
- Vite
- Tailwind CSS v4
- `lucide-react`
- `recharts`
- `@xterm/xterm`

注意：

- 当前代码里没有 `shadcn/ui` 的依赖和目录结构。
- 组件以项目内自定义组件为主。

### 9.2 页面职责

| 页面          | 当前职责                                                           |
| ------------- | ------------------------------------------------------------------ |
| Dashboard     | 节点概览、在线状态、基础容量、实时心跳融合                         |
| NodeDetail    | 节点信息、历史指标图表、节点目录浏览/下载、单节点上传、标签编辑、快速命令 |
| FileBrowser   | 托管文件搜索与下载，仅搜索模式                                     |
| Terminal      | 基于 xterm.js 的远程终端                                           |
| PingTasks     | 任务列表、新建、删除、查看结果                                     |
| Alerts        | 规则增删改、启停、渠道创建与编辑                                   |
| AuditLog      | 表格展示审计日志                                                   |
| Settings      | 修改密码、主题/语言偏好                                            |
| Login         | 登录                                                               |

### 9.3 当前前端边界

以下能力在文档草稿里常被写成“已完成”，但当前前端并未真正实现：

- 标签筛选
- 多节点上传选择器
- 托管文件目录式管理器
- 文件副本管理界面
- 独立 `FileSearch` 页面
- 告警渠道删除按钮

---

## 10. 认证与安全边界

### 10.1 用户认证

- 登录成功后会创建 30 天有效的 `sessions` 记录。
- Master 会写入 `session` Cookie。
- 认证中间件同时支持：Cookie + Bearer Token。
- 启动时如果数据库中没有用户，会自动创建默认管理员：`admin / admin`。

### 10.2 Agent 认证

- 注册使用 `CLOUDNEST_REG_TOKEN`。
- 注册成功后每个节点都会拿到独立 `agent token`。
- Agent WS 连接依赖这个 token。

### 10.3 数据面签名

- 文件上传、下载、路径浏览、终端都使用 HMAC-SHA256 签名。
- Agent 只校验签名是否正确、是否过期。
- 当前“5 分钟有效期”是 Master 生成签名 URL 时的策略，而不是 Agent 端写死的常量。

### 10.4 路径约束

Agent 对以下路径操作做了受管目录校验：

- `GET /api/browse`
- 带 `name/path` 的上传
- 按 `store_path` 删除文件

这保证浏览/删除不会越过受管根目录。

---

## 11. 仓库结构

以下是与当前实现一致的目录概览：

```text
save_vps/
├── cloudnest/
│   ├── cmd/
│   │   ├── root.go
│   │   └── server.go
│   ├── internal/
│   │   ├── api/
│   │   │   ├── admin/
│   │   │   ├── agent/
│   │   │   ├── alerts/
│   │   │   ├── auth/
│   │   │   ├── command/
│   │   │   ├── files/
│   │   │   ├── nodes/
│   │   │   ├── ping/
│   │   │   └── terminal/
│   │   ├── cache/
│   │   │   └── cache.go
│   │   ├── database/
│   │   │   ├── dbcore/
│   │   │   └── models/
│   │   ├── notify/
│   │   │   └── sender.go
│   │   ├── scheduler/
│   │   │   ├── alerts.go
│   │   │   ├── compaction.go
│   │   │   ├── gc.go
│   │   │   └── scheduler.go
│   │   ├── server/
│   │   │   ├── middleware/
│   │   │   └── server.go
│   │   ├── transfer/
│   │   │   └── signer.go
│   │   └── ws/
│   │       ├── dashboard.go
│   │       ├── errors.go
│   │       ├── hub.go
│   │       └── safeconn.go
│   ├── public/
│   │   ├── dist/
│   │   └── embed.go
│   └── main.go
├── cloudnest-agent/
│   ├── cmd/
│   │   ├── register.go
│   │   ├── root.go
│   │   └── run.go
│   ├── internal/
│   │   ├── agent/
│   │   ├── reporter/
│   │   ├── server/
│   │   ├── storage/
│   │   │   └── path.go
│   │   ├── terminal/
│   │   └── ws/
│   └── main.go
├── cloudnest-web/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── contexts/
│   │   ├── hooks/
│   │   ├── i18n/
│   │   ├── pages/
│   │   └── utils/
│   └── package.json
├── Dockerfile
├── docker-compose.yml
└── ARCHITECTURE.md
```

---

## 12. 当前已知边界与后续可演进点

为了避免把“设计目标”误写成“当前事实”，这里把最重要的边界单独列出：

1. 文件上传当前是单节点上传，`node_uuids` 没有真正形成多副本并行上传流程。
2. `FileReplica`、`verifyResult`、`replicateResult` 说明副本体系有数据模型和 Agent 侧预留，但 Master 侧还没有完整编排。
3. `/files` 前端当前只是搜索页，不是完整文件管理器。
4. 标签当前只能显示和编辑，不能筛选。
5. 审计日志不是“所有管理操作全留痕”，目前明确写入的场景主要是 `settings_updated` 和 `alert_fired`。
6. 文档中若要继续写“目标架构”或“Roadmap”，应单独开章节，避免与“当前实现架构”混在一起。

---

## 13. 建议的文档维护原则

后续维护 `ARCHITECTURE.md` 时，建议遵守三条规则：

1. 先写“当前实现”，再写“计划能力”。
2. 所有接口、调度器、目录结构都以仓库现有文件为准，不写想象中的模块名。
3. 凡是只有模型或处理分支、没有完整业务入口的能力，都标成“部分实现”或“预留”。
