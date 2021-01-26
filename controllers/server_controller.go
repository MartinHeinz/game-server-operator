/*
Copyright 2021 Martin Heinz.
*/

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gameserverv1alpha1 "github.com/MartinHeinz/game-server-operator/api/v1alpha1"
)

// ServerReconciler reconciles a Server object
type ServerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var (
	depSuffix = "-deployment"
	svcSuffix = "-service"
	pvcSuffix = "-persistentvolumeclaim"
)

// +kubebuilder:rbac:groups=gameserver.martinheinz.dev,resources=servers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gameserver.martinheinz.dev,resources=servers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gameserver.martinheinz.dev,resources=servers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *ServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("server", req.NamespacedName)

	server := &gameserverv1alpha1.Server{}
	if err := r.Get(ctx, req.NamespacedName, server); err != nil {
		log.Error(err, "unable to fetch Server")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var childDep appsv1.Deployment
	server.Status.Status = gameserverv1alpha1.Inactive
	if err := r.Get(ctx, types.NamespacedName{Name: server.Name + depSuffix, Namespace: server.Namespace}, &childDep); err != nil && errors.IsNotFound(err) {
		log.Info("Child Deployment not available for status update")
	} else if *childDep.Spec.Replicas > int32(0) {
		server.Status.Status = gameserverv1alpha1.Active
	}

	var childPvc corev1.PersistentVolumeClaim
	server.Status.Storage = gameserverv1alpha1.Pending
	if err := r.Get(ctx, types.NamespacedName{Name: server.Name + pvcSuffix, Namespace: server.Namespace}, &childPvc); err != nil && errors.IsNotFound(err) {
		log.Info("Child PersistentVolumeClaim not available for status update")
	} else if childPvc.Status.Phase == corev1.ClaimBound {
		server.Status.Storage = gameserverv1alpha1.Bound
	}

	var childService corev1.Service
	server.Status.Ports = []int32{}
	if err := r.Get(ctx, types.NamespacedName{Name: server.Name + svcSuffix, Namespace: server.Namespace}, &childService); err != nil && errors.IsNotFound(err) {
		log.Info("Child Service not available for status update")
	} else {
		for _, port := range childService.Spec.Ports {
			server.Status.Ports = append(server.Status.Ports, port.NodePort)
		}
	}

	if err := r.Status().Update(ctx, server); err != nil {
		log.Error(err, "unable to update Server status")
		return ctrl.Result{}, err
	}

	gameName := server.Spec.GameName

	var gameSettings gameserverv1alpha1.GameSetting

	for name, game := range gameserverv1alpha1.Games {
		if name == gameName {
			gameSettings = game
			break
		}
	}

	found := []client.Object{
		&appsv1.Deployment{},
		&corev1.Service{},
		&corev1.PersistentVolumeClaim{},
	}

	// --------------------------
	// Initialize

	for _, f := range found {
		t := reflect.TypeOf(f).String()
		suffix := "-" + strings.ToLower(strings.Split(reflect.TypeOf(f).String(), ".")[1])
		objectName := server.Name + suffix
		if err := r.Get(ctx, types.NamespacedName{Name: objectName, Namespace: server.Namespace}, f); err != nil && errors.IsNotFound(err) {
			// Define a new Object
			obj := client.Object(nil)

			switch f.(type) {
			default:
				log.Info("Invalid Kind")
			case *appsv1.Deployment:
				obj = r.deploymentForServer(server, &gameSettings)
			case *corev1.Service:
				obj = r.serviceForServer(server, &gameSettings)
			case *corev1.PersistentVolumeClaim:
				// TODO To preserve PVC after Server deletion - delete PVC OwnersReference - Use Mutating Admission Webhook - https://book.kubebuilder.io/reference/webhook-for-core-types.html
				obj = r.persistentVolumeClaimForServer(server, &gameSettings)
			}
			log.Info(fmt.Sprintf("Creating a new %s", t), fmt.Sprintf("%s.Namespace", t), obj.GetNamespace(), fmt.Sprintf("%s.Name", t), obj.GetName())
			err = r.Create(ctx, obj)
			if err != nil {
				log.Error(err, fmt.Sprintf("Failed to create new %s", t), fmt.Sprintf("%s.Namespace", t), obj.GetNamespace(), fmt.Sprintf("%s.Name", t), obj.GetName())
				return ctrl.Result{}, err
			}
			// Object created successfully - return and requeue
			return ctrl.Result{Requeue: true}, nil
		} else if err != nil {
			log.Error(err, fmt.Sprintf("Failed to get %s", t))
			return ctrl.Result{}, err
		}
	}
	// --------------------------

	currentGen := server.Generation
	if currentGen == 1 {
		return ctrl.Result{}, nil
	}

	// --------------------------
	// Update
	requeue := false
	for _, f := range found {
		t := reflect.TypeOf(f).String()
		suffix := "-" + strings.ToLower(strings.Split(reflect.TypeOf(f).String(), ".")[1])
		objectName := server.Name + suffix
		if err := r.Get(ctx, types.NamespacedName{Name: objectName, Namespace: server.Namespace}, f); err == nil {
			// Define a new Object
			obj := client.Object(nil)

			switch f.(type) {
			default:
				log.Info("Invalid Kind")
			case *appsv1.Deployment:
				obj, requeue = r.updateDeploymentForServer(server, f.(*appsv1.Deployment))
			case *corev1.Service:
				obj, requeue = r.updateServiceForServer(server, f.(*corev1.Service))
			case *corev1.PersistentVolumeClaim:
				continue // TODO
				//obj, requeue = r.updatePersistentVolumeClaimForServer(server, &gameSettings)
			}
			log.Info(fmt.Sprintf("Updating a %s", t), fmt.Sprintf("%s.Namespace", t), obj.GetNamespace(), fmt.Sprintf("%s.Name", t), obj.GetName())

			if err = r.Update(ctx, obj); err != nil {
				log.Error(err, fmt.Sprintf("Failed to update %s", t), fmt.Sprintf("%s.Namespace", t), obj.GetNamespace(), fmt.Sprintf("%s.Name", t), obj.GetName())
				return ctrl.Result{}, err
			}
			// Object updated successfully - return and requeue
		} else if err != nil {
			log.Error(err, fmt.Sprintf("Failed to get %s", t))
			return ctrl.Result{}, err
		}
	}

	if requeue {
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ServerReconciler) deploymentForServer(m *gameserverv1alpha1.Server, gs *gameserverv1alpha1.GameSetting) *appsv1.Deployment {
	ls := labelsForServer(m.Name)

	dep := &appsv1.Deployment{}
	gs.Deployment.DeepCopyInto(dep)

	dep.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name + depSuffix,
		Namespace: m.Namespace,
		Labels:    ls,
	}
	dep.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: ls,
	}
	dep.Spec.Template.Labels = ls
	dep.Spec.Template.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName = m.Name + pvcSuffix

	if m.Spec.Config.MountAs == gameserverv1alpha1.File {

		// Setup `volumes` block of spec
		volume := corev1.Volume{
			Name: m.Name + "-config",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: func(val int32) *int32 { return &val }(0777),
				},
			},
		}
		for _, res := range m.Spec.Config.From {
			projection := corev1.VolumeProjection{}
			if res.ConfigMapRef != nil {
				projection.ConfigMap = &corev1.ConfigMapProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: res.ConfigMapRef.Name},
				}
			} else if res.SecretRef != nil {
				projection.Secret = &corev1.SecretProjection{
					LocalObjectReference: corev1.LocalObjectReference{Name: res.SecretRef.Name},
				}
			}
			volume.VolumeSource.Projected.Sources = append(volume.VolumeSource.Projected.Sources, projection)

		}
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, volume)

		// Setup `volumeMounts` block of containers
		// If mounted game config is present, then replace it
		replaced := false
		for i, volumeMount := range dep.Spec.Template.Spec.Containers[0].VolumeMounts {
			if volumeMount.Name == m.Name+"-config" {
				dep.Spec.Template.Spec.Containers[0].VolumeMounts[i] = corev1.VolumeMount{
					Name:      m.Name + "-config", // Must be same as in `volume` var above
					ReadOnly:  false,
					MountPath: m.Spec.Config.MountPath,
				}
				replaced = true
			}
		}

		// If mounted game config is not present, append it
		if !replaced {
			dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(dep.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      m.Name + "-config", // Must be same as in `volume` var above
				ReadOnly:  false,
				MountPath: m.Spec.Config.MountPath,
			})
		}

	} else {
		dep.Spec.Template.Spec.Containers[0].EnvFrom = nil
		for i, res := range m.Spec.Config.From {
			dep.Spec.Template.Spec.Containers[0].EnvFrom = append(dep.Spec.Template.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{})
			if res.ConfigMapRef != nil {
				dep.Spec.Template.Spec.Containers[0].EnvFrom[i].ConfigMapRef = res.ConfigMapRef
			} else if res.SecretRef != nil {
				dep.Spec.Template.Spec.Containers[0].EnvFrom[i].SecretRef = res.SecretRef
			}
		}
	}

	if m.Spec.ResourceRequirements != nil {
		dep.Spec.Template.Spec.Containers[0].Resources = *m.Spec.ResourceRequirements
	}

	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

func (r *ServerReconciler) updateDeploymentForServer(m *gameserverv1alpha1.Server, dep *appsv1.Deployment) (*appsv1.Deployment, bool) {
	existingConfig := dep.Spec.Template.Spec.Containers[0].EnvFrom
	existingResources := dep.Spec.Template.Spec.Containers[0].Resources
	requeue := false

	// If ConfigMap/Secret were changed
	if !reflect.DeepEqual(m.Spec.Config, existingConfig) {
		requeue = true
		if m.Spec.Config.MountAs == gameserverv1alpha1.File {

			// Remove old projected volume
			for i, vol := range dep.Spec.Template.Spec.Volumes {
				if vol.Name == m.Name+"-config" {
					dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes[:i], dep.Spec.Template.Spec.Volumes[i+1:]...)
				}
			}

			// Prepare new projected volume
			volume := corev1.Volume{
				Name: m.Name + "-config",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: func(val int32) *int32 { return &val }(0777),
					},
				},
			}

			for _, res := range m.Spec.Config.From {
				projection := corev1.VolumeProjection{}
				if res.ConfigMapRef != nil {
					projection.ConfigMap = &corev1.ConfigMapProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: res.ConfigMapRef.Name},
					}
				} else if res.SecretRef != nil {
					projection.Secret = &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: res.SecretRef.Name},
					}
				}
				volume.VolumeSource.Projected.Sources = append(volume.VolumeSource.Projected.Sources, projection)
			}
			// Append new projected volume
			dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, volume)

			// Remove old volumeMount
			for i, vol := range dep.Spec.Template.Spec.Containers[0].VolumeMounts {
				if vol.Name == m.Name+"-config" {
					dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(
						dep.Spec.Template.Spec.Containers[0].VolumeMounts[:i],
						dep.Spec.Template.Spec.Containers[0].VolumeMounts[i+1:]...,
					)
				}
			}
			// Append to volumeMount block of container
			dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(dep.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
				Name:      m.Name + "-config", // Must be same as in `volume` var above
				ReadOnly:  true,
				MountPath: m.Spec.Config.MountPath,
			})

		} else {
			var newConfig []corev1.EnvFromSource
			for i, res := range m.Spec.Config.From {
				newConfig = append(newConfig, corev1.EnvFromSource{})
				if res.ConfigMapRef != nil {
					newConfig[i].ConfigMapRef = res.ConfigMapRef
				} else if res.SecretRef != nil {
					newConfig[i].SecretRef = res.SecretRef
				}
			}
			dep.Spec.Template.Spec.Containers[0].EnvFrom = newConfig
		}
	}

	// If ResourceRequirements were changed
	if !reflect.DeepEqual(m.Spec.ResourceRequirements, existingResources) {
		requeue = true
		dep.Spec.Template.Spec.Containers[0].Resources = *m.Spec.ResourceRequirements
	}

	return dep, requeue
}

func (r *ServerReconciler) serviceForServer(m *gameserverv1alpha1.Server, gs *gameserverv1alpha1.GameSetting) *corev1.Service {
	ls := labelsForServer(m.Name)

	svc := &corev1.Service{}
	gs.Service.DeepCopyInto(svc)
	svc.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name + svcSuffix,
		Namespace: m.Namespace,
		Labels:    ls,
	}
	svc.Spec.Selector = ls

	if m.Spec.Ports != nil {
		svc.Spec.Ports = m.Spec.Ports
	}

	ctrl.SetControllerReference(m, svc, r.Scheme)
	return svc
}

func (r *ServerReconciler) updateServiceForServer(m *gameserverv1alpha1.Server, svc *corev1.Service) (*corev1.Service, bool) {
	existingServicePorts := svc.Spec.Ports
	requeue := false

	for i, port := range existingServicePorts {
		if port.NodePort != m.Spec.Ports[i].NodePort {
			requeue = true
			svc.Spec.Ports[i].NodePort = m.Spec.Ports[i].NodePort
		}
	}

	return svc, requeue
}

func (r *ServerReconciler) persistentVolumeClaimForServer(m *gameserverv1alpha1.Server, gs *gameserverv1alpha1.GameSetting) *corev1.PersistentVolumeClaim {
	ls := labelsForServer(m.Name)

	pvc := &corev1.PersistentVolumeClaim{}
	gs.PersistentVolumeClaim.DeepCopyInto(pvc)

	pvc.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name + pvcSuffix,
		Namespace: m.Namespace,
		Labels:    ls,
	}

	pvc.Spec.Resources.Requests = corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse(m.Spec.Storage.Size),
	}

	ctrl.SetControllerReference(m, pvc, r.Scheme) // TODO if PVC is to be preserved, then do not set Server as owner
	return pvc
}

func labelsForServer(name string) map[string]string {
	return map[string]string{"server": name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gameserverv1alpha1.Server{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
