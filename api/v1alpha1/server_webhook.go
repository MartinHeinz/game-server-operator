/*
Copyright 2021 Martin Heinz.
*/

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
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

// +kubebuilder:webhook:path=/mutate-gameserver-martinheinz-dev-v1alpha1-server,mutating=true,failurePolicy=fail,sideEffects=None,groups=gameserver.martinheinz.dev,resources=servers,verbs=create;update,versions=v1alpha1,name=mserver.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &Server{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Server) Default() {
	serverlog.Info("default", "name", r.Name)

	// TODO Generate random default name for Server
}

// +kubebuilder:webhook:path=/validate-gameserver-martinheinz-dev-v1alpha1-server,mutating=false,failurePolicy=fail,sideEffects=None,groups=gameserver.martinheinz.dev,resources=servers,verbs=create;update,versions=v1alpha1,name=vserver.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &Server{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Server) ValidateCreate() error {
	serverlog.Info("validate create", "name", r.Name)

	// TODO Validation logic on object creation
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Server) ValidateUpdate(old runtime.Object) error {
	serverlog.Info("validate update", "name", r.Name)

	// TODO Validation logic on object update
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Server) ValidateDelete() error {
	serverlog.Info("validate delete", "name", r.Name)

	return nil
}
