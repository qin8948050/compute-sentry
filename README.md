# Compute-Sentry

**面向大规模智算集群的确定性治理与全栈观测平台**

Compute-Sentry 是一款 K8s 原生的算力治理系统，旨在通过非侵入劫持与确定性准入技术，消除 AI 训练集群中的硬件亚健康（Gray Failure）与性能黑盒问题。

---

## 🚀 快速导航 (Documentation)

*   [**项目方案书 (Technical Scheme)**](./docs/Compute-Sentry项目方案.md) - 深度解析核心痛点、技术创新与总体架构。
*   [**开发路线与业务逻辑 (Roadmap)**](./docs/Compute-Sentry开发路线与业务逻辑.md) - 四阶段开发计划、详细业务流转及任务状态跟踪。
*   [**功能与业务流程 (Features & Flow)**](./docs/FEATURES_AND_FLOW.md) - 已完成功能清单与核心业务流程说明。
*   [**开发规范与准则 (Guidelines)**](./docs/DEVELOPMENT.md) - 参与项目贡献必须遵循的 C++/Go 编码标准与性能原则。
*   [**AI 协作手册 (GEMINI.md)**](./GEMINI.md) - 专门用于 AI 开发过程中的任务追踪、进度标记与提交参考。

---

## 🏗️ 核心架构 (Architecture Overview)

Compute-Sentry 采用“感知 (Spy) - 控制 (Operator) - 观测 (Grafana)”三层闭环架构：

1.  **感知层 (Compute-Sentry-Spy)**: 基于 `LD_PRELOAD` 的非侵入劫持库，精准捕获 NCCL/CUDA 算子延迟。
2.  **控制层 (Compute-Sentry-Operator)**: 管理 Pod 生命周期，执行确定性准入（Init-check）与动态注入。
3.  **观测层 (Compute-Sentry-Agent & Dashboard)**: 节点级数据汇聚，通过 Prometheus + Grafana 实现 3D 拓扑可视化。

---

## 🛠️ 技术栈 (Tech Stack)

*   **Languages**: C++17, Go
*   **Infrastructure**: Kubernetes, NCCL, CUDA
*   **Observability**: Prometheus, Grafana, VictoriaMetrics
*   **Communication**: Unix Domain Socket (UDS)

---

## 📝 许可证 (License)

Copyright © 2026 Compute-Sentry Team. All rights reserved.
