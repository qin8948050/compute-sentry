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

	// EvalConfig defines health evaluation parameters for the Agent.
	// +optional
	EvalConfig EvalConfig `json:"evalConfig,omitempty"`

	// Actions defines remediation actions when health check fails.
	// +optional
	Actions Actions `json:"actions,omitempty"`
}

// SpyConfig defines the configuration for the Spy library injection.
type SpyConfig struct {
	// Enabled indicates if the Spy library should be injected.
	Enabled bool `json:"enabled"`
}

// Thresholds defines performance health thresholds.
type Thresholds struct {
	// MaxNCCLLatencyUs is the maximum allowed NCCL latency in microseconds.
	// +optional
	MaxNCCLLatencyUs int64 `json:"maxNcclLatencyUs,omitempty"`

	// MaxJitterUs is the maximum allowed jitter in microseconds.
	// +optional
	MaxJitterUs int64 `json:"maxJitterUs,omitempty"`

	// MinP2PBandwidthGbps is the minimum required P2P bandwidth in Gbps.
	// +optional
	MinP2PBandwidthGbps int64 `json:"minP2PBandwidthGbps,omitempty"`

	// MinHbmBandwidthGbps is the minimum required HBM bandwidth in Gbps.
	// +optional
	MinHbmBandwidthGbps int64 `json:"minHbmBandwidthGbps,omitempty"`
}

// GovernanceConfig is the serialized configuration injected into Pods.
// It combines Thresholds and EvalConfig for consumption by the Agent.
type GovernanceConfig struct {
	Thresholds Thresholds `json:"thresholds"`
	EvalConfig EvalConfig `json:"evalConfig"`
}

// EvalConfig defines health evaluation parameters for the Agent
type EvalConfig struct {
	// WindowSize is the time window (in seconds) for sliding window evaluation
	// +optional
	WindowSize int64 `json:"windowSize,omitempty"`

	// ErrorCountLimit is the number of violations within the window to trigger unhealthy
	// +optional
	ErrorCountLimit int64 `json:"errorCountLimit,omitempty"`
}

// Actions defines remediation actions when health check fails
type Actions struct {
	// EnableTaint enables tainting the node as NoSchedule when unhealthy
	// +optional
	EnableTaint bool `json:"enableTaint,omitempty"`

	// EnableEvict enables evicting the pod when unhealthy
	// +optional
	EnableEvict bool `json:"enableEvict,omitempty"`
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
