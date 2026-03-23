# Compute-Sentry K8S 环境测试指南 (Remote Minikube)

本指南旨在提供一个从环境准备、组件部署到核心功能验证的完整测试流程，专注于节点级自愈能力。

## 1. 概述与 Kubeconfig 配置

本指南将引导您在远端 Minikube 集群中，测试 Compute-Sentry 的关键功能，特别是其**基于 Pod 聚合状态的节点自愈逻辑**。

**!!重要: KUBECONFIG 配置!!**
在执行所有 `kubectl` 和 `make` 命令前，请确保您已正确设置 `KUBECONFIG` 环境变量，指向您的 Minikube 集群配置。例如：
```bash
export KUBECONFIG=./tests/kubeconfig
# 或者根据您的实际路径
# export KUBECONFIG=/path/to/your/minikube/kubeconfig
```

---

## 2. 环境部署：构建、同步与部署所有组件

### A. 镜像构建与同步 (在本地开发环境执行)

1.  **构建 Agent (包含 C++ Spy 库)**：
    ```bash
    cd agent
    make docker-build IMG=compute-sentry-agent:latest
    cd .. # 返回项目根目录
    ```

2.  **构建 Operator**：
    ```bash
    cd operator
    make docker-build IMG=compute-sentry-operator:latest
    cd .. # 返回项目根目录
    ```

3.  **镜像同步到 Minikube** (根据您的环境自行同步，以下是 Minikube 示例)：
    ```bash
    minikube image load compute-sentry-agent:latest
    minikube image load compute-sentry-operator:latest
    ```
    *提示：如果不是 Minikube，请使用适合您 Kubernetes 环境的镜像推送/加载方式。*

### B. 部署 Operator (在 Operator 目录下执行)

```bash
cd operator
# 1. 安装 CRD (自定义资源定义)
make install

# 2. 部署 Operator 控制器管理器
make deploy IMG=compute-sentry-operator:latest
cd .. # 返回项目根目录
```

### C. 部署 Agent (DaemonSet)

Agent 需要在每个节点上运行。
```bash
# 1. 部署 Agent 所需的 RBAC 权限
kubectl apply -f manifests/agent-rbac.yaml

# 2. 部署 Agent DaemonSet
kubectl apply -f manifests/agent-daemonset.yaml
```

**验证部署状态**：
*   检查 Operator Pod 是否运行正常：`kubectl get pods -n compute-sentry-system -l control-plane=controller-manager`
*   检查 Agent Pod 是否在每个节点上运行正常：`kubectl get pods -n compute-sentry-system -l app=compute-sentry-agent`

---

## 3. 核心功能测试：节点级自愈与安全节流

**目标**：验证单个 Pod 异常后的驱逐能力，以及 Operator 基于 Pod 聚合状态的节点污点能力，同时具备自我保护机制。

### 3.1. 准备阶段：配置策略

*   创建一个 `ComputeSentryPolicy`，例如 `test-node-healing-policy.yaml`。此策略将定义当多少个 Pod 异常时触发节点污点。

```yaml
# test-node-healing-policy.yaml
apiVersion: config.aiguard.io/v1
kind: ComputeSentryPolicy
metadata:
  name: test-node-healing-policy
  namespace: default
spec:
  selector:
    matchLabels:
      app: node-healing-test
  actions:
    enableTaint: true
    enableEvict: true
    nodeTaintThreshold:
      minUnhealthyPodsCount: 2 # 示例：至少2个Pod异常才打污点。您可以根据需要调整为 minUnhealthyPodsPercentage。
```
*   应用策略：
    ```bash
    kubectl apply -f test-node-healing-policy.yaml
    ```

### 3.2. 验证单 Pod 驱逐 (不触发节点污点)

*   提交一个 Pod (例如 `pod-test-1.yaml`)，其 Label 匹配上述策略 (例如 `app: node-healing-test`)。

```yaml
# pod-test-1.yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-test-1
  namespace: default
  labels:
    app: node-healing-test
spec:
  containers:
  - name: pause
    image: m.daocloud.io/docker.io/curlimages/curl:latest
    command: ["/bin/sh", "-c", "sleep 3600"] # 保持 Pod 运行
  restartPolicy: Always
```
*   应用 Pod：
    ```bash
    kubectl apply -f pod-test-1.yaml
    ```
*   等待 `pod-test-1` 进入 `Running` 状态：
    ```bash
    kubectl get pod pod-test-1 -n default -o jsonpath='{.status.phase}' # 直到输出 "Running"
    ```
*   获取 `pod-test-1` 所在的节点名称 (后续验证节点污点需要)：
    ```bash
    export NODE_NAME=$(kubectl get pod pod-test-1 -n default -o jsonpath='{.spec.nodeName}')
    echo "Pod 'pod-test-1' 运行在节点: $NODE_NAME"
    ```
*   **模拟 `pod-test-1` 故障** (通过 Annotation，模拟 Agent 检测到异常)：
    ```bash
    kubectl annotate pod pod-test-1 -n default compute-sentry.aiguard.io/health=unhealthy --overwrite
    ```
*   **预期行为**：
    *   `pod-test-1` 被驱逐 (即 Pod 状态变为 `Terminating` 或直接消失)。
    *   节点 **不会** 被打上 `aiguard.io/subhealth` 污点 (因为尚未达到 `minUnhealthyPodsCount: 2` 的阈值)。
*   **验证命令**：
    *   确认 Pod 状态：`kubectl get pod pod-test-1 -n default` (应显示 `Terminating` 或 `NotFound`)
    *   确认节点无污点：`kubectl get nodes $NODE_NAME -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints` (确认输出中不包含 `aiguard.io/subhealth` 污点)

### 3.3. 验证节点污点聚合逻辑

*   创建另外两个 Pod (例如 `pod-test-2.yaml` 和 `pod-test-3.yaml`)，确保它们的 Label 匹配上述策略，并运行在与 `pod-test-1` 相同的节点上。

```yaml
# pod-test-2.yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-test-2
  namespace: default
  labels:
    app: node-healing-test
spec:
  containers:
  - name: pause
    image: m.daocloud.io/docker.io/curlimages/curl:latest
    command: ["/bin/sh", "-c", "sleep 3600"]
  restartPolicy: Always
---
# pod-test-3.yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-test-3
  namespace: default
  labels:
    app: node-healing-test
spec:
  containers:
  - name: pause
    image: m.daocloud.io/docker.io/curlimages/curl:latest
    command: ["/bin/sh", "-c", "sleep 3600"]
  restartPolicy: Always
```
*   应用 Pods：
    ```bash
    kubectl apply -f pod-test-2.yaml
    kubectl apply -f pod-test-3.yaml
    ```
*   等待 `pod-test-2` 和 `pod-test-3` 进入 `Running` 状态。
*   **模拟 `pod-test-2` 故障**：
    ```bash
    kubectl annotate pod pod-test-2 -n default compute-sentry.aiguard.io/health=unhealthy --overwrite
    ```
*   **预期行为**：
    *   `pod-test-2` 被驱逐。
    *   节点 **仍然不会** 被打上 `aiguard.io/subhealth` 污点 (因为目前只有 1 个异常 Pod 曾存在，未达 2 个的阈值)。
*   **模拟 `pod-test-3` 故障**：
    ```bash
    kubectl annotate pod pod-test-3 -n default compute-sentry.aiguard.io/health=unhealthy --overwrite
    ```
*   **预期行为**：
    *   `pod-test-3` 被驱逐。
    *   节点 **将会** 被打上 `aiguard.io/subhealth=true:NoSchedule` 污点 (因为已达到 `minUnhealthyPodsCount: 2` 的阈值)。
*   **验证命令**：
    *   确认 Pod 状态：`kubectl get pod pod-test-2 -n default` 和 `kubectl get pod pod-test-3 -n default` (应显示 `Terminating` 或 `NotFound`)
    *   确认节点已有污点：`kubectl get nodes $NODE_NAME -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints` (确认输出中包含 `aiguard.io/subhealth` 污点)

### 3.4. 验证节点污点自动恢复

*   待所有异常 Pod 被驱逐且不再存在任何匹配策略的 Pod 后 (例如删除所有 `app: node-healing-test` 的 Pod)：
    ```bash
    kubectl delete pod -l app=node-healing-test -n default --ignore-not-found
    ```
*   **预期行为**：节点的 `aiguard.io/subhealth` 污点自动移除。
*   **验证命令**：
    ```bash
    kubectl get nodes $NODE_NAME -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints # 确认输出中不包含 aiguard.io/subhealth 污点
    ```

### 3.5. 验证安全节流

*   **准备阶段**：
    *   创建一个新的策略，例如 `test-throttling-policy.yaml`，其 `actions.nodeTaintThreshold` 可配置为更宽松的阈值（例如 `minUnhealthyPodsCount: 1`），并设置 `enableEvict: true`。
    *   批量创建 10 个以上匹配此策略的 Pod (例如 `app: throttling-test`)。
    *   例如，您可以使用循环命令创建多个 Pod：
    ```bash
    for i in $(seq 1 15); do
      cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: throttling-pod-$i
  namespace: default
  labels:
    app: throttling-test
spec:
  containers:
  - name: pause
    image: curlimages/curl:latest
    command: ["/bin/sh", "-c", "sleep 3600"]
  restartPolicy: Always
EOF
    done
    ```
*   **模拟故障**：
    *   对这 10 个以上 Pod 同时模拟故障（通过 Annotation）。
    ```bash
    for i in $(seq 1 15); do
      kubectl annotate pod throttling-pod-$i -n default compute-sentry.aiguard.io/health=unhealthy --overwrite &
    done
    wait
    ```
*   **预期行为**：Operator 会根据其内部的安全节流配置（例如，默认 5% 的驱逐率）暂停部分 Pod 的驱逐，防止短时间内大量 Pod 被删除。您会观察到部分 Pod 被驱逐，而另一些 Pod 暂时保留。
*   **验证命令**：
    *   检查 Operator 日志，查找节流相关的输出：
        ```bash
        kubectl logs -l control-plane=controller-manager -n compute-sentry-system -c manager | grep "Eviction throttled"
        ```
    *   持续观察 Pod 状态：
        ```bash
        kubectl get pods -l app=throttling-test -n default
        ```
        您会看到一些 Pod 处于 `Terminating` 或已消失，而另一些可能仍处于 `Running` 状态，等待节流解除后被驱逐。

---

## 4. 环境清理

测试完成后，请务必清理您部署的所有资源：

1.  **删除所有测试 Pods 和 Policy**：
    ```bash
    kubectl delete -f test-node-healing-policy.yaml --ignore-not-found
    kubectl delete -f pod-test-1.yaml --ignore-not-found
    kubectl delete -f pod-test-2.yaml --ignore-not-found
    kubectl delete -f pod-test-3.yaml --ignore-not-found
    kubectl delete -f test-throttling-policy.yaml --ignore-not-found # 如果创建了节流测试策略
    kubectl delete pod -l app=node-healing-test -n default --ignore-not-found
    kubectl delete pod -l app=throttling-test -n default --ignore-not-found
    ```
2.  **卸载 Agent DaemonSet 和 RBAC**：
    ```bash
    kubectl delete -f manifests/agent-daemonset.yaml --ignore-not-found
    kubectl delete -f manifests/agent-rbac.yaml --ignore-not-found
    ```
3.  **卸载 Operator**：
    ```bash
    cd operator
    make undeploy
    make uninstall
    cd .. # 返回项目根目录
    ```
4.  **清理镜像** (可选)：
    *   如果您在 Minikube 中加载了镜像，可能需要手动清理：
        ```bash
        minikube image rm compute-sentry-agent:latest
        minikube image rm compute-sentry-operator:latest
        ```
    *   或者直接清理 Docker 镜像：
        ```bash
        docker rmi compute-sentry-agent:latest
        docker rmi compute-sentry-operator:latest
        ```

---

## 5. 常用调试命令

*   **查看 Kubernetes 事件**: `kubectl get events --sort-by=.lastTimestamp`
*   **查看 Operator 日志**: `kubectl logs -l control-plane=controller-manager -n compute-sentry-system -c manager`
*   **查看 Agent 日志** (选择其中一个 Agent Pod)：
    ```bash
    AGENT_POD=$(kubectl get pods -n compute-sentry-system -l app=compute-sentry-agent -o jsonpath='{.items[0].metadata.name}')
    kubectl logs $AGENT_POD -n compute-sentry-system
    ```
*   **检查节点污点**: `kubectl get nodes -o custom-columns=NAME:.metadata.name,TAINTS:.spec.taints`
