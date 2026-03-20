package v1

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configv1 "github.com/qin8948050/compute-sentry/operator/api/v1"
)

func TestPodMutator_Handle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = configv1.AddToScheme(scheme)

	decoder := admission.NewDecoder(scheme)

	// Create a fake client with some policies
	policy := &configv1.ComputeSentryPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "strict-policy",
		},
		Spec: configv1.ComputeSentryPolicySpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "strict"},
			},
			SpyConfig: configv1.SpyConfig{Enabled: true},
			Thresholds: configv1.Thresholds{
				MaxNCCLLatencyUs:    100,
				MinP2PBandwidthGbps: 50,
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(policy).Build()
	mutator := &PodMutator{Client: client, Decoder: decoder}

	tests := []struct {
		name        string
		pod         corev1.Pod
		expected    bool // true if injection expected
		hasPolicy   bool // true if a policy should match
		checkConfig bool
	}{
		{
			name: "Should inject basic sidecar without policy config when only manual annotation is present",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-pod",
					Annotations: map[string]string{"compute-sentry.aiguard.io/inject": "true"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "worker"}},
				},
			},
			expected:    true,
			hasPolicy:   false,
			checkConfig: false,
		},
		{
			name: "Should match policy and inject specific governance config",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "strict-pod",
					Labels: map[string]string{"app": "strict"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "worker"}},
				},
			},
			expected:    true,
			hasPolicy:   true,
			checkConfig: true,
		},
		{
			name: "Should not inject anything without annotation or policy",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "worker"}},
				},
			},
			expected:    false,
			hasPolicy:   false,
			checkConfig: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, _ := json.Marshal(tt.pod)
			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{Raw: raw},
				},
			}

			resp := mutator.Handle(context.Background(), req)

			if tt.expected {
				assert.True(t, resp.Allowed)
				assert.NotEmpty(t, resp.Patches, "Should have patches for injection")

				patchStr, _ := json.Marshal(resp.Patches)
				assert.Contains(t, string(patchStr), "LD_PRELOAD")

				if tt.hasPolicy {
					// Verify policy-specific config is present
					assert.Contains(t, string(patchStr), "governance-config")
					if tt.checkConfig {
						assert.Contains(t, string(patchStr), "100") // Latency
						assert.Contains(t, string(patchStr), "50")  // P2P Bandwidth
						assert.Contains(t, string(patchStr), "PRECHECK_MIN_P2P_GBPS")
					}
				} else {
					// Verify policy-specific config is NOT present
					assert.NotContains(t, string(patchStr), "governance-config")
					assert.NotContains(t, string(patchStr), "PRECHECK_MIN_P2P_GBPS")
				}

			} else {
				assert.True(t, resp.Allowed)
				assert.Empty(t, resp.Patches, "Should not have patches")
			}
		})
	}
}
