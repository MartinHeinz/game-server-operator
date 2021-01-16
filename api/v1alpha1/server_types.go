/*
Copyright 2021 Martin Heinz.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ServerSpec defines the desired state of Server
type ServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make manifests" to regenerate code after modifying this file

	// If omitted, defaults to CRD name
	// +optional
	ServerName string   `json:"serverName,omitempty"`
	GameName   GameName `json:"gameName"`

	// +listType=atomic
	// +optional
	Ports []corev1.ServicePort `json:"port,omitempty"`

	// +listType=atomic
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	Storage *ServerStorage `json:"storage,omitempty"`

	// +optional
	ResourceRequirements *corev1.ResourceRequirements `json:"resources,omitempty"`
}

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
