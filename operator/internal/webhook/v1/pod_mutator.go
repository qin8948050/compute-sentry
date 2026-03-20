package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configv1 "github.com/qin8948050/compute-sentry/operator/api/v1"
)

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.kf.io,admissionReviewVersions=v1

// PodMutator annotates Pods
type PodMutator struct {
	Client  client.Client
	Decoder admission.Decoder
	// policyList caches all ComputeSentryPolicy for selector matching
	policyList *configv1.ComputeSentryPolicyList
}

// Handle handles admission requests
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := m.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Fetch all ComputeSentryPolicy to match against pod labels
	policyList := &configv1.ComputeSentryPolicyList{}
	if err := m.Client.List(ctx, policyList); err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Find matching policy based on selector
	var matchedPolicy *configv1.ComputeSentryPolicy
	for i := range policyList.Items {
		policy := &policyList.Items[i]
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.Selector)
		if err != nil {
			continue
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			matchedPolicy = policy
			break
		}
	}

	// Determine injection status using priority:
	// 1. Pod Annotation (Explicit Override)
	// 2. Matched Policy Enabled Flag
	// 3. Default: Off
	injectValue, hasAnnotation := pod.Annotations["compute-sentry.aiguard.io/inject"]
	var shouldInject bool

	if hasAnnotation {
		shouldInject = (injectValue == "true")
	} else if matchedPolicy != nil {
		shouldInject = matchedPolicy.Spec.SpyConfig.Enabled
	} else {
		shouldInject = false
	}

	if !shouldInject {
		return admission.Allowed("injection not enabled for this pod")
	}

	// Prepare Governance Config and Env Vars ONLY if policy is matched
	var govConfig *configv1.GovernanceConfig
	precheckEnvs := []corev1.EnvVar{}

	if matchedPolicy != nil {
		govConfig = &configv1.GovernanceConfig{
			Thresholds: matchedPolicy.Spec.Thresholds,
			EvalConfig: matchedPolicy.Spec.EvalConfig,
		}

		// Prepare Precheck Envs from Policy if specified
		if govConfig.Thresholds.MinP2PBandwidthGbps != 0 {
			precheckEnvs = append(precheckEnvs, corev1.EnvVar{
				Name:  "PRECHECK_MIN_P2P_GBPS",
				Value: fmt.Sprintf("%d", govConfig.Thresholds.MinP2PBandwidthGbps),
			})
		}
		if govConfig.Thresholds.MinHbmBandwidthGbps != 0 {
			precheckEnvs = append(precheckEnvs, corev1.EnvVar{
				Name:  "PRECHECK_MIN_HBM_GBPS",
				Value: fmt.Sprintf("%d", govConfig.Thresholds.MinHbmBandwidthGbps),
			})
		}
	}

	// Inject LD_PRELOAD
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		injected := false
		for j := range container.Env {
			if container.Env[j].Name == "LD_PRELOAD" {
				container.Env[j].Value = "/opt/compute-sentry/lib/libcompute-sentry-spy.so:" + container.Env[j].Value
				injected = true
				break
			}
		}
		if !injected {
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "LD_PRELOAD",
				Value: "/opt/compute-sentry/lib/libcompute-sentry-spy.so",
			})
		}

		// Inject UDS path
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "COMPUTE_SENTRY_UDS_PATH",
			Value: "/var/run/compute-sentry/spy.sock",
		})

		// Inject Pod Info for Identification
		container.Env = append(container.Env, corev1.EnvVar{
			Name: "COMPUTE_SENTRY_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name: "COMPUTE_SENTRY_POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		})

		// Add VolumeMounts
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "compute-sentry-uds",
			MountPath: "/var/run/compute-sentry",
		})
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "compute-sentry-lib",
			MountPath: "/opt/compute-sentry/lib",
			ReadOnly:  true,
		})
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "compute-sentry-uds",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/run/compute-sentry",
			},
		},
	})
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "compute-sentry-lib",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/opt/compute-sentry/lib",
			},
		},
	})

	// Inject InitContainer for Pre-check
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:    "compute-sentry-precheck",
		Image:   "m.daocloud.io/docker.io/busybox:latest", // In real world, use an image with diagnostic tools
		Command: []string{"sh", "-c", "/opt/compute-sentry/bin/precheck.sh"},
		Env:     precheckEnvs,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "compute-sentry-bin",
				MountPath: "/opt/compute-sentry/bin",
				ReadOnly:  true,
			},
		},
	})

	// Add Binary Volume (HostPath from Agent)
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "compute-sentry-bin",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/opt/compute-sentry/bin",
			},
		},
	})

	// Add annotation to indicate injection happened
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations["compute-sentry.aiguard.io/injected"] = "true"

	// Serialize Governance Config and inject as Annotation ONLY if policy exists
	if govConfig != nil {
		configBytes, err := json.Marshal(govConfig)
		if err == nil {
			pod.Annotations["compute-sentry.aiguard.io/governance-config"] = string(configBytes)
		}
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// PodMutator implements admission.DecoderInstaller.
func (m *PodMutator) InjectDecoder(d admission.Decoder) error {
	m.Decoder = d
	return nil
}
