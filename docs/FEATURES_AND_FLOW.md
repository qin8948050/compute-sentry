# Compute-Sentry 功能与业务流程文档

## 1. 项目概述

Compute-Sentry 是一款 K8s 原生的算力治理系统，旨在通过非侵入劫持与确定性准入技术，消除 AI 训练集群中的硬件亚健康（Gray Failure）与性能黑盒问题。

## 2. 已完成功能

### Phase 1: 感知层 (Spy) ✅

| 组件 | 文件路径 | 功能描述 |
|------|----------|----------|
| **C++ 劫持库** | `spy/src/spy.cpp` | 使用 `LD_PRELOAD` + `dlsym(RTLD_NEXT)` 劫持 `ncclAllReduce` 函数 |
| **无锁队列** | `spy/include/queue.h` | 高性能 Lock-free Queue 用于异步数据传输 |
| **UDS 客户端** | `spy/src/uds.*` | Unix Domain Socket 将指标数据发送给 Agent |
| **GPU 型号检测** | `spy/src/spy.cpp:25-62` | 运行时动态检测 GPU 型号（支持环境变量覆盖 `COMPUTE_SENTRY_GPU_MODEL_OVERRIDE`） |

**劫持的函数：**
- `ncclAllReduce` - NCCL 全规约通信
- (预留) `cudaMalloc` - CUDA 内存分配
- (预留) `cudaMemcpy` - CUDA 内存拷贝

### Phase 2: 控制层 (Operator) ✅

| 组件 | 文件路径 | 功能描述 |
|------|----------|----------|
| **Pod 注入 Webhook** | `operator/internal/webhook/v1/pod_mutator.go` | Mutating Webhook 自动注入 `LD_PRELOAD`、UDS 挂载卷、InitContainer |
| **CRD 定义** | `operator/api/v1/computesentrypolicy_types.go` | 定义 `ComputeSentryPolicy` 策略资源（已定义，Controller 逻辑待完善） |
| **预检脚本注入** | `pod_mutator.go:88-100` | 自动注入 `compute-sentry-precheck` InitContainer |

**Webhook 注入内容：**
1. `LD_PRELOAD` 环境变量指向 `libcompute-sentry-spy.so`
2. `COMPUTE_SENTRY_UDS_PATH` 环境变量
3. UDS 挂载卷 (`/var/run/compute-sentry`)
4. Spy 库挂载卷 (`/opt/compute-sentry/lib`)
5. Precheck 脚本挂载卷 (`/opt/compute-sentry/bin`)
6. InitContainer 执行预检脚本

### Phase 3: 观测层 (Agent) ✅

| 组件 | 文件路径 | 功能描述 |
|------|----------|----------|
| **指标收集器** | `agent/collector/collector.go` | 通过 UDS 接收 Spy 库发送的二进制指标 |
| **Prometheus Exporter** | `agent/exporter/prometheus.go` | 暴露 `/metrics` 接口 |
| **节点拓扑映射** | `agent/main.go:76-110` | 从 K8s Node Labels 读取 `switch/rack/gpu-model` 信息 |
| **文件分发** | `agent/main.go:112-138` | 将 spy 库和预检脚本分发到 HostPath 供 Pod 使用 |
| **DaemonSet 部署** | `manifests/agent-daemonset.yaml` | 以 DaemonSet 形式运行在每个节点 |

**暴露的指标：**

```
compute_sentry_nccl_latency_us{type, node, switch, rack, node_gpu_model, runtime_gpu_model}
compute_sentry_nccl_ops_total{type, node, switch, rack, node_gpu_model, runtime_gpu_model}
```

**Node Label 映射：**
- `topology.aiguard.io/switch` → `switch`
- `topology.aiguard.io/rack` → `rack`
- `topology.aiguard.io/gpu-model` → `node_gpu_model`

## 3. 整体业务流程

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         [ 用户提交 AI 训练 Pod ]                             │
│                                                                             │
│  metadata:                                                                  │
│    annotations:                                                             │
│      compute-sentry.aiguard.io/inject: "true"                               │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                    Operator Mutating Webhook 拦截                            │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  1. 检查 annotation: compute-sentry.aiguard.io/inject="true"        │   │
│  │  2. 注入 LD_PRELOAD=/opt/compute-sentry/lib/libcompute-sentry-spy.so│   │
│  │  3. 注入 COMPUTE_SENTRY_UDS_PATH=/var/run/compute-sentry/spy.sock   │   │
│  │  4. 注入 VolumeMounts: /var/run/compute-sentry, /opt/compute-sentry │   │
│  │  5. 注入 InitContainer: compute-sentry-precheck                      │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
              ┌───────────────────────┼───────────────────────┐
              ▼                       ▼                       ▼
┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│  InitContainer   │    │   Main Container │    │   Agent (Daemon)  │
│   (Pre-check)    │    │  (AI Training)   │    │                  │
├──────────────────┤    ├──────────────────┤    ├──────────────────┤
│ 执行算力指纹检测  │    │  加载 PyTorch     │    │ ① 等待 UDS 连接  │
│ (P2P/HBM/GEMM)  │    │ → 调用 nccl      │    │ ② 接收指标数据   │
│                  │    │ → Spy 劫持生效   │    │ ③ 暴露 Prometheus│
│  准入判定:       │    │                  │    │    metrics       │
│  通过 → 启动     │    │ Spy 计算耗时     │    │                  │
│  失败 → 退出     │    │ 推送到 UDS       │    │ ④ 映射拓扑标签   │
└──────────────────┘    └────────┬─────────┘    └────────┬─────────┘
                                 │                       │
                                 │   UDS Socket         │   HTTP
                                 │   /var/run/          │   :9091/metrics
                                 │   compute-sentry/    │
                                 │   spy.sock           │
                                 └───────────┬───────────┘
                                             │
                                             ▼
                              ┌──────────────────────────┐
                              │     Prometheus           │
                              │  收集节点级指标          │
                              │  (带 node/switch/rack/  │
                              │   node_gpu_model/       │
                              │   runtime_gpu_model)     │
                              └────────────┬─────────────┘
                                           │
                                           ▼
                              ┌──────────────────────────┐
                              │      Grafana             │
                              │  3D 拓扑可视化           │
                              │  算子延迟 → 物理链路    │
                              └──────────────────────────┘
```

## 4. 核心数据流

### 4.1 注入阶段

1. **Pod 创建** → K8s API Server 接收请求
2. **Webhook 拦截** → 修改 Pod Spec，注入环境变量和卷
3. **调度器分配** → Pod 被调度到目标节点
4. **Agent 准备** → Agent DaemonSet 将 spy 库和脚本分发到 HostPath

### 4.2 Init 阶段 (Pre-check)

1. **InitContainer 启动** → 执行 `/opt/compute-sentry/bin/precheck.sh`
2. **算力指纹检测** → P2P 带宽测试、HBM 读写测试、GEMM 性能测试
3. **准入判定** → 对比基准值，偏差过大则 Pod 启动失败

### 4.3 Runtime 阶段 (Observability)

1. **训练进程启动** → 加载 PyTorch/NCCL 库
2. **LD_PRELOAD 生效** → 调用 `ncclAllReduce` 时被 Spy 劫持
3. **计算延迟** → 记录函数入口和出口时间戳
4. **异步发送** → 通过 Lock-free Queue 发送到后台线程
5. **UDS 传输** → 后台线程通过 Unix Domain Socket 发送给 Agent
6. **Agent 接收** → Collector 监听 UDS，解析二进制数据
7. **Prometheus 暴露** → 关联拓扑标签后通过 `/metrics` 接口暴露

## 5. GPU 型号的两种来源

| 标签 | 来源 | 含义 |
|------|------|------|
| `node_gpu_model` | Node Label (`topology.aiguard.io/gpu-model`) | 节点级 GPU 型号 |
| `runtime_gpu_model` | Spy 运行时检测 (CUDA API) | 容器实际使用的 GPU（可能是 MIG 实例） |

**为什么需要两个：**
- 节点可能配置 MIG 分片，同一节点上不同容器可能使用不同的 GPU 实例
- 节点可能有异构 GPU（混合不同型号）
- 运行时检测可以精确反映实际运行情况

## 6. 待完成功能

| 阶段 | 任务 | 状态 |
|------|------|------|
| Phase 3 | Grafana 3D 拓扑面板 | ❌ 待开发 |
| Phase 3 | 异构算力基准规整化 | ❌ 待开发 |
| Phase 4 | Auto-Healer 模块 | ❌ 待开发 |
| Phase 4 | 与 Volcano 调度联动 | ❌ 待开发 |
| - | CRD Controller 逻辑完善 | ❌ 待开发 |

## 7. 技术栈

- **Languages**: C++17, Go
- **Infrastructure**: Kubernetes, NCCL, CUDA
- **Observability**: Prometheus, Grafana
- **Communication**: Unix Domain Socket (UDS)
