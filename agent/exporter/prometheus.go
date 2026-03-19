package exporter

import (
	"bytes"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/qin8948050/compute-sentry/agent/collector"
)

var (
	ncclLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "compute_sentry_nccl_latency_us",
			Help:    "Latency of NCCL operations in microseconds.",
			Buckets: prometheus.ExponentialBuckets(10, 2, 10), // 10us to 10ms approx
		},
		[]string{"type", "node", "switch", "rack", "node_gpu_model", "runtime_gpu_model"},
	)
	ncclOpsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "compute_sentry_nccl_ops_total",
			Help: "Total number of NCCL operations.",
		},
		[]string{"type", "node", "switch", "rack", "node_gpu_model", "runtime_gpu_model"},
	)
)

func init() {
	prometheus.MustRegister(ncclLatency)
	prometheus.MustRegister(ncclOpsCount)
}

type Exporter struct {
	addr          string
	nodeName      string
	switchId      string
	rackId        string
	nodeGpuModel  string // 从 Node Label 获取的节点级 GPU 型号
}

func NewExporter(addr, node, sw, rack, nodeGpuModel string) *Exporter {
	return &Exporter{
		addr:          addr,
		nodeName:      node,
		switchId:      sw,
		rackId:        rack,
		nodeGpuModel:  nodeGpuModel,
	}
}

func (e *Exporter) Record(event collector.MetricEvent) {
	typeName := "unknown"
	switch event.Type {
	case collector.NCCL_ALL_REDUCE:
		typeName = "ncclAllReduce"
	case collector.CUDA_MALLOC:
		typeName = "cudaMalloc"
	case collector.CUDA_MEMCPY:
		typeName = "cudaMemcpy"
	}

	// runtimeGpuModel: 从 Spy 运行时检测获取（可能是 MIG 实例）
	runtimeGpuModel := string(bytes.TrimRight(event.GPUModel[:], "\x00"))
	if runtimeGpuModel == "" {
		runtimeGpuModel = "unknown"
	}

	// nodeGpuModel: 从 Node Label 获取的节点级 GPU 型号
	nodeGpuModel := e.nodeGpuModel
	if nodeGpuModel == "" {
		nodeGpuModel = "unknown"
	}

	labels := []string{typeName, e.nodeName, e.switchId, e.rackId, nodeGpuModel, runtimeGpuModel}

	ncclLatency.WithLabelValues(labels...).Observe(float64(event.DurationUs))
	ncclOpsCount.WithLabelValues(labels...).Add(float64(event.Count))
}

func (e *Exporter) Start() error {
	http.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(e.addr, nil)
}
