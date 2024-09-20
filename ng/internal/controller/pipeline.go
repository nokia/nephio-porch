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

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	kptfilev1 "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
)

// addToPipeline is a mutator that adds KRM functions to the pipeline of the kpt package.
// It generates a unique name that identifies the func (see func generatePVFuncname) and moves it to the top of the mutator sequence.
// It does not preserve yaml indent-style.
type addToPipeline struct {
	mutation *api.Mutation
	pvKey    types.NamespacedName
}

var _ mutator = &addToPipeline{}

func (m *addToPipeline) Apply(ctx context.Context, prr *porchapi.PackageRevisionResources) error {
	newPipeline := map[string][]kptfilev1.Function{
		"validators": m.mutation.Pipeline.Validators,
		"mutators":   m.mutation.Pipeline.Mutators,
	}

	// parse kptfile
	kptfile, err := getFileKubeObject(prr, kptfilev1.KptFileName, "", "")
	if err != nil {
		return err
	}

	pipelineObj := kptfile.UpsertMap("pipeline")
	for fieldname, newFunctions := range newPipeline {
		// do not touch Kptfile if it is not necessary
		if len(newFunctions) == 0 {
			continue
		}

		// get existing functions
		oldFunctionObjs := pipelineObj.GetSlice(fieldname)
		if oldFunctionObjs == nil {
			oldFunctionObjs = fn.SliceSubObjects{}
		}

		// prepare new functions to inject
		var newFunctionObjs = fn.SliceSubObjects{}
		for i, newFunction := range newFunctions {
			newFuncObj, err := fn.NewFromTypedObject(newFunction)
			if err != nil {
				return err
			}
			name := fmt.Sprintf("%s/%s/%s-%d", pvPrefix(m.pvKey), m.mutation.Id(), newFunction.Name, i)
			err = newFuncObj.SetNestedString(name, "name")
			if err != nil {
				return err
			}
			newFunctionObjs = append(newFunctionObjs, &newFuncObj.SubObject)
		}
		// inject the functions
		var result fn.SliceSubObjects
		switch m.mutation.Type {
		case api.MutationTypePrependPipeline:
			result = append(newFunctionObjs, oldFunctionObjs...)
		case api.MutationTypeAppendPipeline:
			result = append(oldFunctionObjs, newFunctionObjs...)
		}
		if err := pipelineObj.SetSlice(result, fieldname); err != nil {
			return err
		}
	}

	// update kptfile
	prr.Spec.Resources[kptfilev1.KptFileName] = kptfile.String()
	return nil
}
