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

const (
	ConditionTypeValid = "Valid" // whether or not the packagevariant object is making progress or not
	ConditionTypeReady = "Ready" // whether or not the reconciliation succeeded
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

	ApprovalPolicyNever               ApprovalPolicy = "never"
	ApprovalPolicyAlways              ApprovalPolicy = "always"
	ApprovalPolicyInitial             ApprovalPolicy = "initial"
	ApprovalPolicyWithManualReadiness ApprovalPolicy = "manualReadiness"

	Finalizer = "config.porch.kpt.dev/packagevariants"
)

// PackageVariantSpec defines the desired state of PackageVariant
type PackageVariantSpec struct {
	//+required
	Upstream PackageRevisionRef `json:"upstream,omitempty"`
	//+required
	Downstream PackageRef `json:"downstream,omitempty"`

	//+default="adoptNone"
	//+kubebuilder:validation:Enum=adoptExisting;adoptNone
	AdoptionPolicy AdoptionPolicy `json:"adoptionPolicy,omitempty"`
	//+default="delete"
	//+kubebuilder:validation:Enum=delete;orphan
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
	//+default="never"
	//+kubebuilder:validation:Enum=never;always;initial;manualReadiness
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

	// The ServiceAccount to use when trying to access resources in other namespaces
	//+default="default"
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

type MutationType string

const (
	MutationTypeInjectPackageRevision       MutationType = "InjectPackageRevision"
	MutationTypeInjectLatestPackageRevision MutationType = "InjectLatestPackageRevision"
	MutationTypeInjectObject                MutationType = "InjectObject"
	MutationTypePrependPipeline             MutationType = "PrependPipeline"
	MutationTypeAppendPipeline              MutationType = "AppendPipeline"
)

// A mutation that should be applied to the downstream package
// +kubebuilder:validation:XValidation:message="injectPackageRevision field is mandatory if type == InjectPackageRevision",rule="self.type != 'InjectPackageRevision' || has(self.injectPackageRevision)"
// +kubebuilder:validation:XValidation:message="injectLatestPackageRevision field is mandatory if type == InjectLatestPackageRevision",rule="self.type != 'InjectLatestPackageRevision' || has(self.injectLatestPackageRevision)"
// +kubebuilder:validation:XValidation:message="injectObject field is mandatory if type == InjectObject",rule="self.type != 'InjectObject' || has(self.injectObject)"
// +kubebuilder:validation:XValidation:message="prependPipeline field is mandatory if type == PrependPipeline",rule="self.type != 'PrependPipeline' || has(self.prependPipeline)"
// +kubebuilder:validation:XValidation:message="appendPipeline field is mandatory if type == AppendPipeline",rule="self.type != 'AppendPipeline' || has(self.appendPipeline)"
type Mutation struct {
	// Name and Manager fields together must uniquely identify a mutation in the scope of a PackageVariant object.
	//+required
	Name string `json:"name,omitempty"`
	// Name of the controller that manages the mutation (may be empty)
	//+default=""
	Manager string `json:"manager,omitempty"`
	// Type selector enum for the union type
	//+required
	//+kubebuilder:validation:Enum=InjectPackageRevision;InjectLatestPackageRevision;InjectObject;PrependPipeline;AppendPipeline
	Type MutationType `json:"type,omitempty"`
	// Data for "InjectPackageRevision" type
	InjectPackageRevision *InjectPackageRevision `json:"injectPackageRevision,omitempty"`
	// Data for "InjectLatestPackageRevision" type
	InjectLatestPackageRevision *InjectLatestPackageRevision `json:"injectLatestPackageRevision,omitempty"`
	// Data for "AppendPipeline" type
	InjectObject *InjectObject `json:"injectObject,omitempty"`
	// Data for "PrependPipeline" type
	PrependPipeline *kptfile.Pipeline `json:"prependPipeline,omitempty"`
	// Data for "AppendPipeline" type
	AppendPipeline *kptfile.Pipeline `json:"appendPipeline,omitempty"`
}

type InjectPackageRevision struct {
	// The package revision to be inserted into the downstream package.
	PackageRevisionRef `json:",inline"`
	// The path within the downstream package to insert the package.
	Subdir string `json:"subdir,omitempty"`
}

type InjectLatestPackageRevision struct {
	// The package revision to be inserted into the downstream package.
	PackageRef `json:",inline"`
	// The path within the downstream package to insert the package.
	Subdir string `json:"subdir,omitempty"`
}

type PackageRevisionRef struct {
	//+required
	Repo string `json:"repo,omitempty"`
	//+required
	Package string `json:"package,omitempty"`
	//+required
	Revision string `json:"revision,omitempty"`
}

type PackageRef struct {
	//+required
	Repo string `json:"repo,omitempty"`
	//+required
	Package string `json:"package,omitempty"`
}

type ObjectRef struct {
	Namespace *string `json:"namespace,omitempty"`
	Name      string  `json:"name"`
}

// InjectObject specifies how to select in-cluster objects for
// resolving injection points.
type InjectObject struct {
	Group       *string   `json:"group,omitempty"`
	Version     *string   `json:"version,omitempty"`
	Kind        *string   `json:"kind,omitempty"`
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

func (m *Mutation) Id() string {
	return m.Manager + "/" + m.Name
}

var (
	PackageVariantGVK = GroupVersion.WithKind("PackageVariant")
)
