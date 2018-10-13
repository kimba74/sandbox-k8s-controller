package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Sampler is a specification for a Sampler resource
type Sampler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SamplerSpec   `json:"spec"`
	Status SamplerStatus `json:"status"`
}

type SamplerSpec struct {
	Host   string `json:"host"`
	Port   *int32 `json:"port"`
	Driver string `json:"driver"`
}

type SamplerStatus struct {
	Connected bool `json:"connected"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SamplerList is a list of Sampler resources
type SamplerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Sampler `json:"items"`
}
