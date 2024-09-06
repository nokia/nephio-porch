// Copyright 2024 The kpt and Nephio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	kptfile "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PackageVariant represents an upstream and downstream porch package pair.
// The upstream package should already exist. The PackageVariant controller is
// responsible for creating the downstream package revisions based on the spec.
type PackageVariant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PackageVariantSpec   `json:"spec,omitempty"`
	Status PackageVariantStatus `json:"status,omitempty"`
}

func (o *PackageVariant) GetSpec() *PackageVariantSpec {
	if o == nil {
		return nil
	}
	return &o.Spec
}

type AdoptionPolicy string
type DeletionPolicy string
type ApprovalPolicy string

const (
	AdoptionPolicyAdoptExisting AdoptionPolicy = "adoptExisting"
	AdoptionPolicyAdoptNone     AdoptionPolicy = "adoptNone"

	DeletionPolicyDelete DeletionPolicy = "delete"
	DeletionPolicyOrphan DeletionPolicy = "orphan"

	ApprovalPolicyNever   ApprovalPolicy = "never"
	ApprovalPolicyAlways  ApprovalPolicy = "always"
	ApprovalPolicyInitial ApprovalPolicy = "initial"

	Finalizer = "config.porch.kpt.dev/packagevariants"
)

// PackageVariantSpec defines the desired state of PackageVariant
type PackageVariantSpec struct {
	Upstream   *Upstream   `json:"upstream,omitempty"`
	Downstream *Downstream `json:"downstream,omitempty"`

	AdoptionPolicy AdoptionPolicy `json:"adoptionPolicy,omitempty"`
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
	//+default="never"
	ApprovalPolicy ApprovalPolicy `json:"approvalPolicy,omitempty"`
	// Readiness gates added to downstream packages
	ReadinessGates []kptfile.ReadinessGate `json:"readinessGates,omitempty"`
	// Labels added to downstream packages
	Labels map[string]string `json:"labels,omitempty"`
	// Annotations added to downstream packages
	Annotations map[string]string `json:"annotations,omitempty"`

	// List of mutations to apply to the downstream package after cloning.
	//+listType=map
	//+listMapKey=manager
	//+listMapKey=name
	Mutations []Mutation `json:"mutations,omitempty"`
}

type MutationType string

const (
	MutationTypeInjectPackage   MutationType = "InjectPackage"
	MutationTypeInjectObject    MutationType = "InjectObject"
	MutationTypePrependPipeline MutationType = "PrependPipeline"
	MutationTypeAppendPipeline  MutationType = "AppendPipeline"
	MutationTypeEvalKrmFunction MutationType = "EvalKrmFunction"
)

// A mutation that should be applied to the downstream package
type Mutation struct {
	// Name and Manager fields together must uniquely identify a mutation in the scope of a PackageVariant object.
	//+required
	Name string `json:"name,omitempty"`
	// Name of the controller that manages the mutation (may be empty)
	//+default=""
	Manager string `json:"manager,omitempty"`
	// Type selector enum for the union type
	Type MutationType `json:"type,omitempty"`
	// Data for "InjectPackage" type
	InjectPackage *InjectPackage `json:"injectPackage,omitempty"`
	// Data for "AppendPipeline" type
	InjectObject *InjectObject `json:"injectObject,omitempty"`
	// Data for "PrependPipeline" type
	PrependPipeline *kptfile.Pipeline `json:"prependPipeline,omitempty"`
	// Data for "AppendPipeline" type
	AppendPipeline *kptfile.Pipeline `json:"appendPipeline,omitempty"`
}

type InjectPackage struct {
	// The package to be inserted into the downstream package.
	Package Upstream `json:"package,omitempty"`
	// The path within the downstream package to insert the package.
	Subdir string `json:"subdir,omitempty"`
}

type Upstream struct {
	Repo     string `json:"repo,omitempty"`
	Package  string `json:"package,omitempty"`
	Revision string `json:"revision,omitempty"`
}

type Downstream struct {
	Repo    string `json:"repo,omitempty"`
	Package string `json:"package,omitempty"`
}

type ObjectRef struct {
	Group     *string `json:"group,omitempty"`
	Version   *string `json:"version,omitempty"`
	Kind      *string `json:"kind,omitempty"`
	Namespace *string `json:"namespace"`
	Name      string  `json:"name"`
}

// InjectObject specifies how to select in-cluster objects for
// resolving injection points.
type InjectObject struct {
	Source      ObjectRef `json:"source"`
	Destination ObjectRef `json:"destination"`
}

// PackageVariantStatus defines the observed state of PackageVariant
type PackageVariantStatus struct {
	// Conditions describes the reconciliation state of the object.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// DownstreamTargets contains the downstream targets that the PackageVariant
	// either created or adopted.
	DownstreamTargets []DownstreamTarget `json:"downstreamTargets,omitempty"`
}

type DownstreamTarget struct {
	Name         string                `json:"name,omitempty"`
	RenderStatus porchapi.RenderStatus `json:"renderStatus,omitempty"`
}

//+kubebuilder:object:root=true

// PackageVariantList contains a list of PackageVariant
type PackageVariantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PackageVariant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PackageVariant{}, &PackageVariantList{})
}
