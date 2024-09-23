package utils

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func TypeMetaOrDie(obj client.Object, scheme *runtime.Scheme) metav1.TypeMeta {
	var err error
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		panic(fmt.Errorf("unable to find GVK for object %v: %w", obj, err))
	}
	return metav1.TypeMeta{
		APIVersion: gvk.GroupVersion().Identifier(),
		Kind:       gvk.Kind,
	}
}

func CreateOwnerReference(obj client.Object, scheme *runtime.Scheme) metav1.OwnerReference {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		var err error
		gvk, err = apiutil.GVKForObject(obj, scheme)
		if err != nil {
			panic(fmt.Errorf("unable to find GVK for object %v: %w", obj, err))
		}
	}
	return *metav1.NewControllerRef(obj, gvk)
}

func CreateOwnerReferenceList(obj client.Object, scheme *runtime.Scheme) []metav1.OwnerReference {
	return []metav1.OwnerReference{CreateOwnerReference(obj, scheme)}
}

// TODO: this must already exist somewhere
func SetAnnotation(obj client.Object, key string, value string) {
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(map[string]string{})
	}
	obj.GetAnnotations()[key] = value
}

func UpdateMap(m map[string]string, toAdd map[string]string) map[string]string {
	if m == nil {
		m = map[string]string{}
	}
	for key, value := range toAdd {
		m[key] = value
	}
	return m
}

func IsOwnedBy(owned client.Object, owner client.Object) bool {
	for _, ref := range owned.GetOwnerReferences() {
		if ref.UID == owner.GetUID() {
			return true
		}
	}
	return false
}
