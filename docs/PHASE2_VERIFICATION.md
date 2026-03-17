# Phase 2 Verification & Mocking Guide

This guide describes how to verify the K8s Operator, Webhook, and Pre-check logic developed in Phase 2.

## 1. 逻辑 Mock (Unit Testing)
This is the most efficient way to verify that the **Injection Logic** is correct without needing a real K8s cluster or TLS certificates.

### How it works:
We simulate an `AdmissionRequest` containing a raw Pod JSON, pass it to our `PodMutator`, and verify that the generated JSON Patch correctly adds:
- `LD_PRELOAD` environment variable.
- `InitContainer` for pre-check.
- UDS Volume and VolumeMounts.

### Run the Mock Test:
```bash
cd operator
go test -v ./internal/webhook/v1/...
```
**Verification Result**: You should see `PASS: TestPodMutator_Handle`.

---

## 2. Pre-check 脚本本地验证
Verify the logic of the hard-admission pre-check script.

### Run the Script:
```bash
cd operator/precheck
./precheck.sh
```
**Expectation**:
- It should detect your local GPU (if any) or skip gracefully.
- It should output "All checks passed. Allowing training process to start."

---

## 3. 集群集成验证 (K8s Integration)
Live verification using the provided `kubeconfig`.

### Step A: CRD 注册
```bash
export KUBECONFIG=$(pwd)/tests/kubeconfig
cd operator
make install
```
**Verification**: Run `kubectl get crd | grep computesentrypolicy`.

### Step B: 本地运行 Operator
```bash
export ENABLE_WEBHOOKS=false # 暂时关闭 Webhook 功能进行 Controller 测试
make run
```

> [!NOTE]
> Webhook 完整测试需要 TLS 证书和 K8s 能够访问的 Endpoint。通常在开发阶段，我们通过上面的 **逻辑 Mock** 确保代码逻辑正确，集成阶段再进行全链路测试。

---

## 4. 总结
Phase 2 的核心逻辑已经通过单元测试验证。
- **自动化注入**: 逻辑正确，支持 `LD_PRELOAD` 和 UDS 挂载。
- **算力准入**: `precheck.sh` 逻辑就绪。
- **API 定义**: CRD 结构已生成且支持 `DeepCopy`。
