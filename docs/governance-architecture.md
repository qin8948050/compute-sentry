# Compute-Sentry 算力治理平台：全链路闭环架构设计与实现指南

## 1. 核心设计理念 (Core Design Vision)
针对大规模 AI 训练集群中的“亚健康（Gray Failure）”和“性能隐性降速”问题，通过 **非侵入劫持** 和 **声明式治理**，构建一套“事前准入 -> 事中观测 -> 自动自愈”的确定性保障体系。

---

## 2. 系统组件职责 (Component Roles)

| 组件 | 角色 | 核心职责 |
| :--- | :--- | :--- |
| **ComputeSentryPolicy (CRD)** | **策略大脑 (Law)** | 定义治理边界（Selector）、健康阈值（Thresholds）和治理动作（Actions）。 |
| **Spy (libspy.so)** | **感知触角 (Senses)** | C++ 编写。通过 `LD_PRELOAD` 劫持 NCCL 算子，计算微秒级耗时，通过 UDS 将原始数据发送给 Agent。 |
| **Agent (DaemonSet)** | **本地判官 (Intelligence)** | Go 编写。聚合 Spy 指标；执行本地阈值与次数判断；更新 K8s 资源状态；暴露 Prometheus 指标。 |
| **Operator (Control Plane)** | **执行机构 (Executor)** | 实现 Mutating Webhook 进行自动注入；监听资源状态变化并执行隔离（Taint）或驱逐（Evict）。 |
| **Prometheus/Grafana** | **历史记录 (Historian)** | 存储聚合指标，提供可视化看板进行根因分析（RCA）和趋势预测。 |

---

## 3. 全链路工作流 (End-to-End Workflow)

### 3.1 策略定义与动态注入 (Policy & Admission)
1. **策略声明**：管理员创建 `ComputeSentryPolicy`，定义针对特定训练任务（如 `app: llm`）的延迟阈值（如 `maxNcclLatencyUs: 500`）。
2. **Webhook 拦截**：用户提交 Pod。Operator Webhook 识别符合 Selector 的 Pod，动态注入：
   - `LD_PRELOAD` 指向宿主机路径下的 `libspy.so`。
   - 挂载共享的 UDS 目录 `/var/run/compute-sentry/`。
   - 注入 **InitContainer (Precheck)**，在主程序运行前执行 `precheck.sh`。
3. **硬准入校验**：InitContainer 执行 30s 硬件压测（GPU P2P, HBM 带宽）。如果检查失败（退出码非0），Pod 停止启动，防止任务在亚健康节点上运行。

### 3.2 运行时感知与诊断 (Observation & Diagnosis)
1. **非侵入采集**：任务启动后，`libspy.so` 劫持 `ncclAllReduce` 等算子，记录微秒级耗时。
2. **极速传输**：`libspy.so` 通过 **Unix Domain Socket (UDS)** 将数据异步发送给本节点的 Agent，确保对业务零干扰。
3. **本地分流**：
   - **分流 A (指标流)**：Agent 聚合 P99 延迟，暴露给 Prometheus 进行全局监控。
   - **分流 B (治理流)**：Agent 开启本地“滑动窗口”监控。如果过去 10 秒内，慢算子（延迟 > 阈值）出现的次数超过 5 次，则判定该 Pod/节点 进入亚健康状态。

### 3.3 声明式自愈与止损 (Self-healing & Remediation)
1. **状态标记**：Agent 操作 K8s SDK 更新 Pod 的 Annotation：`compute-sentry.aiguard.io/health: "unhealthy"`。
2. **响应治理**：Operator 的 Controller 监听到状态异常，执行预定义动作：
   - **隔离 (Taint)**：给 Node 打上 `NoSchedule` 污点，防止新任务进入。
   - **驱逐 (Evict)**：如果是严重故障且策略允许，直接 Evict 异常 Pod，触发训练框架（如 TorchRun）在健康节点重试。

---

## 4. 关键技术规范 (Technical Implementation Details)

### 4.1 极致性能要求
- **Spy 库**：必须使用无锁队列（Lock-free Queue）进行指标缓冲，严禁在劫持函数中执行磁盘 IO 或网络请求。
- **UDS 协议**：Agent 与 Spy 必须使用单一共享的 UDS 文件进行通信，通信延迟应控制在微秒级。

### 4.2 诊断算法 (Agent 侧)
- **输入**：实时算子耗时流。
- **参数**：`threshold` (阈值), `window_size` (时间窗口), `error_count_limit` (容忍次数)。
- **逻辑**：只有当 `(count(duration > threshold) over window) > error_count_limit` 时，才触发异常上报。

### 4.3 解耦原则 (The Decoupling Principle)
- **Agent -> Operator**：禁止直接调用。Agent 只负责“更新状态（Annotation/CRD Status）”。
- **Operator -> Pod/Node**：只负责“调和状态（Reconcile）”。Operator 始终观察 K8s 状态并向期望状态靠拢。

---

## 5. 待执行开发任务清单 (Actionable Task List)

- [ ] **[Operator]** 扩展 `pod_mutator.go`，使其能够根据 `ComputeSentryPolicy` 的 Selector 自动决定注入配置。
- [ ] **[Agent]** 在 `agent/collector/` 模块中实现 `HealthEvaluator` 逻辑，增加 K8s Client 用于更新 Pod 状态。
- [ ] **[Operator]** 实现一个新的 `HealthController`，专门用于监听 `unhealthy` 标签并执行 Taint/Evict 动作。
- [ ] **[Spy]** 确保 C++ 代码中的 UDS 发送逻辑是线程安全且非阻塞的。

---
*文档结论：本架构通过快（治理流）慢（指标流）分离的设计，实现了 AI 算力底座的高可靠、低损耗治理。*
