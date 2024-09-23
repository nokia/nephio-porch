package utils

import (
	"fmt"

	"github.com/nephio-project/porch/third_party/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

func SingleItem(kobjs fn.KubeObjects) (*fn.KubeObject, error) {
	if len(kobjs) == 0 {
		return nil, fmt.Errorf("zero objects found")
	}
	if len(kobjs) > 1 {
		return nil, fmt.Errorf("%v objects found, but exactly 1 is expected", len(kobjs))
	}
	return kobjs[0], nil
}

func SingleItemAs[T any](kobjs fn.KubeObjects) (*T, error) {
	kobj, err := SingleItem(kobjs)
	if err != nil {
		return nil, err
	}

	var x T
	err = kobj.As(&x)
	return &x, err
}

func UpsertTypedObject(objDB *fn.KubeObjects, obj any) error {
	if obj == nil {
		return fmt.Errorf("obj is nil")
	}

	newKubeObj, err := fn.NewFromTypedObject(obj)
	if err != nil {
		return err
	}

	for i, kobj := range *objDB {
		if newKubeObj.GroupVersionKind() == kobj.GroupVersionKind() && newKubeObj.GetName() == kobj.GetName() && newKubeObj.GetNamespace() == kobj.GetNamespace() {
			(*objDB)[i] = kobj
			return nil
		}
	}
	*objDB = append(*objDB, newKubeObj)
	return nil
}

func CreateOrUpdate(objDB *fn.KubeObjects, defaultObj *fn.KubeObject, updater func() error) error {
	if defaultObj == nil {
		return fmt.Errorf("obj is nil")
	}

	objIndex := -1
	for i, kobj := range *objDB {
		if kobj.GroupVersionKind() == defaultObj.GroupVersionKind() && kobj.GetName() == defaultObj.GetName() && kobj.GetNamespace() == defaultObj.GetNamespace() {
			objIndex = i
			break
		}
	}

	if objIndex == -1 {
		*objDB = append(*objDB, defaultObj)
	} else {
		*defaultObj = *(*objDB)[objIndex]
		(*objDB)[objIndex] = defaultObj
	}
	return updater()
}

func UpdateStringFields(obj *fn.SubObject, values map[string]string) error {
	for field, value := range values {
		if obj.GetString(field) != value {
			err := obj.SetNestedString(value, field)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
