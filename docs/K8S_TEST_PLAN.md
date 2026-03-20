# Compute-Sentry K8S 环境测试指南 (Remote Minikube)

本指南用于在远端 Minikube 集群验证 Compute-Sentry 的全链路治理能力，包括：精细化策略注入、硬件准入预检、Agent 分布式观测以及 Operator 安全驱逐。

## !!重要: KUBECONFIG 配置!!
请在执行所有 `kubectl` 和 `make` 命令前，确保您已正确设置 `KUBECONFIG` 环境变量，指向您的 Minikube 集群配置。例如：
```bash
export KUBECONFIG=./tests/kubeconfig
# 或者根据您的实际路径
# export KUBECONFIG=/path/to/your/minikube/kubeconfig
```

---

## 1. 准备工作

### A. 镜像构建 (本地执行)
```bash
# 1. 构建 Agent (包含 C++ Spy 库)
cd agent && make docker-build IMG=compute-sentry-agent:latest

# 2. 构建 Operator
cd ../operator && make docker-build IMG=compute-sentry-operator:latest
```

### B. 镜像同步 (根据您的环境自行同步)
将构建好的镜像 `compute-sentry-agent:latest` 和 `compute-sentry-operator:latest` 推送到您的远端 Minikube 节点中。
*提示：如果是 Minikube，通常使用 `minikube image load <image_name>`。*

---

## 2. 部署组件

### A. 注册 CRD 与 部署 Operator
```bash
cd operator
# 安装 CRD
make install
# 部署 Operator (确保镜像地址正确)
make deploy IMG=compute-sentry-operator:latest
```

### B. 部署 Agent (DaemonSet)
```bash
# 部署权限
kubectl apply -f manifests/agent-rbac.yaml
# 部署 Agent
kubectl apply -f manifests/agent-daemonset.yaml
```

---

## 3. 测试场景验证

### 场景一：无策略手动注入 (验证回退机制)
**目标**：验证 Pod 在没有匹配 Policy 时，能否通过 Annotation 成功注入，且 Agent 使用全局默认值。

1. **提交测试 Pod**:
   ```bash
   kubectl apply -f tests/test-injection.yaml
   ```
2. **验证注入**:
   * 检查 Pod 是否包含 `compute-sentry-precheck` 初始化容器。
   * 检查是否**没有** `compute-sentry.aiguard.io/governance-config` Annotation。
   * 查看 Agent 日志，确认其针对该 Pod 使用了全局默认阈值 (默认 500us)。

### 场景二：精细化策略注入 (验证策略优先级)
**目标**：验证 `ComputeSentryPolicy` 定义的阈值能正确注入到 Pod 中。

1. **创建策略**:
   ```yaml
    apiVersion: config.aiguard.io/v1
    kind: ComputeSentryPolicy
    metadata:
      name: example-gpu-policy
      namespace: default # 或者您的 Pod 所在的命名空间
    spec:
      # 选择器：定义这个策略适用于哪些 Pod
      # 这里的例子是匹配所有带有 app: my-gpu-app 标签的 Pod
      selector:
        matchLabels:
          app: test-injection
      
      # SpyConfig：配置 Spy Sidecar 的注入行为
      spyConfig:
        enabled: true # 如果为 true，则在匹配的 Pod 上注入 Spy 相关的 sidecar

      # Thresholds：定义性能健康阈值
      thresholds:
        maxNcclLatencyUs: 150        # NCCL AllReduce 操作的最大允许延迟 (微秒)
        maxJitterUs: 50              # NCCL 操作的最大允许抖动 (微秒) - 注意：目前 Agent 尚未实现对 Jitter 的具体评估逻辑
        minP2PBandwidthGbps: 25      # P2P 内存拷贝的最小带宽要求 (GB/s) - 用于 InitContainer 预检
        minHbmBandwidthGbps: 1200    # HBM (高带宽显存) 的最小带宽要求 (GB/s) - 用于 InitContainer 预检

      # EvalConfig：定义 Agent 侧的健康评估参数
      evalConfig:
        windowSize: 10               # 滑动窗口大小 (秒)，在此窗口内评估错误次数
        errorCountLimit: 5           # 在 windowSize 内，允许的最大违规次数，超过则标记为 unhealthy

      # Actions：定义当 Pod 被标记为 unhealthy 时的补救措施
      actions:
        enableTaint: true            # 如果为 true，则给 Pod 所在节点打上污点 (NoSchedule)
        enableEvict: true            # 如果为 true，则驱逐该 unhealthy 的 Pod

   ```
2. **提交匹配该策略的 Pod** (Label 包含 `app: llm-train`)。
3. **验证**:
   * 检查 Pod 的 Annotation `compute-sentry.aiguard.io/governance-config` 是否包含 `{"maxNcclLatencyUs": 100, ...}`。

### 场景三：硬件准入阻断 (验证 Precheck)
**目标**：验证硬件指标不达标时，Pod 无法启动。

1. **修改策略**：将 `minP2PBandwidthGbps` 设置为 `100` (模拟无法达到的高要求)。
2. **提交 Pod**。
3. **现象**：Pod 的 InitContainer 应该报错退出。
4. **日志检查**:
   ```bash
   kubectl logs <pod-name> -c compute-sentry-precheck
   # 预期输出: ERROR: P2P Bandwidth below threshold! Blocking startup.
   ```

### 场景四：实时治理与安全节流 (验证 Eviction & Throttle)
**目标**：验证算子变慢后触发驱逐，且 Operator 具备自我保护能力。

1. **模拟故障**：进入业务容器运行 `mock_nccl`，产生超过 100us 的延迟。
2. **观察状态切换**:
   * Agent 发现异常，将 Pod 标注为 `health: unhealthy`。
3. **观察驱逐**:
   * Operator 监听到 `unhealthy` 状态，发起 `Eviction` 请求。
4. **验证节流**：
   * 批量制造 10 个异常 Pod。
   * 检查 Operator 日志，验证是否触发了“5% 阈值保护”，停止了进一步的驱逐。

---

## 4. 常用调试命令

* **查看治理事件**: `kubectl get events --sort-by=.lastTimestamp`
* **查看 Operator 日志**: `kubectl logs -l control-plane=controller-manager -n compute-sentry-system -c manager`
* **检查节点污点**: `kubectl get nodes -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints`
