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
	"fmt"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	kptfile "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
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

	DeletionPolicyDelete          DeletionPolicy = "delete"
	DeletionPolicyProposeDeletion DeletionPolicy = "proposeDeletion"
	DeletionPolicyOrphan          DeletionPolicy = "orphan"

	ApprovalPolicyNever                 ApprovalPolicy = "never"
	ApprovalPolicyAlways                ApprovalPolicy = "always"
	ApprovalPolicyInitial               ApprovalPolicy = "initial"
	ApprovalPolicyAlwaysWithManualEdits ApprovalPolicy = "afterManualEdits"

	Finalizer = "config.porch.kpt.dev/packagevariants"
)

// PackageVariantSpec defines the desired state of PackageVariant
type PackageVariantSpec struct {
	// NOTE: using a pointer for struct fields, even if they are required (i.e. upstream/downstream)
	//       helps creating partial objects (patches) for server-side apply

	//+required
	Upstream *PackageRevisionRef `json:"upstream,omitempty"`
	//+required
	Downstream *PackageRef `json:"downstream,omitempty"`

	//+default="adoptNone"
	//+kubebuilder:validation:Enum=adoptExisting;adoptNone
	AdoptionPolicy AdoptionPolicy `json:"adoptionPolicy,omitempty"`
	//+default="proposeDeletion"
	//+kubebuilder:validation:Enum=delete;orphan;proposeDeletion
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
	//+default="never"
	//+kubebuilder:validation:Enum=never;always;initial;afterManualEdits
	ApprovalPolicy ApprovalPolicy `json:"approvalPolicy,omitempty"`
	// Readiness gates added to downstream packages
	ReadinessGates []porchapi.ReadinessGate `json:"readinessGates,omitempty"`
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

type MutationType string

const (
	MutationTypeCustom                      MutationType = "Custom"
	MutationTypeMergePackageRevision        MutationType = "MergePackageRevision"
	MutationTypeInjectPackageRevision       MutationType = "InjectPackageRevision"
	MutationTypeInjectLatestPackageRevision MutationType = "InjectLatestPackageRevision"
	MutationTypeInjectLiveObject            MutationType = "InjectLiveObject"
	MutationTypeInjectInlineObject          MutationType = "InjectInlineObject"
	MutationTypeInjectObjectFromPackage     MutationType = "InjectObjectFromPackage"
	MutationTypePrependPipeline             MutationType = "PrependPipeline"
	MutationTypeAppendPipeline              MutationType = "AppendPipeline"
)

// A mutation that should be applied to the downstream package
// +kubebuilder:validation:XValidation:message="custom field is mandatory if type == Custom",rule="self.type != 'Custom' || has(self.custom)"
// +kubebuilder:validation:XValidation:message="mergePackageRevision field is mandatory if type == MergePackageRevision",rule="self.type != 'MergePackageRevision' || has(self.mergePackageRevision)"
// +kubebuilder:validation:XValidation:message="injectPackageRevision field is mandatory if type == InjectPackageRevision",rule="self.type != 'InjectPackageRevision' || has(self.injectPackageRevision)"
// +kubebuilder:validation:XValidation:message="injectLatestPackageRevision field is mandatory if type == InjectLatestPackageRevision",rule="self.type != 'InjectLatestPackageRevision' || has(self.injectLatestPackageRevision)"
// +kubebuilder:validation:XValidation:message="injectLiveObject field is mandatory if type == InjectLiveObject",rule="self.type != 'InjectLiveObject' || has(self.injectLiveObject)"
// +kubebuilder:validation:XValidation:message="injectInlineObject field is mandatory if type == InjectInlineObject",rule="self.type != 'InjectInlineObject' || has(self.injectInlineObject)"
// +kubebuilder:validation:XValidation:message="injectObjectFromPackage field is mandatory if type == InjectObjectFromPackage",rule="self.type != 'InjectObjectFromPackage' || has(self.injectObjectFromPackage)"
// +kubebuilder:validation:XValidation:message="pipeline field is mandatory if type == PrependPipeline",rule="self.type != 'PrependPipeline' || has(self.pipeline)"
// +kubebuilder:validation:XValidation:message="pipeline field is mandatory if type == AppendPipeline",rule="self.type != 'AppendPipeline' || has(self.pipeline)"
type Mutation struct {
	// Name and Manager fields together must uniquely identify a mutation in the scope of a PackageVariant object.
	//+required
	Name string `json:"name,omitempty"`
	// Name of the controller that manages the mutation (may be empty)
	//+default=""
	Manager string `json:"manager,omitempty"`
	// Type selector enum for the union type
	//+required
	//+kubebuilder:validation:Enum=Custom;PrependPipeline;AppendPipeline;MergePackageRevision;InjectPackageRevision;InjectLatestPackageRevision;InjectObject;InjectLiveObject;InjectInlineObject;InjectObjectFromPackage
	Type MutationType `json:"type,omitempty"`
	// Data for "Custom" type
	Custom *CustomMutation `json:"custom,omitempty"`
	// Data for "MergePackageRevision" type
	MergePackageRevision *MergePackageRevision `json:"mergePackageRevision,omitempty"`
	// Data for "InjectPackageRevision" type
	InjectPackageRevision *InjectPackageRevision `json:"injectPackageRevision,omitempty"`
	// Data for "InjectLatestPackageRevision" type
	InjectLatestPackageRevision *InjectLatestPackageRevision `json:"injectLatestPackageRevision,omitempty"`
	// Data for "InjectLiveObject" type
	InjectLiveObject *InjectLiveObject `json:"injectLiveObject,omitempty"`
	// Data for "InjectInlineObject" type
	InjectInlineObject string `json:"injectInlineObject,omitempty"`
	// Data for "InjectObjectFromPackage" type
	InjectObjectFromPackage *InjectObjectFromPackage `json:"injectObjectFromPackage,omitempty"`
	// Data for "PrependPipeline" and "AppendPipeline" types
	Pipeline *kptfile.Pipeline `json:"pipeline,omitempty"`
}

type CustomMutation struct {
	// unique identifier of the custom mutation type
	//+required
	Kind string `json:"kind,omitempty"`
	//+kubebuilder:pruning:PreserveUnknownFields
	Parameters runtime.RawExtension `json:"parameters,omitempty"`
}

type MergePackageRevision struct {
	// The package revision to be merged into the downstream package.
	PackageRevisionRef `json:",inline"`
	// If false: keep conflicting resources from the upstream package,
	// overwrite them otherwise.
	//+default=true
	Overwrite bool `json:"overwrite,omitempty"`
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

// InjectLiveObject specifies how to select in-cluster objects for
// resolving injection points.
type InjectLiveObject struct {
	Group       *string   `json:"group,omitempty"`
	Version     *string   `json:"version,omitempty"`
	Kind        *string   `json:"kind,omitempty"`
	Source      ObjectRef `json:"source"`
	Destination ObjectRef `json:"destination"`
}

type InjectObjectFromPackage struct {
	// The package revision containing the object to be injected
	PackageRevisionRef `json:",inline"`
	Group              string `json:"group,omitempty"`
	Version            string `json:"version,omitempty"`
	Kind               string `json:"kind,omitempty"`
	// The name/namespace of the KRM object in the package
	Source ObjectRef `json:"source"`
	// The name/namespace of the injection point
	Destination ObjectRef `json:"destination"`
}

// PackageVariantStatus defines the observed state of PackageVariant
type PackageVariantStatus struct {
	// Conditions describes the reconciliation state of the object.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// DownstreamTargets contains the downstream targets that the PackageVariant
	// either created or adopted.
	DownstreamTargets []DownstreamTargetStatus `json:"downstreamTargets,omitempty"`
}

type DownstreamTargetStatus struct {
	Name         string                `json:"name,omitempty"`
	RenderStatus porchapi.RenderStatus `json:"renderStatus,omitempty"`
	Mutations    []MutationStatus      `json:"mutations,omitempty"`
}

type MutationStatus struct {
	Name    string `json:"name"`
	Manager string `json:"manager,omitempty"`
	Applied bool   `json:"applied"`
	Message string `json:"message,omitempty"`
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

func (m *Mutation) ConditionType(pvPrefix string) string {
	return fmt.Sprintf("mutation:%s/%s", pvPrefix, m.Id())
}

var (
	PackageVariantGVK = GroupVersion.WithKind("PackageVariant")
)
