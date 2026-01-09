# 架构设计指南（阶段一）

- 项目名称：cdpnetool（Chrome DevTools Protocol Network Tools）
- 目标：基于CDP与浏览器通信，拦截并可控篡改网页的网络请求与响应
- GUI策略：GUI与核心逻辑分离，核心以Go库形式提供稳定API与事件流；GUI后续通过进程内或IPC集成
- 依赖：CDP客户端库采用 github.com/mafredri/cdp（Chromium/Chrome/Edge优先）

## 阶段划分与范围

- 阶段一（核心功能）：CDP连接与会话管理、请求/响应拦截、规则引擎（复杂匹配器与内容修改器）、并发与背压、基础可观测性（结构化日志与基础指标）、基础配置（JSON）
- 暂缓实现：权限/鉴权、热更新与回滚、插件与脚本扩展、IPC服务、复杂GUI交互与远程控制、审计与合规的高级特性

## 架构总览

- 分层模块
  - cdp/session：DevTools发现与连接、会话生命周期、Target管理、Fetch/Network启停
  - intercept/handler：Fetch.requestPaused处理、执行continue/fulfill/fail等动作
  - rules/engine：复杂匹配器与内容修改器、优先级与冲突处理、命中统计与缓存
  - payload/store：大Body外存分块与临时文件管理、对象池与零拷贝优化
  - observability：结构化日志、基础指标、事件追踪ID与关联
  - config/runtime：规则与运行配置的JSON加载与校验、默认值管理
- 事件流
  - CDP事件→入队→规则匹配→执行动作→调用CDP返回→上报结果事件与指标
  - 背压与降级：队列超限或超时自动继续原请求并记录降级事件

## 两阶段拦截模型

- 请求阶段：浏览器→拦截与篡改→服务器
  - Fetch.requestPaused(stage=Request)→匹配→continueRequest/fulfillRequest/failRequest
- 响应阶段：服务器→拦截与篡改→浏览器
  - Fetch.requestPaused(stage=Response)→读取响应头与必要时体→匹配→continueResponse/fulfillRequest/failRequest
  - 必要时结合Network.responseReceived与Network.getResponseBody获取体；超出体量阈值走摘要与截断策略
- RequestPattern默认启用
  - 资源类型：XHR、Fetch；可选扩展到Document/Script/Stylesheet/Media（按需）
  - 阶段：Request与Response均启用
  - 回退：若不支持Response阶段拦截，则以Network事件观察与有限替换策略降级

## 规则引擎设计

- 匹配器（复杂匹配）
  - 组合逻辑：allOf/anyOf/noneOf，支持嵌套；默认allOf
  - 条件类型：
    - URL：glob/regex/prefix/exact
    - 方法：集合匹配
    - 头部：键存在/equals/contains/regex（键大小写不敏感）
    - Query/Cookie：键值equals/contains/regex
    - Body：
      - JSON Pointer：存在/equals/regex
      - 文本：mime为text/*时支持contains/regex
      - 体量与类型：size范围、mime前缀
    - 时间窗口与概率：支持窗口化规则与采样
- 内容修改器（请求/响应）
  - 请求修改：URL/方法替换；头/Query/Cookie的set/remove/append；Body支持JSON Patch（RFC6902）、文本regex替换、二进制base64
  - 响应修改：状态码、头部与Body替换；响应体通过fulfillRequest返回
  - 组合与顺序：修改器按声明顺序执行；失败策略可配置（继续原值/失败）
- 优先级与冲突
  - 显式priority与稳定排序；默认短路（命中后停止后续规则）
  - 可选聚合执行（将多个动作组合应用）
  - 命中上限与采样策略可配置（阶段二扩展）

## 手动阻塞修改机制

- 目标：请求前或响应前暂停，等待用户手动编辑；超时自动放行（默认继续原请求或应用默认动作）
- 规则触发：
  - 动作pause：包含stage=request/response、timeoutMS、defaultAction（continue_original/continue_mutated/fulfill/fail）
- 事件与审批：
  - 生成PendingItem事件（URL/方法/头/Query/Body预览、会话/目标ID），进入待审批队列
  - 接口（为GUI/CLI预留）：SubscribePending()、ApproveRequest(itemID, mutations)、ApproveResponse(itemID, mutations)、Reject(itemID)
  - 超时与降级：timeoutMS到达未审批，执行defaultAction并记录事件（完整审计在阶段二开启）

## 公共API（最小稳定集草案）

```go
package api

type SessionID string
type TargetID string
type RuleID string

type SessionConfig struct {
    DevToolsURL       string
    Concurrency       int           // worker数量
    BodySizeThreshold int64         // 外存分块阈值，默认4MB
    PendingCapacity   int           // 待审批队列容量，默认64
    ProcessTimeoutMS  int           // 拦截处理最大耗时，默认200ms
}

type RuleSet struct {
    Version string          // "1.0"
    Rules   []Rule
}

type Rule struct {
    ID       RuleID
    Priority int
    Mode     string         // "short_circuit" | "aggregate"
    Match    Match          // 复杂匹配器
    Action   Action         // 内容修改器 + 控制动作
}

type Match struct {
    AllOf      []Condition
    AnyOf      []Condition
    NoneOf     []Condition
}

type Condition struct {
    Type   string           // "url"|"method"|"header"|"query"|"cookie"|"json_pointer"|"text"|"mime"|"size"|"time_window"|"probability"
    // 依据Type包含相应字段：pattern/mode/values/key/op/value/pointer等
}

type Action struct {
    Rewrite   *Rewrite
    Respond   *Respond
    Fail      *Fail
    DelayMS   int
    DropRate  float64
    Pause     *Pause
}

type Rewrite struct {
    URL     *string
    Method  *string
    Headers map[string]*string
    Query   map[string]*string
    Cookies map[string]*string
    Body    *BodyPatch
}

type BodyPatch struct {
    Type string               // "json_patch"|"text_regex"|"base64"
    Ops  []any                // 根据Type解析
}

type Respond struct {
    Status  int
    Headers map[string]string
    Body    []byte
    Base64  bool
}

type Fail struct {
    Reason string             // 例如 "ConnectionFailed"
}

type Pause struct {
    Stage        string       // "request"|"response"
    TimeoutMS    int          // 默认3000
    DefaultAction struct {
        Type   string         // "continue_original"|"continue_mutated"|"fulfill"|"fail"
        Status int            // 当Type=fulfill
        Reason string         // 当Type=fail
    }
}

type EngineStats struct {
    Total   int64
    Matched int64
    ByRule  map[RuleID]int64
}

type Event struct {
    Type    string            // "intercepted"|"mutated"|"fulfilled"|"failed"|"degraded"|"pending"|"approved"|"timeout"
    Session SessionID
    Target  TargetID
    Rule    *RuleID
    Error   error
}

type Service interface {
    StartSession(cfg SessionConfig) (SessionID, error)
    StopSession(id SessionID) error
    AttachTarget(id SessionID, target TargetID) error
    DetachTarget(id SessionID, target TargetID) error

    EnableInterception(id SessionID) error
    DisableInterception(id SessionID) error

    LoadRules(id SessionID, rs RuleSet) error
    GetRuleStats(id SessionID) (EngineStats, error)

    SubscribeEvents(id SessionID) (<-chan Event, error)
    SubscribePending(id SessionID) (<-chan any, error) // PendingItem
    ApproveRequest(itemID string, mutations Rewrite) error
    ApproveResponse(itemID string, mutations Rewrite) error
    Reject(itemID string) error
}
```

## 规则JSON示例

```json
{
  "version": "1.0",
  "rules": [
    {
      "id": "manual_edit_payment",
      "priority": 200,
      "mode": "short_circuit",
      "match": {
        "allOf": [
          { "type": "url", "mode": "prefix", "pattern": "https://api.example.com/payment" },
          { "type": "method", "values": ["POST"] }
        ]
      },
      "action": {
        "pause": {
          "stage": "request",
          "timeoutMS": 5000,
          "defaultAction": { "type": "continue_original" }
        }
      }
    },
    {
      "id": "inject_500",
      "priority": 90,
      "mode": "short_circuit",
      "match": {
        "anyOf": [
          { "type": "url", "mode": "regex", "pattern": "/payment" },
          { "type": "header", "key": "X-Route", "op": "equals", "value": "pay" }
        ],
        "noneOf": [
          { "type": "probability", "value": 0.0 }
        ]
      },
      "action": {
        "respond": {
          "status": 500,
          "headers": { "Content-Type": "application/json" },
          "body": "eyJlcnJvciI6ICJmYWlsZWQifQ==",
          "base64": true
        },
        "delayMS": 50
      }
    }
  ]
}
```

## CDP集成与调用序列（mafredri/cdp）

- 启动与启用
  - 使用DevTools端点发现Target→建立WebSocket→创建cdp.Client
  - Network.enable→Fetch.enable({patterns: [{stage=Request, resourceTypes=[XHR,Fetch]}, {stage=Response, resourceTypes=[XHR,Fetch]}]})
- 请求阶段处理
  - on Fetch.requestPaused(stage=Request)→构造上下文→规则匹配→continueRequest/fulfillRequest/failRequest
- 响应阶段处理
  - on Fetch.requestPaused(stage=Response)→读取响应头→必要时Network.getResponseBody→规则匹配→continueResponse/fulfillRequest/failRequest
- 回退
  - 若Response拦截缺失，则以Network.responseReceived观察并有限替换（主要头与错误注入）

## 并发与背压策略

- 队列：优先级队列；fast-path（无暂停的小改写）与pending队列（需人工审批）
- Worker Pool：固定或弹性并发数，按SessionConfig.Concurrency控制
- 超时与降级：每拦截事件设最大处理时限（ProcessTimeoutMS，默认200ms）；超时自动continue原请求并记录“degraded”
- pending容量：PendingCapacity（默认64）；满载时直接执行defaultAction并上报降级

## 性能与可靠性目标

- 延迟预算：单次拦截处理开销控制在毫秒级（典型<5ms，不含手动暂停）
- 吞吐：高并发下维持稳定处理；具备限流与降级路径
- 资源管理：BodySizeThreshold（默认4MB）以上走外存分块与摘要；对象池用于临时结构降低GC压力
- 错误处理：区分可恢复/不可恢复；重试与回退策略针对CDP连接与Fetch启停

## 可观测性与日志

- 结构化日志：控制台与文件输出；默认遮盖Cookie、Authorization等敏感字段
- 指标：QPS、规则命中率、延迟分布、错误统计、队列长度、pending超时比例、外存使用
- 事件追踪：为每拦截事件生成TraceID并贯穿生命周期

## 配置管理（阶段一）

- 规则与运行配置均采用JSON；提供Schema校验与明确错误提示
- 默认参数：
  - Concurrency：CPU核数或配置值
  - BodySizeThreshold：4MB
  - ProcessTimeoutMS：200
  - PendingCapacity：64
  - Pause默认timeoutMS：3000

## 兼容与支持范围

- 浏览器：Chromium系稳定版优先；针对CDP能力进行检测与优雅回退
- 平台：Windows/macOS/Linux；Go 1.21+（建议1.22）

## 非目标与边界（阶段一）

- 不做系统级代理/MITM；仅作用于受控CDP会话
- 不持久化敏感数据；日志默认遮盖
- 权限/鉴权、审计与合规高级特性将在后续阶段规划

## 里程碑

- M1：最小可用（连接、两阶段拦截、基本规则与修改器、日志与指标）
- M2：规则引擎完善（优先级、聚合、复杂Body处理与优化）
- M3：可观测性与稳定性增强（更全面指标、事件追踪、降级策略打磨）
- M4：扩展能力（热更新、IPC、GUI对接、插件/脚本）

## 验收准则（阶段一）

- 稳定拦截：在Chromium稳定版上持续拦截与处理请求/响应
- 规则正确性：表驱动测试覆盖复杂匹配器与修改器核心路径
- 性能与资源：典型场景延迟毫秒级；高并发具备背压与降级；外存与对象池生效
- 可观测性：日志与指标完整；支持定位问题与分析命中
- 配置与错误提示：规则与配置加载可靠，错误信息清晰

## 后续工作

- 提供规则JSON Schema草案，保证配置校验与版本化
- 完善API与伪实现流程图，开始搭建代码框架

## 重构与优化计划（阶段性方案）

本节用于指导后续对 cdpnetool 的架构重构与代码优化，从宏观到具体步骤都以此为基准。

### 1. 重构目标

- **保持能力**：在不破坏现有对外行为的前提下，保留并强化当前已具备的能力：
  - 稳定的 CDP 会话管理与两阶段拦截（Request/Response）。
  - 规则 DSL 与 JSON 配置（`pkg/rulespec` + CONFIG.md）。
  - Service 接口（`pkg/api.Service`）作为对 GUI/CLI/远程控制的统一门面。
- **优化架构**：将当前偏“原型化”的内部实现，演化为分层清晰、可扩展的系统：
  - 拆解 God Object（`internal/cdp/manager.go`）。
  - 引入协议无关的领域模型（InterceptContext/Domain Session）。
  - 规范 Pending/Pause 机制、并发与背压策略。
- **增强可观测性与可维护性**：
  - 统一日志与事件模型，预留指标与 Trace 扩展点。
  - 提升可测试性，支持在不依赖真实浏览器的情况下进行单元测试。

### 2. 目标分层结构

#### 2.1 领域层（Domain）

- **职责**：只表达“网络拦截”这一领域本身，不关心 CDP 等具体协议细节。
- **核心模型（规划）**：
  - `InterceptContext`：
    - URL、Method、Headers、Query、Cookies、Body、ContentType、Stage（request/response）、时间戳、TraceID 等。
  - `RuleEngine` 接口：抽象规则引擎评估能力：
    - `Eval(ctx InterceptContext) (Result, bool)`。
    - 当前 `internal/rules.Engine` 通过轻量包装即可实现。
  - `Result`/`Action`：可重用 `rulespec.Action` 与 `model.RuleID`，也可在后续演进为更抽象动作模型。
  - `EngineStats`、`Event`、`PendingItem`：承接架构文档中公共 API 设计，用于统计、审计与 GUI 展示。

#### 2.2 CDP 适配层（Adapter）

- **职责**：把 `mafredri/cdp` 和浏览器 DevTools 的细节封装起来，对上暴露领域友好的接口。
- **核心组件（规划）**：
  - `BrowserSession/CDPConnection`：
    - 负责 DevTools 发现、Target 附加/分离、Network/Fetch 启停。
  - `CDPInterceptor`（重构后的 Manager）：
    - 订阅 `Fetch.requestPaused` 事件 → 构造 `InterceptContext` → 调用领域层 RuleEngine → 根据结果调用 Continue/Fail/Fulfill/Rewrite 等 CDP 操作。
  - `BodyMutator`：
    - 专门处理 JSON Patch / 文本正则 / Base64 等 Body 修改逻辑，减少拦截器本身的复杂度。
  - `PauseCoordinator`：
    - 管理 Pause/审批相关的 pending 队列、超时 DefaultAction 和外部 Approve/Reject 调用。
  - `WorkspaceWatcher/TargetSelector`：
    - 负责 DevTools 目标轮询与“前台可见页面”选择，为自动跟随当前 Tab 提供支持。

#### 2.3 应用服务层（Service/API）

- **职责**：为上层提供稳定的服务接口，屏蔽内部实现演化。
- **主要元素**：
  - `SessionRegistry`：
    - 管理 Session 生命周期（Start/Stop）、当前规则集与运行配置。
    - 将领域 Session 与 CDPInterceptor 组合在一起对外暴露。
  - `pkg/api.Service`：
    - 保持现有方法签名基本不变，内部通过 SessionRegistry 与领域层协作。

#### 2.4 横切关注点：配置与规则装载

- 集中处理 `SessionConfig` 与 `RuleSet` 的：
  - JSON 解析与基础 Schema 校验。
  - 默认值填充（Concurrency、BodySizeThreshold、ProcessTimeoutMS、Pause.TimeoutMS 等）。
  - 错误提示与版本兼容策略。

#### 2.5 横切关注点：可观测性

- 在现有 `logger.Logger` 基础上：
  - 规范日志字段（至少包含 TraceID/SessionID/TargetID/RuleID）。
  - 预留 `Metrics` 接口，用于后续接入 Prometheus 等。
  - 在拦截事件中贯穿 TraceID，方便日志与事件关联分析。

### 3. 重点改造方向

#### 3.1 Manager 减负与职责拆分

- 拆出以下子组件（可先以内联结构/函数形式存在于 `internal/cdp`，后续再独立成文件或包）：
  - `ContextBuilder`：从 CDP 事件构造 `InterceptContext`（或当前的 `rules.Ctx`）。
  - `BodyMutator`：实现 JSON Patch/Text Regex/Base64 三种 Body 修改策略。
  - `PauseCoordinator`：管理 approvals map、pending 通道以及超时逻辑。
  - `TargetWatcher`：封装工作区轮询与可见性监听逻辑。
- Manager/拦截器本身聚焦于：
  - 消费拦截事件流、调用 RuleEngine、分发到各子组件，并上报 `model.Event` 与统计信息。

#### 3.2 领域模型与 CDP 边界

- 引入抽象的 CDP 客户端接口，例如：
  - `ContinueRequest/ContinueResponse`、`FailRequest`、`FulfillRequest`、`GetResponseBody`、`RewriteRequest/RewriteResponse` 等。
- 好处：
  - 可以通过 mock 接口做单元测试，无需依赖真实浏览器。
  - 为未来接入其他协议（例如代理模式）预加载同构接口。

#### 3.3 Pending/Pause 系统化

- 明确 `PendingItem` 结构：
  - ID、Stage、URL、Method、Headers/Cookies 摘要、Body 摘要（例如截断后 + hash）、SessionID、TargetID、触发 RuleID 等。
- 优化 API：
  - `SubscribePending` 返回 `<-chan PendingItem` 而非 `<-chan any`。
  - `ApproveRequest/ApproveResponse/Reject` 与 `PendingItem.ID` 明确绑定，区分请求/响应阶段行为。

#### 3.4 并发与背压策略

- 将当前基于 `sem` 的简单并发控制演进为：
  - 显式队列：fast-path 队列（不涉及 Pause）、pending 队列（涉及人工审批）。
  - Worker Pool：由 `SessionConfig.Concurrency` 决定工作协程数。
- 集中管理降级策略：
  - 队列溢出、处理超时、Pending 队列满时，统一执行“继续原请求并标记 degraded”。

#### 3.5 配置与规则装载

- 提供统一的配置入口：
  - 从 JSON 文件/字符串加载 `SessionConfig` 与 `RuleSet`，并进行校验与默认值填充。
- Demo 与未来 CLI/GUI 均复用该入口，避免分散的配置解析。

### 4. 分阶段实施计划

为减少一次性变更风险，整个重构拆为多个阶段，每个阶段尽量保持外部行为不变，仅优化内部结构。

1. **阶段 1：逻辑搬家与 Manager 减负**
   - 抽取构建决策上下文、Body 修改、Pause 审批等逻辑为独立函数/结构，减小 `manager.go` 复杂度。
   - 为 JSON Patch、条件匹配等纯逻辑增加单元测试。
2. **阶段 2：CDP 适配接口抽象**
   - 定义 CDP 抽象接口，并将现有 `cdp.Client` 封装为实现。
   - Manager/拦截器改为依赖接口，准备好未来测试与多实现切换。
3. **阶段 3：领域层与服务层梳理**
   - 将 `EngineStats`、`Event`、`PendingItem` 等统一放入领域层定义。
   - `internal/service` 聚焦 SessionRegistry 与 API 实现，弱化对 CDP 细节的感知。
4. **阶段 4：Pending/Pause 完善**
   - 落地 `PendingItem` 结构与序列化格式，强化 `SubscribePending`/`Approve`/`Reject` 的语义。
   - 为 Pending 流程增加事件与日志（pending/approved/rejected/timeout）。
5. **阶段 5：并发与背压重构（可与其他阶段交叉推进）**
   - 引入队列 + Worker Pool，集中管理超时和降级策略。
   - 配合 metrics 记录队列长度、处理延迟与降级次数。
6. **阶段 6：配置与可观测性增强**
   - 统一配置加载与校验逻辑，并更新 demo 使用配置驱动。
   - 定义基础 Metrics 接口，在关键路径打点和埋指标。

上述计划将作为后续所有重构工作的“路线图”，实际执行中如有设计调整，应在此文档中更新对应小节并说明原因。

## GUI 与 HTTP API（阶段一）

- **目标**：为后续 GUI（Web/Electron/桌面）提供稳定、协议无关的控制接口，封装内部 `pkg/api.Service` 能力。
- **接口形态**：
  - HTTP JSON：所有控制接口统一使用 `POST` + JSON 请求体，采用 RPC 风格，不暴露资源路径层级细节。
  - 事件流：后续通过独立通道（如 SSE/WebSocket）推送拦截事件与 Pending 审批项。
- **调用约定**：
  - 统一入口路径（例如 `/api/v1`）。
  - 请求格式：
    - `method`：字符串，表示操作名（例如 `session.start`、`target.list`）。
    - `id`：可选，请求 ID，便于 GUI 关联响应。
    - `params`：任意 JSON 对象，按 `method` 不同有不同结构。
  - 响应格式：
    - `id`：与请求对应的 ID。
    - `result`：成功时的返回数据。
    - `error`：失败时的错误对象，包含 `code` 与 `message`。
- **核心方法规划（第一阶段）**：
  - 会话管理：
    - `session.start`：创建拦截会话，对应 `StartSession`。
    - `session.stop`：停止会话，对应 `StopSession`。
  - 目标管理：
    - `target.list`：列出某个 Session 下浏览器中的 page 目标，对应 `ListTargets`。
    - `target.attach`：附加指定 page 目标，对应 `AttachTarget`。
    - `target.detach`：移除指定 page 目标，对应 `DetachTarget`。
  - 拦截控制：
    - `session.enable`：启用拦截，对应 `EnableInterception`。
    - `session.disable`：停用拦截，对应 `DisableInterception`。
  - 规则与统计：
    - `rules.load`：装载 RuleSet，对应 `LoadRules`。
    - `stats.rules`：查询规则命中统计，对应 `GetRuleStats`。
- **后续扩展方向**：
  - 增加 Pending 审批相关方法（例如 `pending.list`、`pending.approve`、`pending.reject`）。
  - 确定事件通道方案（SSE 或 WebSocket），并与上述 `method` 命名保持一致的事件类型。

## GUI 客户端（Fyne）阶段规划

- **目标形态**：提供一个桌面控制台，用于本地管理拦截会话、浏览器页面目标、规则集与拦截事件/Pause 审批。
- **界面总体布局**：
  - 主窗口采用左右分栏 + 上部工具条：
    - 顶部工具条：全局操作（新建会话、启用/停用拦截、附加默认页面等）。
    - 左侧侧边栏：会话列表与会话级配置，展示当前所有 Session 及其状态。
    - 右侧主区域：按 Tab 分为 Targets/Rules/Events/Pending 四个功能视图。

### 1. 布局设计

- 左侧 Session 面板：
  - 顶部：DevToolsURL 输入框 + "新建会话" 按钮；必要时支持并发数等高级配置。
  - 列表：显示 SessionID（缩略）、DevToolsURL、启用状态等，支持选择当前活跃会话。
  - 选中会话后，右侧各 Tab 的数据与操作均作用于该会话。
- 右侧主视图 Tabs：
  - Targets：管理浏览器 page 目标（列表展示 Title/URL/类型/是否已附加，提供附加/移除操作）。
  - Rules：展示当前规则集概要（ID/Name/Priority/Mode），支持从 JSON 文件加载规则并应用到会话。
  - Events：实时展示拦截事件流（类型、TargetID、RuleID、简要描述），用于观察规则命中与降级情况。
  - Pending（后续阶段）：展示 Pause 审批队列，支持在 GUI 中编辑 Rewrite 并审批或拒绝。

### 2. 功能阶段划分

1. **阶段 1：最小可用 GUI（Session + Targets）**
   - 实现基础布局骨架：顶部工具条、左侧 Session 面板、右侧 Tabs，其中只启用 Targets Tab。
   - 支持：
     - 新建会话：输入 DevToolsURL，创建 Session（默认并发与阈值）。
     - 选择当前会话：在 Session 列表中切换活跃会话。
     - 列出目标：调用 `target.list` 或直接使用 `api.Service.ListTargets`，在 Targets Tab 中展示所有 page。
     - 附加/移除目标：基于列表中的选中项调用 `target.attach` / `target.detach`。
   - 目标：在 GUI 中完成“启动会话 → 选择页面 → 开始监听该页面”的最小闭环。

2. **阶段 2：拦截控制与规则管理（Session + Rules）**
   - 工具条增加启用/停用拦截按钮，调用 `session.enable` / `session.disable`。
   - Rules Tab：
     - 支持从文件选择器加载 RuleSet JSON，调用 `rules.load` 应用于当前 Session。
     - 展示规则摘要列表（ID/Name/Priority/Mode），可查看规则数量与简单统计信息。
   - 目标：让用户在 GUI 内完成“配置规则 → 启用拦截 → 观测规则行为”的配置闭环。

3. **阶段 3：事件视图（Events）**
   - Events Tab：
     - 与后端事件通道（本地 channel 或 HTTP SSE/WebSocket）对接，实时追加拦截事件。
     - 支持按 Session/Target/RuleType 简单过滤或高亮关键事件（例如 degraded/failed）。
   - 可选：提供基础的统计摘要（例如最近 N 条事件中各类型占比）。
   - 目标：提供“观察与调试”视角，方便用户理解规则效果和降级行为。

4. **阶段 4：Pause 审批与高级交互（Pending）**
   - Pending Tab：
     - 展示 Pause 产生的待审批项（PendingID/Stage/URL/Method/RuleID 等）。
     - 支持在 GUI 中构造 Rewrite（URL/头/Body JSON Patch 等），调用后端审批接口。
     - 支持快速拒绝或应用默认动作，并展示审批历史记录。
   - 目标：在 GUI 中完整覆盖架构文档中“手动阻塞修改机制”的核心交互流程。

5. **阶段 5：体验优化与扩展**
   - 引入更多状态展示与诊断信息，例如：规则命中统计图、并发队列长度与降级次数等。
   - 优化布局与交互细节（快捷键、右键菜单、多窗口支持等），根据实际使用反馈迭代。
   - 预留与远程 HTTP API 协同的模式，使 GUI 可选地作为远程控制台使用。