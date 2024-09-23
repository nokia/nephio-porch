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

package utils

import (
	"fmt"
	"sort"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	kptfileapi "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	"github.com/nephio-project/porch/third_party/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

const (
	statusFieldName     = "status"
	conditionsFieldName = "conditions"
)

var (
	BoolToConditionStatus = map[bool]kptfileapi.ConditionStatus{
		true:  kptfileapi.ConditionTrue,
		false: kptfileapi.ConditionFalse,
	}
)

// KptfileObject provides an API to manipulate the conditions of a kpt package,
// more precisely the list of conditions in the status of the Kptfile resource
type KptfileObject struct {
	kptfile *fn.KubeObject
}

func NewKptfileFromKubeObjects(items fn.KubeObjects) (*KptfileObject, error) {
	var ret KptfileObject
	ret.kptfile = items.GetRootKptfile()
	if ret.kptfile == nil {
		return nil, fmt.Errorf("the Kptfile object is missing from the package")

	}
	return &ret, nil
}

func NewKptfileFromResources(resources map[string]string) (*KptfileObject, error) {
	kptfileStr, found := resources[kptfileapi.KptFileName]
	if !found {
		return nil, fmt.Errorf("'%s' is missing from the package", kptfileapi.KptFileName)
	}

	kos, err := ReadKubeObjectsFromString(kptfileStr, kptfileapi.KptFileName)
	if err != nil {
		return nil, err
	}
	return NewKptfileFromKubeObjects(kos)
}

func (kfo *KptfileObject) WriteToResources(resources map[string]string) error {
	if kfo.kptfile == nil {
		return fmt.Errorf("attempt to write empty Kptfile to the package")
	}
	kptfileStr, err := WriteKubeObjectsToString(fn.KubeObjects{kfo.kptfile})
	if err != nil {
		return err
	}
	resources[kptfileapi.KptFileName] = kptfileStr
	return nil
}

// Status returns with the Status field of the Kptfile as a SubObject
func (kfo *KptfileObject) Status() *fn.SubObject {
	return kfo.kptfile.UpsertMap(statusFieldName)
}

func (kfo *KptfileObject) Conditions() fn.SliceSubObjects {
	return kfo.Status().GetSlice(conditionsFieldName)
}

func (kfo *KptfileObject) SetConditions(conditions fn.SliceSubObjects) error {
	sort.SliceStable(conditions, func(i, j int) bool {
		return conditions[i].GetString("type") < conditions[j].GetString("type")
	})
	return kfo.Status().SetSlice(conditions, conditionsFieldName)
}

// TypedConditions returns with (a copy of) the list of current conditions of the kpt package
func (kfo *KptfileObject) TypedConditions() []kptfileapi.Condition {
	statusObj := kfo.kptfile.GetMap(statusFieldName)
	if statusObj == nil {
		return nil
	}
	var status kptfileapi.Status
	err := statusObj.As(&status)
	if err != nil {
		return nil
	}
	return status.Conditions
}

// GetTypedCondition returns with the condition whose type is `conditionType` as its first return value, and
// whether the component exists or not as its second return value
func (kfo *KptfileObject) GetTypedCondition(conditionType string) (kptfileapi.Condition, bool) {
	for _, cond := range kfo.TypedConditions() {
		if cond.Type == conditionType {
			return cond, true
		}
	}
	return kptfileapi.Condition{}, false
}

// SetTypedCondition creates or updates the given condition using the Type field as the primary key
func (kfo *KptfileObject) SetTypedCondition(condition kptfileapi.Condition) error {
	conditions := kfo.Conditions()
	for _, conditionSubObj := range conditions {
		if conditionSubObj.GetString("type") == condition.Type {
			return UpdateStringFields(conditionSubObj, map[string]string{
				"status":  string(condition.Status),
				"reason":  condition.Reason,
				"message": condition.Message,
			})
		}
	}
	ko, err := fn.NewFromTypedObject(condition)
	if err != nil {
		return err
	}
	conditions = append(conditions, &ko.SubObject)
	return kfo.SetConditions(conditions)
}

// DeleteByTpe deletes all conditions with the given type
func (kfo *KptfileObject) DeleteConditionByType(conditionType string) error {
	oldConditions, found, err := kfo.kptfile.NestedSlice(conditionsFieldName)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	newConditions := make([]*fn.SubObject, 0, len(oldConditions))
	for _, c := range oldConditions {
		if c.GetString("type") != conditionType {
			newConditions = append(newConditions, c)
		}
	}
	return kfo.SetConditions(newConditions)
}

func (kfo *KptfileObject) AddReadinessGates(gates []porchapi.ReadinessGate) error {
	info := kfo.kptfile.UpsertMap("info")
	gateObjs := info.GetSlice("readinessGates")
	for _, gate := range gates {
		found := false
		for _, gateObj := range gateObjs {
			if gateObj.GetString("conditionType") == gate.ConditionType {
				found = true
				break
			}
		}
		if !found {
			ko, err := fn.NewFromTypedObject(gate)
			if err != nil {
				return err
			}
			gateObjs = append(gateObjs, &ko.SubObject)
		}
	}
	info.SetSlice(gateObjs, "readinessGates")
	return nil
}
