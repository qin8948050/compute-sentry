package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/fields" // This import is no longer needed
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	configv1 "github.com/qin8948050/compute-sentry/operator/api/v1"
)

const (
	// Annotation keys
	healthAnnotationKey = "compute-sentry.aiguard.io/health"

	// Taint keys
	noScheduleTaintKey = "compute-sentry.aiguard.io/unhealthy"
)

// HealthController reconciles Pods marked as unhealthy and performs remediation actions
type HealthController struct {
	client.Client
	K8sClient kubernetes.Interface
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/eviction,verbs=create
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch

// Reconcile handles unhealthy pods
func (r *HealthController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the pod
	pod := &corev1.Pod{}
	err := r.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil // Pod deleted, nothing to do
		}
		return ctrl.Result{}, err
	}

	// Find matching policy for this pod
	policy, err := r.findMatchingPolicy(ctx, pod)
	if err != nil {
		log.Error(err, "Failed to find matching policy")
		return ctrl.Result{}, err
	}
	// If no policy found, policy will be nil. evaluateNodeTaintDecision will use aggressive defaults.

	// Always evaluate node taint decision when a pod's health status changes
	// This covers both unhealthy and healthy transitions, ensuring proper taint/untaint.
	nodeName := pod.Spec.NodeName
	if nodeName == "" {
		log.Info("Pod has no nodeName, skipping node taint evaluation", "pod", pod.Name)
		return ctrl.Result{}, nil
	}

	// Only proceed with node actions if policy explicitly enables Taint
	// If policy is nil, it means no specific policy matched this pod, aggressive handling for taint should be avoided,
	// because there is no explicit instruction to taint.
	if policy == nil || !policy.Spec.Actions.EnableTaint {
		// If no policy, or policy doesn't enable taint, we should ensure the node is untainted if it was tainted by us.
		// This covers cases where a policy might be removed or updated to disable tainting.
		if err := r.untaintNodeIfTaintedByUs(ctx, nodeName); err != nil {
			log.Error(err, "Failed to ensure node is untainted when no policy or taint disabled", "node", nodeName)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil // No policy, or taint not enabled, so no further taint/untaint decisions here.
	}


	// If Taint is enabled in policy, evaluate the decision
	taintDecision, err := r.evaluateNodeTaintDecision(ctx, nodeName, policy)
	if err != nil {
		log.Error(err, "Failed to evaluate node taint decision", "node", nodeName)
		return ctrl.Result{}, err
	}

	if taintDecision.ShouldTaint {
		if err := r.taintNode(ctx, nodeName); err != nil {
			log.Error(err, "Failed to taint node", "node", nodeName)
			return ctrl.Result{}, err
		}
		log.Info("Tainted node", "node", nodeName, "reason", taintDecision.Reason)
	} else if taintDecision.ShouldUntaint {
		if err := r.untaintNode(ctx, nodeName); err != nil {
			log.Error(err, "Failed to untaint node", "node", nodeName)
			return ctrl.Result{}, err
		}
		log.Info("Untainted node", "node", nodeName, "reason", taintDecision.Reason)
	}

	// Evict pod if enabled (this is pod-specific, not node-aggregated)
	// This logic remains as-is, as eviction is for the specific unhealthy pod.
	healthStatus, exists := pod.Annotations[healthAnnotationKey]
	if exists && healthStatus == "unhealthy" && policy.Spec.Actions.EnableEvict {
		// Throttling check to prevent mass eviction (max 5% of matching pods)
		if err := r.checkEvictionThrottling(ctx, policy); err != nil {
			log.Info("Eviction throttled to prevent cluster-wide avalanche", "reason", err.Error())
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		if err := r.evictPod(ctx, pod); err != nil {
			log.Error(err, "Failed to evict pod")
			return ctrl.Result{}, err
		}
		log.Info("Evicted pod", "namespace", pod.Namespace, "name", pod.Name)
	}

	return ctrl.Result{}, nil
}

// checkEvictionThrottling checks if eviction should be throttled
func (r *HealthController) checkEvictionThrottling(ctx context.Context, policy *configv1.ComputeSentryPolicy) error {
	podList := &corev1.PodList{}
	selector, _ := metav1.LabelSelectorAsSelector(&policy.Spec.Selector)
	if err := r.List(ctx, podList, &client.ListOptions{
		LabelSelector: selector,
	}); err != nil {
		return err
	}

	total := len(podList.Items)
	if total == 0 {
		return nil
	}

	unhealthy := 0
	for _, p := range podList.Items {
		if p.Annotations[healthAnnotationKey] == "unhealthy" {
			unhealthy++
		}
	}

	// Throttle if more than 5% (and at least 2 pods) are unhealthy
	ratio := float64(unhealthy) / float64(total)
	if ratio > 0.05 && unhealthy > 1 {
		return fmt.Errorf("too many unhealthy pods (%d/%d, %.2f%%)", unhealthy, total, ratio*100)
	}

	return nil
}

// findMatchingPolicy finds a ComputeSentryPolicy that matches the pod
func (r *HealthController) findMatchingPolicy(ctx context.Context, pod *corev1.Pod) (*configv1.ComputeSentryPolicy, error) {
	policyList := &configv1.ComputeSentryPolicyList{}
	if err := r.List(ctx, policyList); err != nil {
		return nil, err
	}

	for i := range policyList.Items {
		policy := &policyList.Items[i]
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.Selector)
		if err != nil {
			continue
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			return policy, nil
		}
	}

	return nil, nil
}

// taintNode adds a NoSchedule taint to the node
func (r *HealthController) taintNode(ctx context.Context, nodeName string) error {
	if nodeName == "" {
		return fmt.Errorf("node name is empty")
	}

	node := &corev1.Node{}
	err := r.Get(ctx, client.ObjectKey{Name: nodeName}, node)
	if err != nil {
		return err
	}

	// Check if taint already exists
	for _, taint := range node.Spec.Taints {
		if taint.Key == noScheduleTaintKey {
			// Already tainted
			return nil
		}
	}

	// Add taint using Patch to avoid conflict
	patch := client.MergeFrom(node.DeepCopy())
	node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
		Key:    noScheduleTaintKey,
		Value:  "true",
		Effect: corev1.TaintEffectNoSchedule,
	})

	return r.Patch(ctx, node, patch)
}

// evictPod evicts the pod using the Kubernetes Eviction API
func (r *HealthController) evictPod(ctx context.Context, pod *corev1.Pod) error {
	if r.K8sClient == nil {
		// Fallback to direct delete if clientset is missing (e.g. in some tests)
		return r.Delete(ctx, pod)
	}

	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{},
	}

	return r.K8sClient.PolicyV1().Evictions(pod.Namespace).Evict(ctx, eviction)
}

// untaintNode removes the unhealthy taint from the node
func (r *HealthController) untaintNode(ctx context.Context, nodeName string) error {
	node := &corev1.Node{}
	err := r.Get(ctx, client.ObjectKey{Name: nodeName}, node)
	if err != nil {
		return err
	}

	newTaints := []corev1.Taint{}
	found := false
	for _, taint := range node.Spec.Taints {
		if taint.Key == noScheduleTaintKey {
			found = true
			continue
		}
		newTaints = append(newTaints, taint)
	}

	if !found {
		return nil // No taint to remove
	}

	nodeCopy := node.DeepCopy()
	nodeCopy.Spec.Taints = newTaints
	return r.Patch(ctx, node, client.MergeFrom(nodeCopy))
}

// untaintNodeIfTaintedByUs checks if the node is tainted by us and untaints it
func (r *HealthController) untaintNodeIfTaintedByUs(ctx context.Context, nodeName string) error {
	node := &corev1.Node{}
	err := r.Get(ctx, client.ObjectKey{Name: nodeName}, node)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // Node not found, nothing to untaint
		}
		return fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	isCurrentlyTainted := false
	for _, taint := range node.Spec.Taints {
		if taint.Key == noScheduleTaintKey {
			isCurrentlyTainted = true
			break
		}
	}

	if isCurrentlyTainted {
		return r.untaintNode(ctx, nodeName)
	}
	return nil
}

// nodePodsHealthStatus holds the aggregated health status of pods on a node
type nodePodsHealthStatus struct {
	TotalPods      int
	UnhealthyPods  int
	HealthyPods    int
}

// taintDecision provides the decision to taint or untaint a node
type taintDecision struct {
	ShouldTaint   bool
	ShouldUntaint bool
	Reason        string
}

// getNodePodsHealthStatus fetches all pods on a node and aggregates their health status
func (r *HealthController) getNodePodsHealthStatus(ctx context.Context, nodeName string) (nodePodsHealthStatus, error) {
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.MatchingFields{"spec.nodeName": nodeName},
	}

	if err := r.List(ctx, podList, listOpts...); err != nil {
		return nodePodsHealthStatus{}, fmt.Errorf("failed to list pods on node %s: %w", nodeName, err)
	}

	status := nodePodsHealthStatus{}
	for _, p := range podList.Items {
		status.TotalPods++
		if p.Annotations[healthAnnotationKey] == "unhealthy" {
			status.UnhealthyPods++
		} else {
			status.HealthyPods++
		}
	}
	return status, nil
}

// evaluateNodeTaintDecision evaluates whether a node should be tainted or untainted based on policy and current pod health
func (r *HealthController) evaluateNodeTaintDecision(ctx context.Context, nodeName string, policy *configv1.ComputeSentryPolicy) (taintDecision, error) {
	node := &corev1.Node{}
	err := r.Get(ctx, client.ObjectKey{Name: nodeName}, node)
	if err != nil {
		return taintDecision{}, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	isCurrentlyTainted := false
	for _, taint := range node.Spec.Taints {
		if taint.Key == noScheduleTaintKey {
			isCurrentlyTainted = true
			break
		}
	}

	status, err := r.getNodePodsHealthStatus(ctx, nodeName)
	if err != nil {
		return taintDecision{}, err
	}

	// Default to aggressive: taint if any pod is unhealthy, untaint if all are healthy.
	shouldTaintAggressive := status.UnhealthyPods > 0
	shouldUntaintAggressive := status.UnhealthyPods == 0

	// If no NodeTaintThreshold is specified, use the aggressive default
	if policy == nil || policy.Spec.Actions.NodeTaintThreshold == nil {
		if shouldTaintAggressive && !isCurrentlyTainted {
			return taintDecision{ShouldTaint: true, Reason: "aggressive taint: at least one pod is unhealthy"}, nil
		}
		if shouldUntaintAggressive && isCurrentlyTainted {
			return taintDecision{ShouldUntaint: true, Reason: "aggressive untaint: all pods are healthy"}, nil
		}
		return taintDecision{}, nil // No change needed
	}

	threshold := policy.Spec.Actions.NodeTaintThreshold
	shouldTaintBasedOnThreshold := false
	
	if threshold.MinUnhealthyPodsCount != nil && status.UnhealthyPods >= int(*threshold.MinUnhealthyPodsCount) {
		shouldTaintBasedOnThreshold = true
	}

	if threshold.MinUnhealthyPodsPercentage != nil && status.TotalPods > 0 {
		unhealthyPercentage := (float64(status.UnhealthyPods) / float64(status.TotalPods)) * 100
		if unhealthyPercentage >= float64(*threshold.MinUnhealthyPodsPercentage) {
			shouldTaintBasedOnThreshold = true
		}
	}

	// Make a decision
	if shouldTaintBasedOnThreshold && !isCurrentlyTainted {
		return taintDecision{ShouldTaint: true, Reason: fmt.Sprintf("threshold met: %d/%d pods unhealthy", status.UnhealthyPods, status.TotalPods)}, nil
	}

	if !shouldTaintBasedOnThreshold && isCurrentlyTainted {
		return taintDecision{ShouldUntaint: true, Reason: fmt.Sprintf("threshold no longer met: %d/%d pods unhealthy", status.UnhealthyPods, status.TotalPods)}, nil
	}

	return taintDecision{}, nil // No change needed
}

// SetupWithManager sets up the controller with the Manager
func (r *HealthController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Named("health").
		Complete(r)
}
