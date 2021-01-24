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
	"k8s.io/apimachinery/pkg/util/intstr"
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

	var gameSettings GameSetting

	for name, game := range Games {
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

func (r *ServerReconciler) deploymentForServer(m *gameserverv1alpha1.Server, gs *GameSetting) *appsv1.Deployment {
	ls := labelsForServer(m.Name)

	gs.Deployment.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name + depSuffix,
		Namespace: m.Namespace,
		Labels:    ls,
	}
	gs.Deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: ls,
	}
	gs.Deployment.Spec.Template.Labels = ls
	gs.Deployment.Spec.Template.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName = m.Name + pvcSuffix

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
		gs.Deployment.Spec.Template.Spec.Volumes = append(gs.Deployment.Spec.Template.Spec.Volumes, volume)

		// Setup `volumeMounts` block of containers
		gs.Deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(gs.Deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      m.Name + "-config", // Must be same as in `volume` var above
			ReadOnly:  false,
			MountPath: m.Spec.Config.MountPath,
		})
	} else {
		gs.Deployment.Spec.Template.Spec.Containers[0].EnvFrom = nil
		for i, res := range m.Spec.Config.From {
			gs.Deployment.Spec.Template.Spec.Containers[0].EnvFrom = append(gs.Deployment.Spec.Template.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{})
			if res.ConfigMapRef != nil {
				gs.Deployment.Spec.Template.Spec.Containers[0].EnvFrom[i].ConfigMapRef = res.ConfigMapRef
			} else if res.SecretRef != nil {
				gs.Deployment.Spec.Template.Spec.Containers[0].EnvFrom[i].SecretRef = res.SecretRef
			}
		}
	}

	if m.Spec.ResourceRequirements != nil {
		gs.Deployment.Spec.Template.Spec.Containers[0].Resources = *m.Spec.ResourceRequirements
	}

	ctrl.SetControllerReference(m, &gs.Deployment, r.Scheme)
	return &gs.Deployment
}

func (r *ServerReconciler) updateDeploymentForServer(m *gameserverv1alpha1.Server, dep *appsv1.Deployment) (*appsv1.Deployment, bool) {
	existingConfig := dep.Spec.Template.Spec.Containers[0].EnvFrom
	existingResources := dep.Spec.Template.Spec.Containers[0].Resources
	requeue := false

	// If ConfigMap/Secret were changed
	if !reflect.DeepEqual(m.Spec.Config, existingConfig) {
		requeue = true
		// TODO Test
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

func (r *ServerReconciler) serviceForServer(m *gameserverv1alpha1.Server, gs *GameSetting) *corev1.Service {
	ls := labelsForServer(m.Name)

	gs.Service.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name + svcSuffix,
		Namespace: m.Namespace,
		Labels:    ls,
	}
	gs.Service.Spec.Selector = ls

	if m.Spec.Ports != nil {
		gs.Service.Spec.Ports = m.Spec.Ports
	}

	ctrl.SetControllerReference(m, &gs.Service, r.Scheme)
	return &gs.Service
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

func (r *ServerReconciler) persistentVolumeClaimForServer(m *gameserverv1alpha1.Server, gs *GameSetting) *corev1.PersistentVolumeClaim {
	ls := labelsForServer(m.Name)

	gs.PersistentVolumeClaim.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name + pvcSuffix,
		Namespace: m.Namespace,
		Labels:    ls,
	}

	gs.PersistentVolumeClaim.Spec.Resources.Requests = corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse(m.Spec.Storage.Size),
	}

	ctrl.SetControllerReference(m, &gs.PersistentVolumeClaim, r.Scheme) // TODO if PVC is to be preserved, then do not set Server as owner
	return &gs.PersistentVolumeClaim
}

type GameSetting struct {
	Deployment            appsv1.Deployment
	Service               corev1.Service
	PersistentVolumeClaim corev1.PersistentVolumeClaim
}

var (
	Games = map[gameserverv1alpha1.GameName]GameSetting{
		gameserverv1alpha1.CSGO: {Deployment: appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: func(val int32) *int32 { return &val }(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{{
							Name: "csgo-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "", // This gets set to server name (m.Name)
								},
							},
						}},
						Containers: []corev1.Container{{
							Name:  "csgo",
							Image: "kmallea/csgo:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 27015, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 27015, Protocol: corev1.ProtocolUDP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "csgo-data", MountPath: "/home/steam/csgo"},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						}},
					},
				},
			},
		},
			Service: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csgo",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "27015-tcp", Port: 27015, NodePort: 30015, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""}, Protocol: corev1.ProtocolTCP},
						{Name: "27015-udp", Port: 27015, NodePort: 30015, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""}, Protocol: corev1.ProtocolUDP},
					},
					Type: corev1.ServiceTypeNodePort,
				},
			},
			PersistentVolumeClaim: corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csgo",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			},
		},
		gameserverv1alpha1.Factorio: {Deployment: appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: func(val int32) *int32 { return &val }(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{{
							Name: "factorio-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "", // This gets set to server name (m.Name)
								},
							},
						}},
						Containers: []corev1.Container{{
							Name:  "factorio",
							Image: "factoriotools/factorio:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 27015, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 34197, Protocol: corev1.ProtocolUDP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "factorio-data", MountPath: "/factorio"},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						}},
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser:  func(val int64) *int64 { return &val }(845),
							RunAsGroup: func(val int64) *int64 { return &val }(845),
							FSGroup:    func(val int64) *int64 { return &val }(845),
						},
					},
				},
			},
		},
			Service: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "factorio",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "27015-tcp", Port: 27015, NodePort: 30015, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""}, Protocol: corev1.ProtocolTCP},
						{Name: "34197-udp", Port: 34197, NodePort: 32197, TargetPort: intstr.IntOrString{Type: 0, IntVal: 34197, StrVal: ""}, Protocol: corev1.ProtocolUDP},
					},
					Type: corev1.ServiceTypeNodePort,
				},
			},
			PersistentVolumeClaim: corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "factorio",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			},
		},
		gameserverv1alpha1.Rust: {Deployment: appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: func(val int32) *int32 { return &val }(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{{
							Name: "rust-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "", // This gets set to server name (m.Name)
								},
							},
						}},
						Containers: []corev1.Container{{
							Name:  "rust",
							Image: "didstopia/rust-server:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 30015, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 30015, Protocol: corev1.ProtocolUDP},
								{ContainerPort: 30016, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 8080, Protocol: corev1.ProtocolUDP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "rust-data", MountPath: "/steamcmd/rust"},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						}},
					},
				},
			},
		},
			Service: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rust",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "30015-tcp", Port: 30015, NodePort: 30015, TargetPort: intstr.IntOrString{Type: 0, IntVal: 30015, StrVal: ""}, Protocol: corev1.ProtocolTCP},
						{Name: "30015-udp", Port: 30015, NodePort: 30015, TargetPort: intstr.IntOrString{Type: 0, IntVal: 30015, StrVal: ""}, Protocol: corev1.ProtocolUDP},
						{Name: "30016-tcp", Port: 30016, NodePort: 30016, TargetPort: intstr.IntOrString{Type: 0, IntVal: 30016, StrVal: ""}, Protocol: corev1.ProtocolTCP},
						{Name: "8080-tcp", Port: 8080, NodePort: 30080, TargetPort: intstr.IntOrString{Type: 0, IntVal: 8080, StrVal: ""}, Protocol: corev1.ProtocolTCP},
						{Name: "8080-udp", Port: 8080, NodePort: 30080, TargetPort: intstr.IntOrString{Type: 0, IntVal: 8080, StrVal: ""}, Protocol: corev1.ProtocolUDP},
					},
					Type: corev1.ServiceTypeNodePort,
				},
			},
			PersistentVolumeClaim: corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rust",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			},
		},
		gameserverv1alpha1.Minecraft: {Deployment: appsv1.Deployment{
			Spec: appsv1.DeploymentSpec{
				Replicas: func(val int32) *int32 { return &val }(1),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{{
							Name: "minecraft-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "", // This gets set to server name (m.Name)
								},
							},
						}},
						Containers: []corev1.Container{{
							Name:  "minecraft",
							Image: "itzg/minecraft-server:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 25565, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 2375, Protocol: corev1.ProtocolTCP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "minecraft-data", MountPath: "/data"},
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						}},
					},
				},
			},
		},
			Service: corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "minecraft",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{Name: "25565-tcp", Port: 25565, NodePort: 30565, TargetPort: intstr.IntOrString{Type: 0, IntVal: 25565, StrVal: ""}, Protocol: corev1.ProtocolTCP},
						{Name: "25575-tcp", Port: 25575, NodePort: 30575, TargetPort: intstr.IntOrString{Type: 0, IntVal: 25575, StrVal: ""}, Protocol: corev1.ProtocolTCP},
					},
					Type: corev1.ServiceTypeNodePort,
				},
			},
			PersistentVolumeClaim: corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "minecraft",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				},
			},
		},
	}
)

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
