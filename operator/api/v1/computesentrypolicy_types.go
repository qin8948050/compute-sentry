/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ComputeSentryPolicySpec defines the desired state of ComputeSentryPolicy
type ComputeSentryPolicySpec struct {
	// Selector is a label selector to identify which Pods this policy applies to.
	Selector metav1.LabelSelector `json:"selector"`

	// SpyConfig defines how the Spy library should be injected.
	// +optional
	SpyConfig SpyConfig `json:"spyConfig,omitempty"`

	// Thresholds defines performance health thresholds.
	// +optional
	Thresholds Thresholds `json:"thresholds,omitempty"`
}

// SpyConfig defines the configuration for the Spy library injection.
type SpyConfig struct {
	// Enabled indicates if the Spy library should be injected.
	Enabled bool `json:"enabled"`

	// Image is the sidecar or init container image containing the spy library.
	// +optional
	Image string `json:"image,omitempty"`

	// Path is the path to the libcompute-sentry-spy.so inside the container.
	// +optional
	Path string `json:"path,omitempty"`
}

// Thresholds defines performance health thresholds.
type Thresholds struct {
	// MaxNCCLLatencyUs is the maximum allowed NCCL latency in microseconds.
	// +optional
	MaxNCCLLatencyUs int64 `json:"maxNcclLatencyUs,omitempty"`

	// MaxJitterUs is the maximum allowed jitter in microseconds.
	// +optional
	MaxJitterUs int64 `json:"maxJitterUs,omitempty"`
}

// ComputeSentryPolicyStatus defines the observed state of ComputeSentryPolicy.
type ComputeSentryPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the ComputeSentryPolicy resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ComputeSentryPolicy is the Schema for the computesentrypolicies API
type ComputeSentryPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of ComputeSentryPolicy
	// +required
	Spec ComputeSentryPolicySpec `json:"spec"`

	// status defines the observed state of ComputeSentryPolicy
	// +optional
	Status ComputeSentryPolicyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ComputeSentryPolicyList contains a list of ComputeSentryPolicy
type ComputeSentryPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ComputeSentryPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ComputeSentryPolicy{}, &ComputeSentryPolicyList{})
}
