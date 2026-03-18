# Compute-Sentry K8S 环境测试指南

## TIP: kubeconfig: ./tests/kubeconfig

## 1. 核心修复 (必须执行)

在 `manifests/agent-daemonset.yaml` 中增加 `/opt/compute-sentry/bin` 的挂载，否则预检脚本无法分发到宿主机。

```yaml
# 修改示例：
        volumeMounts:
        - name: host-lib
          mountPath: /opt/compute-sentry/lib
        - name: host-bin  # 新增
          mountPath: /opt/compute-sentry/bin
        - name: host-uds
          mountPath: /var/run/compute-sentry
      volumes:
      - name: host-lib
        hostPath:
          path: /opt/compute-sentry/lib
          type: DirectoryOrCreate
      - name: host-bin  # 新增
        hostPath:
          path: /opt/compute-sentry/bin
          type: DirectoryOrCreate
```

## 2. 构建与部署流程

### A. 构建镜像
```bash
# 构建 Agent (包含 C++ libspy.so)
cd agent && make docker-build IMG=<your-registry>/compute-sentry-agent:latest

# 构建 Operator
cd ../operator && make docker-build IMG=<your-registry>/compute-sentry-operator:latest
```

### B. 部署组件
1. **部署 Operator**:
   ```bash
   cd operator && make deploy IMG=<your-registry>/compute-sentry-operator:latest
   ```
2. **部署 Agent**:
   更新 `manifests/agent-daemonset.yaml` 中的镜像地址后执行：
   ```bash
   kubectl apply -f manifests/agent-daemonset.yaml
   ```

## 3. 验证步骤

### A. 提交测试 Pod
使用包含注入标记的 Pod 进行测试：
```bash
kubectl apply -f tests/test-injection.yaml
```

### B. 检查注入结果
确认 Pod 包含以下内容：
1.  **InitContainer**: `compute-sentry-precheck` 是否存在且成功运行。
2.  **环境变量**: 业务容器是否有 `LD_PRELOAD=/opt/compute-sentry/lib/libcompute-sentry-spy.so`。
3.  **挂载**: 是否正确挂载了 UDS Socket (`/var/run/compute-sentry`)。

### C. 检查日志
```bash
# 查看预检日志
kubectl logs <pod-name> -c compute-sentry-precheck

# 查看 Agent 采集日志
kubectl logs -l app=compute-sentry-agent -n compute-sentry-system
```
