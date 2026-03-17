package v1

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate-v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create;update,versions=v1,name=mpod.kf.io,admissionReviewVersions=v1

// PodMutator annotates Pods
type PodMutator struct {
	Client  client.Client
	Decoder admission.Decoder
}

// Handle handles admission requests
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := m.Decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Logic to decide if we should inject the Spy library
	if pod.Annotations["compute-sentry.aiguard.io/inject"] != "true" {
		return admission.Allowed("not a target pod")
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
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "compute-sentry-bin",
				MountPath: "/opt/compute-sentry/bin",
				ReadOnly:  true,
			},
		},
	})

	// Add Binary Volume (ConfigMap based)
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "compute-sentry-bin",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "compute-sentry-precheck",
				},
				Items: []corev1.KeyToPath{
					{
						Key:  "precheck.sh",
						Path: "precheck.sh",
					},
				},
				DefaultMode: func(i int32) *int32 { return &i }(0755),
			},
		},
	})

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
