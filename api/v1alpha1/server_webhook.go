/*
Copyright 2021 Martin Heinz.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var serverlog = logf.Log.WithName("server-resource")

func (r *Server) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-gameserver-martinheinz-dev-v1alpha1-server,mutating=true,failurePolicy=Ignore,sideEffects=None,groups=gameserver.martinheinz.dev,resources=servers,verbs=create;update,versions=v1alpha1,name=mserver.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &Server{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Server) Default() {
	serverlog.Info("default", "name", r.Name)

	// Note: If not assignment is made here, then "webhook returned response.patchType but not response.patch" is thrown
	//       Therefore failurePolicy=Ignore, otherwise no resource gets admitted
	if r.Spec.ResourceRequirements == nil {
		r.Spec.ResourceRequirements = &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("128m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
	}

	if r.Spec.Ports == nil {
		r.Spec.Ports = Games[r.Spec.GameName].Service.Spec.Ports
	}
}

// +kubebuilder:webhook:path=/validate-gameserver-martinheinz-dev-v1alpha1-server,mutating=false,failurePolicy=fail,sideEffects=None,groups=gameserver.martinheinz.dev,resources=servers,verbs=create;update,versions=v1alpha1,name=vserver.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &Server{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Server) ValidateCreate() error {
	serverlog.Info("validate create", "name", r.Name)
	var allErrs field.ErrorList

	// Validation logic on object creation
	if len(r.Spec.Config.From) == 0 {
		serverlog.Info("validate create - Environment configuration missing", "name", r.Name)
		allErrs = append(allErrs, field.Required(field.NewPath("spec").Child("envFrom"), "Environment configuration is required"))
	} else if r.Spec.Config.MountAs == File && r.Spec.Config.MountPath == "" {
		serverlog.Info("validate create - MountPath is required when MountAs: File is specified", "name", r.Name)
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("envFrom").Child("MountPath"), "MountPath is required when MountAs: File is specified"))
	} else {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "gameserver.martinheinz.dev", Kind: "Server"},
		r.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Server) ValidateUpdate(old runtime.Object) error {
	serverlog.Info("validate update", "name", r.Name)
	return r.enforceImmutability(old)
}

func (r *Server) enforceImmutability(old runtime.Object) error {
	var allErrs field.ErrorList
	errorMessage := "Field is immutable"
	oldServer := (old).(*Server)

	portsPath := field.NewPath("spec").Child("ports")
	if len(oldServer.Spec.Ports) != len(r.Spec.Ports) {
		allErrs = append(allErrs, field.Forbidden(portsPath, errorMessage))
	} else if len(r.Spec.Ports) > 0 {
		for i, ports := range r.Spec.Ports {
			if ports.Port != oldServer.Spec.Ports[i].Port {
				allErrs = append(allErrs, field.Forbidden(portsPath.Child("port"), errorMessage))
			}
			if ports.TargetPort != oldServer.Spec.Ports[i].TargetPort {
				allErrs = append(allErrs, field.Forbidden(portsPath.Child("targetPort"), errorMessage))
			}
		}
	}

	if !reflect.DeepEqual(oldServer.Spec.Storage, r.Spec.Storage) {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("storage"), errorMessage))
	}

	if oldServer.Spec.GameName != r.Spec.GameName {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec").Child("gameName"), errorMessage))
	}

	if !reflect.DeepEqual(oldServer.Spec.Config.MountAs, r.Spec.Config.MountAs) {
		allErrs = append(allErrs, field.Forbidden(field.
			NewPath("spec").
			Child("EnvFrom").
			Child("MountAs"), errorMessage))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "gameserver.martinheinz.dev", Kind: "Server"},
		r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Server) ValidateDelete() error {
	serverlog.Info("validate delete", "name", r.Name)

	return nil
}
