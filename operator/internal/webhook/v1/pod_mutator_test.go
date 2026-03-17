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
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestPodMutator_Handle(t *testing.T) {
	decoder := admission.NewDecoder(runtime.NewScheme())
	mutator := &PodMutator{Decoder: decoder}

	tests := []struct {
		name     string
		pod      corev1.Pod
		expected bool // true if injection expected
	}{
		{
			name: "Should inject when annotation is present",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-pod",
					Annotations: map[string]string{"compute-sentry.aiguard.io/inject": "true"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "worker"}},
				},
			},
			expected: true,
		},
		{
			name: "Should not inject without annotation",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "worker"}},
				},
			},
			expected: false,
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
				
				// Verify LD_PRELOAD and Volumes are in patches
				patchStr, _ := json.Marshal(resp.Patches)
				assert.Contains(t, string(patchStr), "LD_PRELOAD")
				assert.Contains(t, string(patchStr), "compute-sentry-precheck")
				assert.Contains(t, string(patchStr), "compute-sentry-lib")
			} else {
				assert.True(t, resp.Allowed)
				assert.Empty(t, resp.Patches, "Should not have patches")
			}
		})
	}
}
