# 问题清单

本文件按复核结果整理为三类：

- `保留`：确认成立，建议继续作为有效问题跟踪
- `降级`：核心现象成立，但依赖使用边界、设计取舍，或原描述/严重度偏重
- `删除`：当前代码下结论不成立，或更接近功能缺口/设计选择，不建议继续当作真实问题

## 保留的真问题

| 严重程度 | 问题 | 原因 | 是否修改 |
| --- | --- | --- | --- |
| Critical | symlink 路径校验只做词法判断，上传和删除可能越过 managed root 改写外部文件 | `ResolveManagedPath` 和 `RelativeManagedPath` 没有处理符号链接，只校验了清理后的路径；当受管目录下存在指向外部目录的 symlink 时，上传和删除会跟随到根目录外 | 保留 |
| Critical | 默认签名密钥硬编码，可伪造所有签名 URL | `transfer/signer.go` 和 `cloudnest-agent/internal/server/auth.go` 都使用默认密钥 `cloudnest-default-secret`；未配置 `CLOUDNEST_SIGNING_SECRET` 时，签名可被直接伪造 | 保留 |
| Critical | 首次启动自动创建 `admin/admin` 账户 | `EnsureDefaultAdmin()` 会在无用户时自动创建默认管理员；如果部署后未立即改密，管理面直接暴露 | 保留 |
| Critical | Agent 注册 Token 使用已知弱默认值 | 未配置 `CLOUDNEST_REG_TOKEN` 时默认使用 `cloudnest-register`，且注册接口没有额外防护，默认部署风险明显 | 保留 |
| Critical | 登录接口无速率限制，可暴力破解密码 | 登录路由在公开路由组中，没有 IP 限流、失败惩罚或账户锁定机制 | 保留 |
| High | 浏览和下载接口会跟随 symlink 读取 managed root 外的文件或目录 | `handleBrowseDownload` 后续使用 `os.Stat`、`c.FileAttachment`、`filepath.Walk`，都会跟随 symlink，把根目录外内容暴露出来 | 保留 |
| High | `MoveFile` 只改数据库元数据，Agent 侧真实文件不会移动 | `MoveFile` 仅修改 `files.path` 和 `files.name`，未向 Agent 下发 rename，也未同步 `file_replicas.store_path`，会导致下载路径错位 | 保留 |
| High | 覆盖上传会先截断旧文件，传输中断时会把文件写坏 | 覆盖上传直接以 `O_TRUNC` 打开目标文件，没有先写临时文件再原子替换；一旦传输失败，旧文件先被清空 | 保留 |
| High | 会话过期后的 401 不会让前端回到登录页 | 前端只在 `AuthProvider` 首次挂载时检查登录状态，后续 API 返回 401 时不会统一清理会话，页面会静默显示为空数据 | 保留 |
| High | 上传初始化先落库，真实上传失败后不会回滚，留下脏数据 | `InitUpload` 在真实文件上传前就写入 `files.status=uploading` 和 `file_replicas.status=pending`，`ProxyUpload` 失败后没有回滚 | 保留 |
| High | `install.sh` 将签名密钥写入命令行历史和 systemd unit 文件 | `SIGNING_SECRET` 通过命令行参数传递，容易进入 shell history、进程列表，并被写入 unit 文件 | 保留 |
| High | `BroadcastToAgents` 持读锁执行网络 I/O，慢 Agent 会阻塞整个 Hub | `BroadcastToAgents` 在持有 `RLock` 时逐个 `WriteJSON`，慢连接会拖住其它 Hub 操作 | 保留 |
| High | 指标缓冲区无上限，且 flush 逐条写库，DB 故障时可能内存无界增长 | `metricsBuffer` 是无界切片，`metricFlusher` 每 60 秒逐条 `db.Create`；一旦数据库卡住或 Agent 数量增大，堆积会明显放大 | 保留 |
| High | 告警评估存在 N+1 查询问题 | 先查全部规则，再按规则分别查节点、指标，规则一多数据库压力会快速上升 | 保留 |
| High | 登录、登出事件未写入审计日志 | 目前只有告警和设置更新写入 `AuditLog`，登录成功、失败、登出都没有审计记录 | 保留 |
| High | 远程命令执行未写入审计日志 | `command.Exec` 会创建任务记录，但没有写入审计日志，无法清晰追溯谁在什么时间执行了什么命令 | 保留 |
| High | Agent WebSocket 连接无读取超时，僵尸连接可能长期不清理 | `conn.ReadMessage()` 没有 `SetReadDeadline`，网络断链但未正常 close 时，服务端可能长期卡在读循环里 | 保留 |
| High | 通知发送和文件代理使用无超时的 HTTP 客户端 | `notify/sender.go` 和 `api/files/proxy.go` 的外部 HTTP 请求都没有超时，异常连接会长时间挂住 goroutine | 保留 |
| Medium | HTTP 类通知渠道把 4xx 和 5xx 当成发送成功 | Telegram、Webhook、Bark、ServerChan 只判断请求是否报错，不检查状态码和响应体；失败时仍可能被当成成功 | 保留 |
| Medium | Agent 注册和文件复制使用无超时 HTTP 客户端，异常网络下会永久等待 | `RegisterWithMaster` 和 `replicateFile` 使用默认客户端且无 `timeout/context`，半开连接或异常代理会让任务一直阻塞 | 保留 |
| Medium | WS 断线后任务结果会被静默吞掉 | `SendJSON` 在 `conn == nil` 时直接返回 `nil`，调用方会误以为发送成功，导致命令执行、校验、复制结果丢失 | 保留 |
| Medium | Dashboard 每次节点状态变化都会触发全量 HTTP 拉取 | `statusVersion` 变化后会重新 `getNodes()` 和 `getSettings()`，实时推送被当成全量拉取触发器，扩展性较差 | 保留 |
| Medium | 网速计算硬编码采样间隔，且包级状态无并发保护 | `reporter/metrics.go` 直接用 `/10` 计算速率，并依赖包级 `lastNetIn/lastNetOut`，心跳间隔变化或并发调用时结果会错 | 保留 |
| Medium | 命令结果轮询无取消机制，最长持续 60 秒，离开页面后仍会继续请求 | `handleExecCommand` 轮询 60 次且没有 abort 机制；组件卸载后请求仍会继续跑，存在资源浪费和状态更新风险 | 保留 |
| Medium | Session 创建缺少错误检查，可能返回无效 Token | `dbcore.DB().Create(&session)` 没有检查错误，数据库写入失败时仍可能给前端返回 200 和 token | 保留 |
| Medium | 文件搜索 `LIKE` 通配符未转义，可能产生意外匹配 | 用户输入中的 `%`、`_` 没有转义，搜索结果可能比预期更宽 | 保留 |
| Medium | 节点详情页的指标和文件列表会被旧请求覆盖 | 切换时间范围或目录时会并发发起多个请求，没有 `AbortController` 或请求序号保护，慢响应会覆盖新状态 | 保留 |
| Medium | Ping 结果面板会串台到其它任务 | 所有展开面板共用一份 `results` 和 `resultsLoading` 状态，快速切换时旧请求会覆盖当前结果 | 保留 |
| Medium | 文件搜索结果可能与输入框当前内容不一致 | 搜索只做了防抖，没有取消旧请求，也没有忽略过期响应，旧关键字结果可能覆盖新关键字 | 保留 |
| Low | 退出后再登录时会短暂显示上一轮会话的实时指标 | `useWebSocket` 关闭连接时不清空全局缓存，重新登录后在新心跳到来前会短暂显示旧数据 | 保留 |
| Low | `compaction.go` 中 `idx` 变量计算后从未使用 | 代码里保留了 70th percentile 索引计算，但实际已改成平均值，属于明确的死代码 | 保留 |
| Low | Agent 重复注册不做去重，数据库中会积累僵尸节点 | 每次执行 `register` 都创建新的 UUID 和 Token，旧节点记录不会自动清理 | 保留 |
| Low | Bark 和 Webhook 告警渠道配置字段混用，逻辑混淆 | 前端把 webhook 和 bark 都写成 `{ url, server_url }`，会产生多余字段，配置语义不清晰 | 保留 |

## 条件成立 / 需要降级的问题

| 严重程度 | 问题 | 原因 | 是否修改 |
| --- | --- | --- | --- |
| Medium | Agent 注册信任客户端自报 IP，后续代理和终端会回连该地址 | 这条只在“注册 token 泄露并被恶意 Agent 使用”时才会真正形成 pivot/SSRF；对个人自用、受信 Agent 部署场景优先级可明显降低 | 降级 |
| Medium | 告警冷却按规则全局生效，不按节点独立生效 | 现状确实是规则级冷却，但是否算 bug 取决于预期语义；如果你希望同一规则全局只告一次，这就是设计选择 | 降级 |
| Low | `UpdateSettings` 无键白名单，允许写入任意设置键 | 现在更像“会产生无效脏配置键”，但当前代码里这些自定义键并未明显驱动关键行为；原表述里“可污染任意配置影响应用行为”偏重 | 降级 |
| Low | Agent WebSocket 重连循环不响应 context 取消 | `Connect()` 的确不接受 cancel，但当前进程收到退出信号后会直接结束；它更像优雅退出不足，而不是持续性运行 bug | 降级 |
| Low | 终端代理双 goroutine 关闭协调不完整 | 代码清理确实不够漂亮，但更偏资源回收边角问题，未看到会稳定复现的高危错误路径 | 降级 |
| Low | `generateInstallScript` 中 `masterURL` 直接字符串插值，存在脚本注入边界风险 | 只有在 Host 头可被异常代理或恶意流量控制时才比较值得担心；正常个人部署场景优先级很低 | 降级 |
| Low | `ScanDirectories` 无文件数量、深度上限，大目录场景下代价会很大 | 这是实际存在的扩展性问题，但是否会成为故障高度依赖你受管目录规模；若目录可控且不大，优先级可后放 | 降级 |
| Low | `Tags` 字段存储裸字符串无格式验证 | 后端确实允许写入任意字符串，但前端已有 `try/catch` 兜底；在只通过自家前端操作时影响很有限 | 降级 |

## 伪问题 / 建议删除的问题

| 严重程度 | 问题 | 原因 | 是否修改 |
| --- | --- | --- | --- |
| - | CORS 配置 `AllowAllOrigins + AllowCredentials` 同时开启，导致 CSRF | 你当前使用的 `gin-contrib/cors` 会返回 `Access-Control-Allow-Origin: *`；浏览器不会接受带凭据的这种跨域响应，再叠加 `SameSite=Lax`，原描述的 CSRF 攻击链不成立 | 删除 |
| - | 所有 WebSocket Upgrader 禁用 Origin 检查，可直接跨站 WebSocket 劫持 | `CheckOrigin=true` 是坏味道，但当前几条关键 WebSocket 路径要么依赖 `Authorization` 头、要么依赖同站 cookie 或签名 URL，原表述“都可直接劫持”不成立 | 删除 |
| - | 命令执行接口无 RBAC 控制 | README 已明确这是“单管理员认证”项目，不存在多角色权限模型；“没有 RBAC”不是 bug | 删除 |
| - | Ping 任务无 SSRF 防护 | 对这个项目来说，Ping/探测本来就是功能的一部分；在单管理员、自用场景下，把它写成 SSRF 漏洞不合适 | 删除 |
| - | SMTP 通知使用明文 `PlainAuth`，凭据在非 TLS 连接上裸传 | Go 标准库 `net/smtp.PlainAuth` 在非 TLS 且非 localhost 场景下会直接返回错误，不会发送凭据，原描述不成立 | 删除 |
| - | `compactNodeMetrics` 中 `Count -> Insert` 非原子，并发时产生重复数据 | 当前 compaction 只有一个后台 goroutine 入口，代码里没有并发执行该逻辑的路径；这是未来扩展风险，不是当前 bug | 删除 |
| - | 文件树缓存增量更新非原子，存在并发竞态损坏 | `handleFileTree` 在单连接读循环里串行执行，同一 Agent 的消息不会并发进入该函数；原问题把理论上的 RMW 风险写成了当前竞态 | 删除 |
| - | Hub 连接替换存在竞态，新连接可能被错误标记离线 | 当前 `Register` 与 `UnregisterIfCurrent` 的连接对象比对就是为了解决这个问题；按现有实现，这条更像误报 | 删除 |
| - | `AlertChannel` 缺少 DELETE 接口 | 这是功能缺口，不是 bug | 删除 |
| - | 审计日志查询硬限 200 条且无分页 | 这是能力不足或产品迭代项，不属于当前错误行为 | 删除 |
| - | `dbcore.DB()` 在初始化失败后返回 nil，调用方会 panic | 主启动路径上 `dbcore.Init` 失败会直接 `log.Fatalf` 退出，当前运行路径不会继续带着 nil DB 提供服务；原问题不构成现网 bug | 删除 |

