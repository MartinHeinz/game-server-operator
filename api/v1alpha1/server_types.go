/*
Copyright 2021 Martin Heinz.
*/

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ServerSpec defines the desired state of Server
type ServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make manifests" to regenerate code after modifying this file

	GameName GameName `json:"gameName"`

	// +listType=atomic
	// +optional
	Ports []corev1.ServicePort `json:"port,omitempty"`

	// +optional
	Config Config `json:"config,omitempty"`

	Storage *ServerStorage `json:"storage,omitempty"`

	// +optional
	ResourceRequirements *corev1.ResourceRequirements `json:"resources,omitempty"`
}

type Config struct {
	// +listType=atomic
	From []corev1.EnvFromSource `json:"from,omitempty"`
	// +optional
	// +kubebuilder:default:=Env
	MountAs MountType `json:"mountAs,omitempty"`
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

// +kubebuilder:validation:Enum=File;Env
type MountType string

const (
	File MountType = "File"
	Env  MountType = "Env"
)

// +kubebuilder:validation:Enum=CSGO;Factorio;Rust;Minecraft
type GameName string

const (
	CSGO      GameName = "CSGO"
	Factorio  GameName = "Factorio"
	Rust      GameName = "Rust"
	Minecraft GameName = "Minecraft"
)

// ServerStorage ...
// +k8s:openapi-gen=false
type ServerStorage struct {
	// +kubebuilder:validation:Pattern=^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$
	Size string `json:"size,omitempty"`
	// +optional
	//Name string `json:"name"`
}

// +kubebuilder:validation:Enum=Bound;Pending
type StorageStatus string

const (
	Bound   StorageStatus = "Bound"
	Pending StorageStatus = "Pending"
)

// +kubebuilder:validation:Enum=Active;Inactive
type Status string

const (
	Active   Status = "Active"
	Inactive Status = "Inactive"
)

// ServerStatus defines the observed state of Server
type ServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make manifests" to regenerate code after modifying this file

	// Describes whether there are running pods for this server
	// +optional
	Status Status `json:"status,omitempty"`
	// Describes whether storage for server is ready
	// +optional
	Storage StorageStatus `json:"storage,omitempty"`
	// List of available server NodePorts
	// +optional
	Ports []int32 `json:"ports,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Server is the Schema for the servers API
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=servers,scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.status",priority=0,description="Status of deployment"
// +kubebuilder:printcolumn:name="Storage",type="string",JSONPath=".status.storage",priority=0,description="Status of the reconcile condition"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",priority=0,description="Age of the resource"
// +kubebuilder:printcolumn:name="Ports",type="string",JSONPath=".status.ports",priority=10,description="List of available server NodePorts"
type Server struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServerSpec   `json:"spec,omitempty"`
	Status ServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ServerList contains a list of Server
type ServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Server `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Server{}, &ServerList{})
}

type GameSetting struct {
	Deployment            appsv1.Deployment
	Service               corev1.Service
	PersistentVolumeClaim corev1.PersistentVolumeClaim
}

var (
	Games = map[GameName]GameSetting{
		CSGO: {Deployment: appsv1.Deployment{
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
		Factorio: {Deployment: appsv1.Deployment{
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
				Strategy: appsv1.DeploymentStrategy{
					Type: appsv1.RecreateDeploymentStrategyType, // Pod keep crashing with rolling update if one instance already exists
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
		Rust: {Deployment: appsv1.Deployment{
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
		Minecraft: {Deployment: appsv1.Deployment{
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
