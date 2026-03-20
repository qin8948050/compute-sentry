# Compute-Sentry 项目开发路线与核心业务逻辑全景图

## 1. 核心业务逻辑概览 (Core Business Logic)

Compute-Sentry 的核心逻辑围绕“**状态感知 -> 风险判定 -> 自动执行 -> 效能优化**”的闭环展开。其本质是为每一个 AI 训练任务增加一个“智能护航层”。

### 1.1 业务流向
1.  **任务提交**：用户通过 K8s 提交训练 Pod。
2.  **动态拦截 (Hook)**：Operator 拦截请求，根据 `ComputeSentryPolicy` 或 Pod Annotation 识别注入需求，并动态注入 `Spy` 劫持库路径、`LD_PRELOAD` 环境，以及**策略相关的治理参数**。
3.  **阶段 A：硬准入 (Hard Admission)**：
    *   Pod 的 `InitContainer` 启动。
    *   执行 **“算力指纹公测”**（P2P Bandwidth, HBM R/W, GEMM Test）。这些测试的**阈值由匹配的 ComputeSentryPolicy 动态配置**。
    *   对比型号基准词典。若偏差超过动态配置的阈值，则任务报错退出。
4.  **阶段 B：软监控 (Soft Observability)**：
    *   任务正式开始运行。
    *   `Spy` 劫持库捕获每一个 NCCL 通信算子的 `Start` 和 `End`，计算执行延迟。
    *   将包含 Pod 身份、GPU 型号和延迟的指标流通过 UDS 异步推送到本节点的 Agent。
5.  **风险判定与状态更新 (Agent Edge Intelligence)**：
    *   Agent 接收指标流，**根据 Pod Annotation 中注入的策略参数** (如 `maxNcclLatencyUs`, `windowSize`, `errorCountLimit`)，在本地内存中执行滑动窗口算法进行实时风险判定。
    *   若判定为亚健康状态，Agent 使用 **高效 Patch 机制** 更新 Pod 的 Annotation (`compute-sentry.aiguard.io/health: "unhealthy"`)。
    *   Agent 同时汇聚指标上报 Prometheus。
6.  **反馈闭环与自愈 (Operator Central Enforcement)**：
    *   `Compute-Sentry Operator` 的 `HealthController` 监听 Pod 的 `unhealthy` 状态 Annotation。
    *   根据 Policy 的 `Actions` 配置，执行：
        *   **节点污点 (Taint)**：给 Pod 所在节点打上 `NoSchedule` 污点，防止新任务进入。
        *   **Pod 驱逐 (Evict)**：使用 **K8s Eviction API** 驱逐异常 Pod。
    *   **全局安全节流**：Operator 内置保护机制，防止短时间内大量驱逐 Pod 造成集群雪崩。

---

## 2. 详细开发路线图 (Development Roadmap)

我们将开发过程分为四个阶段，确保每一步都有可验证的交付物。

### Phase 1: 感知原型期 (Perception Layer - Spy)
**目标**：实现在不改动代码的情况下获取 NCCL 延迟数据。
- [x] **任务 1.1**：构建 C++ `Spy` 项目骨架，实现核心符号劫持（劫持 `ncclAllReduce`）。
- [x] **任务 1.2**：设计无锁异步队列与 Unix Domain Socket (UDS) 传输协议。
- [x] **任务 1.3**：编写 Mock 训练程序，验证劫持库能精准捕获延迟。
- [x] **任务 1.4**：**Phase 1 Demo & Testing**：演示不改代码劫持 NCCL 调用并异步输出指标流。
- **交付物**：`libCompute-Sentry-Spy.so` 原型与验证报告。

### Phase 2: K8s 治理集成期 (Control Plane - Operator)
**目标**：实现 K8s 原生的自动注入与策略驱动的预检及运行时治理。
- [x] **任务 2.1**：编写 `Compute-Sentry-Operator`。实现 Mutating Webhook，自动改写 Pod 环境变量（注入 `LD_PRELOAD`）。
- [x] **任务 2.2**：**Pod Mutator 注入强化**：实现策略优先级判断 (Annotation > Policy)、注入参数 (GovernanceConfig)，以及预检环境变量 (`PRECHECK_MIN_...`)。
- [x] **任务 2.3**：**InitContainer 预检程序强化**：实现基于策略的动态阈值校验，并具备硬件指标不达标的阻断能力。
- [x] **任务 2.4**：设计 `Compute-SentryPolicy` CRD。允许用户针对不同任务设置不同的健康阈值。
- [x] **任务 2.5**：**HealthController 安全治理模块**：实现 K8s Eviction API 驱逐、Node Taint、以及全局驱逐节流保护，并优化 Pod 列表性能 (NodeName Indexer)。
- [x] **任务 2.6**：**Phase 2 Demo & Testing**：在 K8s 集群中演示 Pod 自动注入 Spy 库、策略驱动的准入与运行时治理。
- **交付物**：`Compute-Sentry-Operator` 与完善的注入和治理策略。

### Phase 3: 全栈观测闭环期 (Observability - Grafana-First)
**目标**：利用业务侧最熟悉的 Grafana 实现“免开发”的高性能可视化与 Agent 分布式观测闭环。
- [x] **任务 3.1**：**Compute-Sentry-Agent (DaemonSet) 强化**：负责接收 UDS 数据、**动态加载 Pod 治理策略**、高效 Patch K8s 状态，并暴露 Prometheus Metrics 接口。 (`agent/main.go`, `collector/collector.go`, `manifests/agent-daemonset.yaml`)
- [x] **任务 3.2**：Prometheus Metrics 暴露与指标聚合逻辑实现 (Verified)
- [ ] **任务 3.3**：Grafana Dashboard 模板设计。充分利用 PromQL 实现"型号-节点-Pod"的多维聚合。 (`agent/exporter/prometheus.go`)
- [ ] **任务 3.4**：实现 **基于 K8s Node Labels 的物理拓扑数据映射**。
    *   **原则**：Compute-Sentry 采用 **“K8s 代码驱动”** 模式。直接读取 Node Labels（例如 `topology.aiguard.io/switch: leaf-01`）获取物理架构信息。
    *   **优势**：确保了方案的通用性与 K8s 原生性，实现监控链路与外部系统的完全解耦，提升了在不同数据中心环境下的适配能力。
- [x] **任务 3.5**：实现异构算力基准规整化逻辑。
- [ ] **任务 3.6**：**Phase 3 Demo & Testing**：演示从 GPU 型号到交换机端口的全栈性能看板。
- **交付物**：`Compute-Sentry-Agent` 与 Grafana 3D 监控面板。

### Phase 4: 确定性自愈期 (Automation & Self-healing)
**目标**：实现亚健康节点的自动化管理。
- [ ] **任务 4.1**：开发 `Auto-Healer` 模块。当监控发现某节点 NCCL 延迟持续异常且超过阈值时，自动给节点打上 `aiguard.io/subhealth=true:NoSchedule` 污点。
- [ ] **任务 4.2**：实现与 Volcano 的联动。将链路延迟作为自定义分值（Weighted Priority）传递给调度组件。
- [ ] **任务 4.3**：模拟压力测试（注入 PCIe 人为降级），通过验证端到端自愈流程。
- [ ] **任务 4.4**：**Final Project Presentation**：演示从探测到亚健康后的自动容错与调度闭环。
- **交付物**：完备的算力治理自愈体系。

---

## 3. 各阶段核心代码目录预演
```text
/ai-guard
  /spy            # (C++) 劫持库源码
    /src          # 劫持逻辑与 UDS 通信
    /include      # 通用头文件
  /operator       # (Go) K8s 控制逻辑
    /webhook      # 注入动态逻辑
    /controller   # CRD 协调逻辑
  /agent          # (Go) 节点级 DaemonSet
    /collector    # 数据搜集与聚合
    /exporter     # Prometheus 暴露
  /manifests      # K8s 部署文件 (YAML)
  /tests          # 性能测试与 Mock 场景
```
