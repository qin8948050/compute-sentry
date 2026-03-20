package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/qin8948050/compute-sentry/agent/collector"
	"github.com/qin8948050/compute-sentry/agent/exporter"
)

func main() {
	udsPath := flag.String("uds-path", "/var/run/compute-sentry/spy.sock", "Path to UDS socket")
	metricsAddr := flag.String("metrics-addr", ":9091", "Address to expose Prometheus metrics")
	libSrc := flag.String("lib-src", "/app/lib/libcompute-sentry-spy.so", "Source path of the spy library inside container")
	libDest := flag.String("lib-dest", "/opt/compute-sentry/lib/libcompute-sentry-spy.so", "Destination path on the host")
	scriptSrc := flag.String("script-src", "/app/bin/precheck.sh", "Source path of the precheck script inside container")
	scriptDest := flag.String("script-dest", "/opt/compute-sentry/bin/precheck.sh", "Destination path of the precheck script on the host")

	// Health evaluation parameters
	windowSize := flag.Int64("window-size", 10, "Sliding window size in seconds for health evaluation")
	errorCountLimit := flag.Int64("error-count-limit", 5, "Max violations within window to trigger unhealthy")
	thresholdUs := flag.Int64("threshold-us", 500, "Threshold in microseconds for slow operation detection")

	flag.Parse()

	log.Println("Starting Compute-Sentry Agent...")

	// 1. 获取拓扑标签
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		log.Println("WARNING: NODE_NAME env not set, topology will be unknown")
		nodeName = "unknown"
	}

	sw, rack, gpu := getTopology(nodeName)
	log.Printf("Node Topology: Node=%s, Switch=%s, Rack=%s, GPU=%s", nodeName, sw, rack, gpu)

	// 2. Library Distribution Logic
	if err := distributeFile(*libSrc, *libDest); err != nil {
		log.Fatalf("Failed to distribute library: %v", err)
	}
	log.Printf("Library distributed to %s", *libDest)

	// 3. Script Distribution Logic
	if err := distributeFile(*scriptSrc, *scriptDest); err != nil {
		log.Fatalf("Failed to distribute script: %v", err)
	}
	log.Printf("Script distributed to %s", *scriptDest)

	// 4. Start Collector
	col := collector.NewCollector(*udsPath)
	if err := col.Start(); err != nil {
		log.Fatalf("Failed to start collector: %v", err)
	}
	log.Printf("Collector listening on %s", *udsPath)

	// 5. Start Exporter
	exp := exporter.NewExporter(*metricsAddr, nodeName, sw, rack, gpu)
	go func() {
		log.Printf("Exporter listening on %s/metrics", *metricsAddr)
		if err := exp.Start(); err != nil {
			log.Fatalf("Exporter failed: %v", err)
		}
	}()

	// 6. Start HealthEvaluator
	evaluator := collector.NewHealthEvaluator(*windowSize, *errorCountLimit, *thresholdUs)
	if err := evaluator.Start(nodeName); err != nil {
		log.Printf("WARNING: Failed to start health evaluator: %v (continuing without health evaluation)", err)
	} else {
		log.Printf("HealthEvaluator started: window=%ds, limit=%d, threshold=%dus",
			*windowSize, *errorCountLimit, *thresholdUs)
	}

	// 7. Main Loop: Forward events to exporter and evaluator
	for event := range col.MetricsChan {
		exp.Record(event)
		// Also send to evaluator for health checking
		select {
		case evaluator.MetricsChan() <- event:
		default:
			// Channel full, skip
		}
	}
}

func getTopology(nodeName string) (sw, rack, gpu string) {
	sw, rack, gpu = "unknown", "unknown", "unknown"
	if nodeName == "unknown" {
		return
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Failed to get in-cluster config: %v", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Failed to create clientset: %v", err)
		return
	}

	node, err := clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get node %s info: %v", nodeName, err)
		return
	}

	if s, ok := node.Labels["topology.aiguard.io/switch"]; ok {
		sw = s
	}
	if r, ok := node.Labels["topology.aiguard.io/rack"]; ok {
		rack = r
	}
	if g, ok := node.Labels["topology.aiguard.io/gpu-model"]; ok {
		gpu = g
	}
	return
}

func distributeFile(src, dest string) error {
	// Ensure destination directory exists
	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create dest dir: %v", err)
	}

	// Copy file
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source lib: %v", err)
	}
	defer source.Close()

	destination, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create dest lib: %v", err)
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return fmt.Errorf("failed to copy lib: %v", err)
	}

	// Make it readable for anyone (so LD_PRELOAD can find it)
	return os.Chmod(dest, 0755)
}
