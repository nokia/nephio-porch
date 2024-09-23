/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MutationInjectorSpec defines the desired state of MutationInjector
type MutationInjectorSpec struct {
	// defines the set of target PackageVariants
	PackageVariantSelector *metav1.LabelSelector `json:"packageVariantSelector,omitempty"`

	// mutations to apply to the target PackageVariants
	Mutations []Mutation `json:"mutations,omitempty"`
}

// MutationInjectorStatus defines the observed state of MutationInjector
type MutationInjectorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MutationInjector is the Schema for the mutationinjectors API
type MutationInjector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MutationInjectorSpec   `json:"spec,omitempty"`
	Status MutationInjectorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MutationInjectorList contains a list of MutationInjector
type MutationInjectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MutationInjector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MutationInjector{}, &MutationInjectorList{})
}

var (
	MutationInjectorGVK = GroupVersion.WithKind(reflect.TypeOf(MutationInjector{}).Name())
)
