package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
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
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check health status
	healthStatus, exists := pod.Annotations[healthAnnotationKey]
	if !exists {
		return ctrl.Result{}, nil
	}

	if healthStatus == "healthy" {
		// Attempt to untaint node if this was the last unhealthy pod
		if err := r.untaintNodeIfPossible(ctx, pod.Spec.NodeName); err != nil {
			log.Error(err, "Failed to untaint node", "node", pod.Spec.NodeName)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if healthStatus != "unhealthy" {
		return ctrl.Result{}, nil
	}

	log.Info("Processing unhealthy pod", "namespace", pod.Namespace, "name", pod.Name, "node", pod.Spec.NodeName)

	// Find matching policy for this pod
	policy, err := r.findMatchingPolicy(ctx, pod)
	if err != nil {
		log.Error(err, "Failed to find matching policy")
		return ctrl.Result{}, err
	}

	if policy == nil {
		log.Info("No matching policy found for pod, skipping remediation")
		return ctrl.Result{}, nil
	}

	// Execute actions based on policy
	actions := policy.Spec.Actions

	// Taint node if enabled
	if actions.EnableTaint {
		if err := r.taintNode(ctx, pod.Spec.NodeName); err != nil {
			log.Error(err, "Failed to taint node")
			return ctrl.Result{}, err
		}
		log.Info("Tainted node", "node", pod.Spec.NodeName)
	}

	// Evict pod if enabled
	if actions.EnableEvict {
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

	eviction := &policyv1beta1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{},
	}

	return r.K8sClient.CoreV1().Pods(pod.Namespace).Evict(ctx, eviction)
}

// untaintNodeIfPossible removes the unhealthy taint from the node if no unhealthy pods remain
func (r *HealthController) untaintNodeIfPossible(ctx context.Context, nodeName string) error {
	if nodeName == "" {
		return nil
	}

	// 1. Check if there are any other unhealthy pods on this node
	podList := &corev1.PodList{}
	err := r.List(ctx, podList, &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName),
	})
	if err != nil {
		return err
	}

	for _, p := range podList.Items {
		if p.Annotations[healthAnnotationKey] == "unhealthy" {
			// Still have unhealthy pods, keep the taint
			return nil
		}
	}

	// 2. No unhealthy pods, remove the taint
	node := &corev1.Node{}
	err = r.Get(ctx, client.ObjectKey{Name: nodeName}, node)
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
	return r.Update(ctx, nodeCopy)
}

// SetupWithManager sets up the controller with the Manager
func (r *HealthController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Named("health").
		Complete(r)
}
