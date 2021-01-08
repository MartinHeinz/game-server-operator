/*
Copyright 2021 Martin Heinz.
*/

package controllers

import (
	"context"
	"fmt"
	"k8s.io/api/networking/v1beta1"
	"reflect"

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
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

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
		&v1beta1.Ingress{},
	}

	for _, f := range found {
		t := reflect.TypeOf(f).String()
		if err := r.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, f); err != nil && errors.IsNotFound(err) {
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
				obj = r.persistentVolumeClaimForServer(server, &gameSettings) // TODO Always use existing PVC if there is one
			case *v1beta1.Ingress:
				obj = r.ingressForServer(server, &gameSettings)
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
		corev1.ResourceStorage: resource.MustParse(m.Spec.Storage.Size), // TODO Does not propagate correctly?
	}

	ctrl.SetControllerReference(m, &gs.PersistentVolumeClaim, r.Scheme)
	return &gs.PersistentVolumeClaim
}

func (r *ServerReconciler) ingressForServer(m *gameserverv1alpha1.Server, gs *GameSetting) *v1beta1.Ingress {
	gs.Ingress.ObjectMeta = metav1.ObjectMeta{
		Name:      m.Name,
		Namespace: m.Namespace,
	}
	gs.Ingress.Spec.Rules[0].Host = m.Spec.Route

	ctrl.SetControllerReference(m, &gs.Ingress, r.Scheme)
	return &gs.Ingress
}

type GameSetting struct {
	Deployment            appsv1.Deployment
	Service               corev1.Service
	PersistentVolumeClaim corev1.PersistentVolumeClaim
	Ingress               v1beta1.Ingress
}

var (
	Games = map[gameserverv1alpha1.GameName]GameSetting{
		gameserverv1alpha1.CSGO: {Deployment: appsv1.Deployment{
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
							Name:  "csgo",
							Image: "kmallea/csgo:latest",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 27015, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 27015, Protocol: corev1.ProtocolUDP},
								{ContainerPort: 27020, Protocol: corev1.ProtocolTCP},
								{ContainerPort: 27020, Protocol: corev1.ProtocolUDP},
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
			Ingress: v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: "csgo",
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "", // Populated dynamically from "Route" attribute
							IngressRuleValue: v1beta1.IngressRuleValue{HTTP: &v1beta1.HTTPIngressRuleValue{Paths: []v1beta1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: func(val v1beta1.PathType) *v1beta1.PathType { return &val }(v1beta1.PathTypePrefix),
									Backend: v1beta1.IngressBackend{
										ServiceName: "csgo",
										ServicePort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""},
									},
								},
							}}},
						},
					},
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
