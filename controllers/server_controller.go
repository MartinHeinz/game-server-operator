/*
Copyright 2021 Martin Heinz.
*/

package controllers

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
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

	foundDep := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, foundDep); err != nil && errors.IsNotFound(err) {
		// Define a new Deployment
		dep := r.deploymentForServer(server)
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
		svc := r.serviceForServer(server)
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
		pvc := r.persistentVolumeClaimForServer(server)
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

func (r *ServerReconciler) deploymentForServer(m *gameserverv1alpha1.Server) *appsv1.Deployment {
	ls := labelsForServer(m.Name)
	replicas := new(int32)
	*replicas = 1

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: *r.podSpecForServerDeployment(m),
			},
		},
	}
	ctrl.SetControllerReference(m, dep, r.Scheme)
	return dep
}

func (r *ServerReconciler) podSpecForServerDeployment(m *gameserverv1alpha1.Server) *corev1.PodSpec {
	spec := &corev1.PodSpec{}
	switch m.Spec.GameName {
	case gameserverv1alpha1.CSGO:
		spec = &corev1.PodSpec{Containers: []corev1.Container{csgo.container}, Volumes: []corev1.Volume{csgo.volume}}
		spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName = m.Name
	default:
		fmt.Printf("Game not found: %s.\n", m.Spec.GameName) // TODO Make this into error and propagate it up to be logged.
	}
	return spec
}

func (r *ServerReconciler) serviceForServer(m *gameserverv1alpha1.Server) *corev1.Service {
	ls := labelsForServer(m.Name)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports:    *r.portSpecForServerService(m),
		},
	}
	ctrl.SetControllerReference(m, svc, r.Scheme)
	return svc
}

func (r *ServerReconciler) portSpecForServerService(m *gameserverv1alpha1.Server) *[]corev1.ServicePort {
	ports := &[]corev1.ServicePort{}
	switch m.Spec.GameName {
	case gameserverv1alpha1.CSGO:
		ports = &csgo.servicePorts
	default:
		fmt.Printf("Game not found: %s.\n", m.Spec.GameName) // TODO Make this into error and propagate it up to be logged.
	}
	return ports
}

func (r *ServerReconciler) persistentVolumeClaimForServer(m *gameserverv1alpha1.Server) *corev1.PersistentVolumeClaim {
	ls := labelsForServer(m.Name)

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			VolumeName:  m.Name,
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1G"),
				},
			},
		},
	}
	ctrl.SetControllerReference(m, pvc, r.Scheme)
	return pvc
}

type GameSetting struct {
	name         gameserverv1alpha1.GameName
	container    corev1.Container
	servicePorts []corev1.ServicePort
	volume       corev1.Volume
}

var (
	csgo = GameSetting{
		name: gameserverv1alpha1.CSGO,
		container: corev1.Container{
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
		},
		volume: corev1.Volume{
			Name: "csgo-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "", // This gets set to server name (m.Name)
				},
			},
		},
		servicePorts: []corev1.ServicePort{
			{Name: "27015-tcp", Port: 27015, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""}, Protocol: corev1.ProtocolTCP},
			{Name: "27015-udp", Port: 27015, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27015, StrVal: ""}, Protocol: corev1.ProtocolUDP},
			{Name: "27020-tcp", Port: 27020, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27020, StrVal: ""}, Protocol: corev1.ProtocolTCP},
			{Name: "27020-udp", Port: 27020, TargetPort: intstr.IntOrString{Type: 0, IntVal: 27020, StrVal: ""}, Protocol: corev1.ProtocolUDP},
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
		Complete(r)
}
