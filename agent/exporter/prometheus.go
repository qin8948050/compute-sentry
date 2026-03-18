package exporter

import (
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
		[]string{"type", "node", "switch", "rack"},
	)
	ncclOpsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "compute_sentry_nccl_ops_total",
			Help: "Total number of NCCL operations.",
		},
		[]string{"type", "node", "switch", "rack"},
	)
)

func init() {
	prometheus.MustRegister(ncclLatency)
	prometheus.MustRegister(ncclOpsCount)
}

type Exporter struct {
	addr           string
	nodeName       string
	switchId       string
	rackId         string
}

func NewExporter(addr, node, sw, rack string) *Exporter {
	return &Exporter{
		addr:     addr,
		nodeName: node,
		switchId: sw,
		rackId:   rack,
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

	ncclLatency.WithLabelValues(typeName, e.nodeName, e.switchId, e.rackId).Observe(float64(event.DurationUs))
	ncclOpsCount.WithLabelValues(typeName, e.nodeName, e.switchId, e.rackId).Add(float64(event.Count))
}

func (e *Exporter) Start() error {
	http.Handle("/metrics", promhttp.Handler())
	return http.ListenAndServe(e.addr, nil)
}
