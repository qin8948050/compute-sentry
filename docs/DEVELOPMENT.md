# Compute-Sentry 开发规范 (Development Guidelines)

为了保证 Compute-Sentry 这一高性能智算治理平台的代码质量与极致性能，本项目深度参考并遵循业界大厂（Google, Uber, CNCF）的工程实践标准。

## 1. 核心指南与基识 (Reference Standards)

本项目代码风格必须严格遵守以下标准：
*   **C++ 核心规范**: [Google C++ Style Guide](https://google.github.io/styleguide/cppguide.html)
*   **Go 核心规范**: [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md) 及 [Effective Go](https://golang.org/doc/effective_go)
*   **K8s 设计模式**: 遵循 [Kubernetes API Conventions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md)

---

## 2. C++ (Spy 层) 特色规范

### 2.1 极致性能与低干扰 (Low Overhead)
*   **栈内存优先**: 劫持库内尽量减少堆内存分配，防止触发隐式的锁竞争。
*   **内联与编译优化**: 对高频劫持函数使用 `inline`，并启用 `-O3` 与 `LTO`（链接时优化）。
*   **ABI 兼容性**: 避免在公共接口中使用复杂的 STL 容器，确保 `libCompute-Sentry-Spy.so` 能在不同环境的 AI 训练容器中稳定预加载。

### 2.2 安全性 (Safety)
*   **RAII 机制**: 严格遵循 RAII。所有系统资源（FD, Socket, Memory）必须由智能指针或包装类自动管理，绝对禁止裸指针手动释放。

---

## 3. Go (控制面) 特色规范

### 3.1 健壮的并发模型 (Concurrency)
*   **Uber 风格并发**: 优先使用原子操作和 Channel 进行同步；避免在长时间循环中使用闭包 goroutine 带来的变量捕捉陷阱。
*   **防御式退出**: 所有 Goroutine 必须监听 `ctx.Done()`，确保在 Pod 销毁或组件热更新时能优雅退出，不留僵尸进程。

### 3.2 错误处理 (Error Handling)
*   **上下文关联**: 禁止直接返回 `return err`。必须使用 `fmt.Errorf("context message: %w", err)` 包裹错误，实现完整的错误溯约链。

---

## 4. 基础设施治理原则 (Infra Principles)

*   **观测优先 (Observability-First)**: 新增任何逻辑均需同步考虑对应的 Prometheus Metrics 或 OpenTelemetry Trace 指标。
*   **故障隔离 (Failure Isolation)**: Compute-Sentry 自身的崩溃**绝对不能**导致业务训练 Pod 崩溃。劫持逻辑必须包含完备的 `try-catch/recover` 机制及 Bypass 退避逻辑。

---

## 5. 提交规范 (Git Flow)

*   commit信息要用英文，并遵循 **Conventional Commits** 规范：
    *   `feat`: 新功能
    *   `fix`: 修复 Bug
    *   `docs`: 文档改动
    *   `perf`: 性能优化
    *   `refactor`: 代码重构
