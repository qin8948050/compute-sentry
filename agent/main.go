package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

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

	flag.Parse()

	log.Println("Starting Compute-Sentry Agent...")

	// 1. Library Distribution Logic
	if err := distributeFile(*libSrc, *libDest); err != nil {
		log.Fatalf("Failed to distribute library: %v", err)
	}
	log.Printf("Library distributed to %s", *libDest)

	// 2. Script Distribution Logic
	if err := distributeFile(*scriptSrc, *scriptDest); err != nil {
		log.Fatalf("Failed to distribute script: %v", err)
	}
	log.Printf("Script distributed to %s", *scriptDest)

	// 2. Start Collector
	col := collector.NewCollector(*udsPath)
	if err := col.Start(); err != nil {
		log.Fatalf("Failed to start collector: %v", err)
	}
	log.Printf("Collector listening on %s", *udsPath)

	// 3. Start Exporter
	exp := exporter.NewExporter(*metricsAddr)
	go func() {
		log.Printf("Exporter listening on %s/metrics", *metricsAddr)
		if err := exp.Start(); err != nil {
			log.Fatalf("Exporter failed: %v", err)
		}
	}()

	// 4. Main Loop: Forward events to exporter
	for event := range col.MetricsChan {
		exp.Record(event)
	}
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
