/*
Copyright 2021 Martin Heinz.
*/

package controllers

import (
	"context"
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

// +kubebuilder:rbac:groups=gameserver.martinheinz.dev,resources=servers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gameserver.martinheinz.dev,resources=servers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gameserver.martinheinz.dev,resources=servers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments;services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *ServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("server", req.NamespacedName)

	server := &gameserverv1alpha1.Server{}
	if err := r.Get(ctx, req.NamespacedName, server); err != nil {
		log.Error(err, "unable to fetch Server")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	gameName := server.Spec.GameName

	var gameSettings GameSetting

	for i, game := range Games {
		if game.Name == gameName {
			gameSettings = Games[i]
			break
		}
	}

	foundDep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, foundDep); err != nil && errors.IsNotFound(err) {
		// Define a new Deployment
		dep := r.deploymentForServer(server, &gameSettings)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	foundSvc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, foundSvc); err != nil && errors.IsNotFound(err) {
		// Define a new Service
		svc := r.serviceForServer(server, &gameSettings)
		log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.Create(ctx, svc)
		if err != nil {
			log.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
		// Service created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Service")
		return ctrl.Result{}, err
	}

	foundPvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, foundPvc); err != nil && errors.IsNotFound(err) {
		// Define a new PersistentVolumeClaim
		pvc := r.persistentVolumeClaimForServer(server, &gameSettings)
		log.Info("Creating a new PersistentVolumeClaim", "PersistentVolumeClaim.Namespace", pvc.Namespace, "PersistentVolumeClaim.Name", pvc.Name)
		err = r.Create(ctx, pvc)
		if err != nil {
			log.Error(err, "Failed to create new PersistentVolumeClaim", "PersistentVolumeClaim.Namespace", pvc.Namespace, "PersistentVolumeClaim.Name", pvc.Name)
			return ctrl.Result{}, err
		}
		// PersistentVolumeClaim created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get PersistentVolumeClaim")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ServerReconciler) deploymentForServer(m *gameserverv1alpha1.Server, gs *GameSetting) *appsv1.Deployment {
	ls := labelsForServer(m.Name)

	gs.Deployment.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name,
		Namespace: m.Namespace,
	}
	gs.Deployment.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: ls,
	}
	gs.Deployment.Spec.Template.Labels = ls
	gs.Deployment.Spec.Template.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName = m.Name
	for i, res := range m.Spec.EnvFrom {
		gs.Deployment.Spec.Template.Spec.Containers[0].EnvFrom = append(gs.Deployment.Spec.Template.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{})
		if res.ConfigMapRef != nil {
			gs.Deployment.Spec.Template.Spec.Containers[0].EnvFrom[i].ConfigMapRef = res.ConfigMapRef
		} else if res.SecretRef != nil {
			gs.Deployment.Spec.Template.Spec.Containers[0].EnvFrom[i].SecretRef = res.SecretRef
		}
	}
	ctrl.SetControllerReference(m, &gs.Deployment, r.Scheme)
	return &gs.Deployment
}

func (r *ServerReconciler) serviceForServer(m *gameserverv1alpha1.Server, gs *GameSetting) *corev1.Service {
	ls := labelsForServer(m.Name)

	gs.Service.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name,
		Namespace: m.Namespace,
	}
	gs.Service.Spec.Selector = ls

	ctrl.SetControllerReference(m, &gs.Service, r.Scheme)
	return &gs.Service
}

func (r *ServerReconciler) persistentVolumeClaimForServer(m *gameserverv1alpha1.Server, gs *GameSetting) *corev1.PersistentVolumeClaim {
	ls := labelsForServer(m.Name)

	gs.PersistentVolumeClaim.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name,
		Namespace: m.Namespace,
	}
	gs.PersistentVolumeClaim.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: ls,
	}
	gs.PersistentVolumeClaim.Spec.VolumeName = m.Name
	gs.PersistentVolumeClaim.Spec.Resources.Requests = corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse(m.Spec.Storage.Size),
	}

	ctrl.SetControllerReference(m, &gs.PersistentVolumeClaim, r.Scheme)
	return &gs.PersistentVolumeClaim
}

type GameSetting struct {
	Name                  gameserverv1alpha1.GameName
	Deployment            appsv1.Deployment
	Service               corev1.Service
	PersistentVolumeClaim corev1.PersistentVolumeClaim
}

var (
	// TODO make this a map with game Name as a key
	Games = []GameSetting{{
		Name: gameserverv1alpha1.CSGO,
		Deployment: appsv1.Deployment{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
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
							Image: "kmallea/csgo:latest",
							Name:  "csgo",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 27015, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 27015, Protocol: corev1.ProtocolUDP},
								{ContainerPort: 27020, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 27020, Protocol: corev1.ProtocolUDP},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "csgo-data", MountPath: "/home/steam/csgo"},
							},
						}},
					},
				},
			},
		},
		Service: corev1.Service{
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{Name: "27015-tcp", Port: 27015, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""}, Protocol: corev1.ProtocolTCP},
					{Name: "27015-udp", Port: 27015, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""}, Protocol: corev1.ProtocolUDP},
					{Name: "27020-tcp", Port: 27020, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27020, StrVal: ""}, Protocol: corev1.ProtocolTCP},
					{Name: "27020-udp", Port: 27020, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27020, StrVal: ""}, Protocol: corev1.ProtocolUDP},
				},
			},
		},
		PersistentVolumeClaim: corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				VolumeName:  "", // This gets set to server name (m.Name)
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
		},
	}}
)

func labelsForServer(name string) map[string]string {
	return map[string]string{"server": name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gameserverv1alpha1.Server{}).
		Complete(r)
}
