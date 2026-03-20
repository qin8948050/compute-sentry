package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	configv1 "github.com/qin8948050/compute-sentry/operator/api/v1"
)

const (
	// RecoveryMultiplier defines how long to wait after the last violation
	// before marking a pod as healthy again (Multiplier * WindowSize).
	RecoveryMultiplier = 3

	GovernanceConfigAnnotation = "compute-sentry.aiguard.io/governance-config"
	HealthAnnotation           = "compute-sentry.aiguard.io/health"
)

// HealthEvaluator evaluates health based on sliding window of metric events
type HealthEvaluator struct {
	mu               sync.Mutex
	windowSize       time.Duration
	errorCountLimit  int64
	thresholdUs      int64

	// violationTimes stores timestamps of threshold violations per pod
	violationTimes map[string][]time.Time
	// lastSeenTimes stores the last time we saw any metric from a pod
	lastSeenTimes map[string]time.Time
	// unhealthyPods tracks pods that are currently marked as unhealthy
	unhealthyPods map[string]bool
	// podConfigs stores dynamic configuration per pod
	podConfigs map[string]*configv1.GovernanceConfig

	// K8s client for updating pod status
	k8sClient *kubernetes.Clientset
	nodeName  string

	// Channel to receive metric events
	metricsChan chan MetricEvent

	// Stop channel
	stopChan chan struct{}
}

// NewHealthEvaluator creates a new HealthEvaluator
func NewHealthEvaluator(windowSizeSec int64, errorCountLimit int64, thresholdUs int64) *HealthEvaluator {
	return &HealthEvaluator{
		windowSize:      time.Duration(windowSizeSec) * time.Second,
		errorCountLimit: errorCountLimit,
		thresholdUs:     thresholdUs,
		violationTimes:  make(map[string][]time.Time),
		lastSeenTimes:   make(map[string]time.Time),
		unhealthyPods:   make(map[string]bool),
		podConfigs:      make(map[string]*configv1.GovernanceConfig),
		metricsChan:     make(chan MetricEvent, 1000),
		stopChan:        make(chan struct{}),
	}
}

// Start initializes K8s client and starts the evaluation loop
func (e *HealthEvaluator) Start(nodeName string) error {
	e.nodeName = nodeName

	// Initialize in-cluster K8s client
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %v", err)
	}
	e.k8sClient = clientset

	// Initial sync of pods on this node to populate policy cache
	e.syncNodePods(context.Background())
	go e.periodicSync()

	go e.runLoop()
	return nil
}

// MetricsChan returns the channel to receive metric events
func (e *HealthEvaluator) MetricsChan() chan MetricEvent {
	return e.metricsChan
}

// Stop stops the evaluator
func (e *HealthEvaluator) Stop() {
	close(e.stopChan)
}

func (e *HealthEvaluator) runLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopChan:
			return
		case event := <-e.metricsChan:
			e.processEvent(event)
		case <-ticker.C:
			e.checkRecoveryAndCleanup()
		}
	}
}

func bytesToString(b []byte) string {
	n := bytes.IndexByte(b, 0)
	if n == -1 {
		return string(b)
	}
	return string(b[:n])
}

func (e *HealthEvaluator) processEvent(event MetricEvent) {
	// Only evaluate NCCL_ALL_REDUCE for now
	if event.Type != NCCL_ALL_REDUCE {
		return
	}

	podName := bytesToString(event.PodName[:])
	podNamespace := bytesToString(event.PodNamespace[:])

	if podName == "" || podName == "unknown" {
		return
	}

	podKey := fmt.Sprintf("%s/%s", podNamespace, podName)
	now := time.Now()

	e.mu.Lock()
	e.lastSeenTimes[podKey] = now
	config, ok := e.podConfigs[podKey]
	e.mu.Unlock()

	if !ok {
		// Attempt to fetch and cache if not in cache (first time seeing this pod)
		config = e.fetchAndCachePodConfig(podNamespace, podName)
	}

	// Fallback to global defaults if no specific policy is found
	threshold := e.thresholdUs
	window := e.windowSize
	limit := e.errorCountLimit

	if config != nil {
		threshold = config.Thresholds.MaxNCCLLatencyUs
		window = time.Duration(config.EvalConfig.WindowSize) * time.Second
		limit = config.EvalConfig.ErrorCountLimit
	}

	// Check if duration exceeds threshold
	if event.DurationUs > threshold {
		e.mu.Lock()
		// Add violation timestamp
		e.violationTimes[podKey] = append(e.violationTimes[podKey], now)

		// Check if violations exceed limit
		violations := e.countViolationsLocked(podKey, window)

		if int64(violations) > limit && !e.unhealthyPods[podKey] {
			e.unhealthyPods[podKey] = true
			e.mu.Unlock()
			e.markPodStatus(podName, podNamespace, "unhealthy")
			return
		}
		e.mu.Unlock()
	}
}

func (e *HealthEvaluator) countViolationsLocked(podKey string, windowSize time.Duration) int {
	cutoff := time.Now().Add(-windowSize)
	count := 0
	for _, t := range e.violationTimes[podKey] {
		if t.After(cutoff) {
			count++
		}
	}
	return count
}

func (e *HealthEvaluator) checkRecoveryAndCleanup() {
	e.mu.Lock()
	now := time.Now()

	var podsToRecover []string

	// 1. Cleanup old violations and detect recovery
	for podKey, times := range e.violationTimes {
		// Get pod-specific window size if available
		windowSize := e.windowSize
		if config, ok := e.podConfigs[podKey]; ok {
			windowSize = time.Duration(config.EvalConfig.WindowSize) * time.Second
		}
		cutoff := now.Add(-windowSize)
		recoveryCutoff := now.Add(-windowSize * RecoveryMultiplier)

		var valid []time.Time
		latestViolation := time.Time{}
		for _, t := range times {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
			if t.After(latestViolation) {
				latestViolation = t
			}
		}

		if len(valid) == 0 {
			delete(e.violationTimes, podKey)
			// If no violations in window and it was unhealthy, check if it has been quiet long enough
			if e.unhealthyPods[podKey] && (latestViolation.IsZero() || latestViolation.Before(recoveryCutoff)) {
				podsToRecover = append(podsToRecover, podKey)
			}
		} else {
			e.violationTimes[podKey] = valid
		}
	}

	// 2. Cleanup stale pods from lastSeenTimes
	for podKey, lastSeen := range e.lastSeenTimes {
		windowSize := e.windowSize
		if config, ok := e.podConfigs[podKey]; ok {
			windowSize = time.Duration(config.EvalConfig.WindowSize) * time.Second
		}
		recoveryCutoff := now.Add(-windowSize * RecoveryMultiplier)

		if lastSeen.Before(recoveryCutoff) {
			delete(e.lastSeenTimes, podKey)
			delete(e.unhealthyPods, podKey)
			delete(e.podConfigs, podKey)
		}
	}
	e.mu.Unlock()

	// 3. Mark pods as healthy
	for _, podKey := range podsToRecover {
		parts := strings.Split(podKey, "/")
		if len(parts) == 2 {
			e.markPodStatus(parts[1], parts[0], "healthy")
			e.mu.Lock()
			delete(e.unhealthyPods, podKey)
			e.mu.Unlock()
		}
	}
}

func (e *HealthEvaluator) markPodStatus(podName, podNamespace, status string) {
	if e.k8sClient == nil || podName == "" || podName == "unknown" {
		return
	}

	ctx := context.Background()

	// Use Patch for lightweight updates and to avoid conflicts
	patchData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				HealthAnnotation: status,
			},
		},
	}
	patchBytes, _ := json.Marshal(patchData)

	_, err := e.k8sClient.CoreV1().Pods(podNamespace).Patch(ctx, podName, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		fmt.Printf("Failed to patch pod %s/%s status to %s: %v\n", podNamespace, podName, status, err)
	} else {
		fmt.Printf("Marked pod %s/%s as %s (via patch)\n", podNamespace, podName, status)
	}
}

func (e *HealthEvaluator) fetchAndCachePodConfig(podNamespace, podName string) *configv1.GovernanceConfig {
	if e.k8sClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pod, err := e.k8sClient.CoreV1().Pods(podNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil
	}

	configStr, ok := pod.Annotations[GovernanceConfigAnnotation]
	if !ok {
		return nil
	}

	var config configv1.GovernanceConfig
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.podConfigs[fmt.Sprintf("%s/%s", podNamespace, podName)] = &config
	return &config
}

func (e *HealthEvaluator) syncNodePods(ctx context.Context) {
	if e.k8sClient == nil || e.nodeName == "" || e.nodeName == "unknown" {
		return
	}

	// Scoped listing: only pods on this node
	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", e.nodeName),
	}
	pods, err := e.k8sClient.CoreV1().Pods("").List(ctx, listOptions)
	if err != nil {
		fmt.Printf("Failed to list pods on node %s: %v\n", e.nodeName, err)
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for _, pod := range pods.Items {
		configStr, ok := pod.Annotations[GovernanceConfigAnnotation]
		if ok {
			var config configv1.GovernanceConfig
			if err := json.Unmarshal([]byte(configStr), &config); err == nil {
				e.podConfigs[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = &config
			}
		}
	}
}

func (e *HealthEvaluator) periodicSync() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.stopChan:
			return
		case <-ticker.C:
			e.syncNodePods(context.Background())
		}
	}
}

// SetK8sClient allows injecting a custom K8s client (useful for testing)
func (e *HealthEvaluator) SetK8sClient(client *kubernetes.Clientset) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.k8sClient = client
}

// SetNodeName sets the node name
func (e *HealthEvaluator) SetNodeName(nodeName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nodeName = nodeName
}
