//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	porchv1alpha1 "github.com/nephio-project/porch/api/porch/v1alpha1"
	"github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DownstreamTargetStatus) DeepCopyInto(out *DownstreamTargetStatus) {
	*out = *in
	in.RenderStatus.DeepCopyInto(&out.RenderStatus)
	if in.Mutations != nil {
		in, out := &in.Mutations, &out.Mutations
		*out = make([]MutationStatus, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DownstreamTargetStatus.
func (in *DownstreamTargetStatus) DeepCopy() *DownstreamTargetStatus {
	if in == nil {
		return nil
	}
	out := new(DownstreamTargetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InjectLatestPackageRevision) DeepCopyInto(out *InjectLatestPackageRevision) {
	*out = *in
	out.PackageRef = in.PackageRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InjectLatestPackageRevision.
func (in *InjectLatestPackageRevision) DeepCopy() *InjectLatestPackageRevision {
	if in == nil {
		return nil
	}
	out := new(InjectLatestPackageRevision)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InjectLiveObject) DeepCopyInto(out *InjectLiveObject) {
	*out = *in
	if in.Group != nil {
		in, out := &in.Group, &out.Group
		*out = new(string)
		**out = **in
	}
	if in.Version != nil {
		in, out := &in.Version, &out.Version
		*out = new(string)
		**out = **in
	}
	if in.Kind != nil {
		in, out := &in.Kind, &out.Kind
		*out = new(string)
		**out = **in
	}
	in.Source.DeepCopyInto(&out.Source)
	in.Destination.DeepCopyInto(&out.Destination)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InjectLiveObject.
func (in *InjectLiveObject) DeepCopy() *InjectLiveObject {
	if in == nil {
		return nil
	}
	out := new(InjectLiveObject)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InjectObjectFromPackage) DeepCopyInto(out *InjectObjectFromPackage) {
	*out = *in
	out.PackageRevisionRef = in.PackageRevisionRef
	in.Source.DeepCopyInto(&out.Source)
	in.Destination.DeepCopyInto(&out.Destination)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InjectObjectFromPackage.
func (in *InjectObjectFromPackage) DeepCopy() *InjectObjectFromPackage {
	if in == nil {
		return nil
	}
	out := new(InjectObjectFromPackage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InjectPackageRevision) DeepCopyInto(out *InjectPackageRevision) {
	*out = *in
	out.PackageRevisionRef = in.PackageRevisionRef
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InjectPackageRevision.
func (in *InjectPackageRevision) DeepCopy() *InjectPackageRevision {
	if in == nil {
		return nil
	}
	out := new(InjectPackageRevision)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Mutation) DeepCopyInto(out *Mutation) {
	*out = *in
	if in.InjectPackageRevision != nil {
		in, out := &in.InjectPackageRevision, &out.InjectPackageRevision
		*out = new(InjectPackageRevision)
		**out = **in
	}
	if in.InjectLatestPackageRevision != nil {
		in, out := &in.InjectLatestPackageRevision, &out.InjectLatestPackageRevision
		*out = new(InjectLatestPackageRevision)
		**out = **in
	}
	if in.InjectLiveObject != nil {
		in, out := &in.InjectLiveObject, &out.InjectLiveObject
		*out = new(InjectLiveObject)
		(*in).DeepCopyInto(*out)
	}
	if in.InjectObjectFromPackage != nil {
		in, out := &in.InjectObjectFromPackage, &out.InjectObjectFromPackage
		*out = new(InjectObjectFromPackage)
		(*in).DeepCopyInto(*out)
	}
	if in.PrependPipeline != nil {
		in, out := &in.PrependPipeline, &out.PrependPipeline
		*out = new(v1.Pipeline)
		(*in).DeepCopyInto(*out)
	}
	if in.AppendPipeline != nil {
		in, out := &in.AppendPipeline, &out.AppendPipeline
		*out = new(v1.Pipeline)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Mutation.
func (in *Mutation) DeepCopy() *Mutation {
	if in == nil {
		return nil
	}
	out := new(Mutation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MutationStatus) DeepCopyInto(out *MutationStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MutationStatus.
func (in *MutationStatus) DeepCopy() *MutationStatus {
	if in == nil {
		return nil
	}
	out := new(MutationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObjectRef) DeepCopyInto(out *ObjectRef) {
	*out = *in
	if in.Namespace != nil {
		in, out := &in.Namespace, &out.Namespace
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObjectRef.
func (in *ObjectRef) DeepCopy() *ObjectRef {
	if in == nil {
		return nil
	}
	out := new(ObjectRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageRef) DeepCopyInto(out *PackageRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageRef.
func (in *PackageRef) DeepCopy() *PackageRef {
	if in == nil {
		return nil
	}
	out := new(PackageRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageRevisionRef) DeepCopyInto(out *PackageRevisionRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageRevisionRef.
func (in *PackageRevisionRef) DeepCopy() *PackageRevisionRef {
	if in == nil {
		return nil
	}
	out := new(PackageRevisionRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageVariant) DeepCopyInto(out *PackageVariant) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageVariant.
func (in *PackageVariant) DeepCopy() *PackageVariant {
	if in == nil {
		return nil
	}
	out := new(PackageVariant)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PackageVariant) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageVariantList) DeepCopyInto(out *PackageVariantList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PackageVariant, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageVariantList.
func (in *PackageVariantList) DeepCopy() *PackageVariantList {
	if in == nil {
		return nil
	}
	out := new(PackageVariantList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PackageVariantList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageVariantSpec) DeepCopyInto(out *PackageVariantSpec) {
	*out = *in
	out.Upstream = in.Upstream
	out.Downstream = in.Downstream
	if in.ReadinessGates != nil {
		in, out := &in.ReadinessGates, &out.ReadinessGates
		*out = make([]porchv1alpha1.ReadinessGate, len(*in))
		copy(*out, *in)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Mutations != nil {
		in, out := &in.Mutations, &out.Mutations
		*out = make([]Mutation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageVariantSpec.
func (in *PackageVariantSpec) DeepCopy() *PackageVariantSpec {
	if in == nil {
		return nil
	}
	out := new(PackageVariantSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PackageVariantStatus) DeepCopyInto(out *PackageVariantStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.DownstreamTargets != nil {
		in, out := &in.DownstreamTargets, &out.DownstreamTargets
		*out = make([]DownstreamTargetStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PackageVariantStatus.
func (in *PackageVariantStatus) DeepCopy() *PackageVariantStatus {
	if in == nil {
		return nil
	}
	out := new(PackageVariantStatus)
	in.DeepCopyInto(out)
	return out
}
