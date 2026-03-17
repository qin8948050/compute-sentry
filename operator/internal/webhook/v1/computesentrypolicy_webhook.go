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
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configv1 "github.com/qin8948050/compute-sentry/operator/api/v1"
)

// nolint:unused
// log is for logging in this package.
var computesentrypolicylog = logf.Log.WithName("computesentrypolicy-resource")

// SetupComputeSentryPolicyWebhookWithManager registers the webhook for ComputeSentryPolicy in the manager.
func SetupComputeSentryPolicyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &configv1.ComputeSentryPolicy{}).
		WithValidator(&ComputeSentryPolicyCustomValidator{}).
		WithDefaulter(&ComputeSentryPolicyCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-config-aiguard-io-v1-computesentrypolicy,mutating=true,failurePolicy=fail,sideEffects=None,groups=config.aiguard.io,resources=computesentrypolicies,verbs=create;update,versions=v1,name=mcomputesentrypolicy-v1.kb.io,admissionReviewVersions=v1

// ComputeSentryPolicyCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind ComputeSentryPolicy when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type ComputeSentryPolicyCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind ComputeSentryPolicy.
func (d *ComputeSentryPolicyCustomDefaulter) Default(_ context.Context, obj *configv1.ComputeSentryPolicy) error {
	computesentrypolicylog.Info("Defaulting for ComputeSentryPolicy", "name", obj.GetName())

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: If you want to customise the 'path', use the flags '--defaulting-path' or '--validation-path'.
// +kubebuilder:webhook:path=/validate-config-aiguard-io-v1-computesentrypolicy,mutating=false,failurePolicy=fail,sideEffects=None,groups=config.aiguard.io,resources=computesentrypolicies,verbs=create;update,versions=v1,name=vcomputesentrypolicy-v1.kb.io,admissionReviewVersions=v1

// ComputeSentryPolicyCustomValidator struct is responsible for validating the ComputeSentryPolicy resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ComputeSentryPolicyCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ComputeSentryPolicy.
func (v *ComputeSentryPolicyCustomValidator) ValidateCreate(_ context.Context, obj *configv1.ComputeSentryPolicy) (admission.Warnings, error) {
	computesentrypolicylog.Info("Validation for ComputeSentryPolicy upon creation", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ComputeSentryPolicy.
func (v *ComputeSentryPolicyCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj *configv1.ComputeSentryPolicy) (admission.Warnings, error) {
	computesentrypolicylog.Info("Validation for ComputeSentryPolicy upon update", "name", newObj.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ComputeSentryPolicy.
func (v *ComputeSentryPolicyCustomValidator) ValidateDelete(_ context.Context, obj *configv1.ComputeSentryPolicy) (admission.Warnings, error) {
	computesentrypolicylog.Info("Validation for ComputeSentryPolicy upon deletion", "name", obj.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
