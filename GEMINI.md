# GEMINI.md - AI 协作开发索引与任务手册

本文件为 Antigravity/Gemini 专用的工程执行手册，用于追踪开发进度、规范 AI 提交逻辑以及建立高效的“人机协作”开发流。

## 1. 任务完成度看板 (Project Progress Dashboard)

> [!TIP]
> 每次完成子任务后，AI 将同步更新此看板及 `./docs/Compute-Sentry开发路线与业务逻辑.md` 中的状态。例如：
### Phase 1: 感知原型期 (Perception Layer)
- [x] **任务 1.1**: C++ Spy 骨架搭建与 `LD_PRELOAD` 劫持逻辑实现
- [x] **任务 1.2**: 无锁异步队列与 UDS 通信系统实现
- [x] **任务 1.3**: Mock 训练环境搭建与劫持功能验证
- [x] **任务 1.4**: **Phase 1 Demo**: 验证非侵入式劫持与数据产出

### Phase 2: K8s 治理集成期 (Governance Integration)
- [x] **任务 2.1**: Compute-Sentry Operator 骨架搭建与 Webhook 实现
- [x] **任务 2.2**: Pod Mutator (LD_PRELOAD & UDS) 注入逻辑实现
- [x] **任务 2.3**: InitContainer 算力硬准入预检程序实现
- [x] **任务 2.4**: **Phase 2 Demo**: 验证 K8s 原生自动注入与准入控制 (Unit Tests Verified)
- [ ] **任务 2.5**: **跨节点自动部署**: (计划在 Phase 3 通过 Agent 自动化实现)

### Phase 3: 全栈观测闭环期 (Observability)
- [ ] **任务 3.1**: Compute-Sentry Agent (DaemonSet) 实现与 .so 自动分发
- [ ] **任务 3.2**: Prometheus Metrics 暴露与指标聚合逻辑实现
- [ ] **任务 3.3**: Grafana 3D 拓扑看板与物理拓扑映射实现
- [ ] **任务 3.4**: **Phase 3 Demo**: 验证从算子劫持到全栈看板的实时观测

---

## 2. 提交规范参考 (Commit Standards)

所有由 AI 执行的提交必须遵循以下 Conventional Commits 规范，且 **Message 必须使用英文**：

| 格式 | 适用场景 | 示例 |
| :--- | :--- | :--- |
| `feat: <msg>` | 新功能实现 | `feat: implement ncclAllReduce hijacking` |
| `fix: <msg>` | 错误修复 | `fix: resolve uds socket leak in background thread` |
| `perf: <msg>` | 性能优化 | `perf: optimized lock-free queue memory barrier` |
| `docs: <msg>` | 文档更新 | `docs: add technical introduction to scheme` |
| `refactor: <msg>`| 重构逻辑 | `refactor: clean up nccl symbol loading logic` |

---

## 3. AI 开发流规范 (AI Workflow)

1.  **Context 优先**: AI 在每次启动开发任务前，必须首先阅读 [Compute-Sentry项目方案.md](./docs/Compute-Sentry项目方案.md)、[DEVELOPMENT.md](./docs/DEVELOPMENT.md) 以及 [Compute-Sentry开发路线与业务逻辑.md](./docs/Compute-Sentry开发路线与业务逻辑.md)，确保完全理解业务逻辑、开发规范与当前进度。
2.  **原子化执行**: 保持颗粒度，每一个 `[ ]` 任务对应一次完整的代码生成、自验与状态标记。
3.  **安全性检查**: 任何生成的劫持逻辑必须带有 `try-catch` 或退避机制，防止干扰业务进程。

---

## 4. 快速查看索引 (Quick Links)

- **核心方案**: [Compute-Sentry项目方案.md](./docs/Compute-Sentry项目方案.md)
- **开发路线与业务逻辑**: [Compute-Sentry开发路线与业务逻辑.md](./docs/Compute-Sentry开发路线与业务逻辑.md)
- **代码规范**: [DEVELOPMENT.md](./docs/DEVELOPMENT.md)
