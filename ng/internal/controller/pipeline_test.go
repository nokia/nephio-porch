// Copyright 2022 The kpt and Nephio Authors
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

package packagevariant

import (
	"context"
	"testing"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	kptfile "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestEnsureKRMFunctions(t *testing.T) {
	prrBase := `
apiVersion: porch.kpt.dev/v1alpha1
kind: PackageRevisionResources
metadata:
  name: prr
  namespace: default
spec:
  packageName: nephio-system
  repository: nephio-packages
  resources:
    Kptfile: |
      apiVersion: kpt.dev/v1
      kind: Kptfile
      metadata:
        name: prr
        annotations:
          config.kubernetes.io/local-config: "true"
      info:
        description: Example
`[1:]

	testCases := map[string]struct {
		initialPipeline string
		pvPipeline      []api.Mutation
		expectedErr     string
		expectedPrr     string
	}{
		"add one mutator with existing mutators": {
			initialPipeline: `
        mutators:
          - image: gcr.io/kpt-fn/set-labels:v0.1
            name: set-labels
            configMap:
              app: foo
          - image: gcr.io/kpt-fn/set-annotations:v0.1
            name: set-annotations`[1:],
			pvPipeline: []api.Mutation{
				{
					Type: api.MutationTypePrependPipeline,
					Name: "prepend-functions",
					PrependPipeline: &kptfile.Pipeline{
						Mutators: []kptfile.Function{
							{
								Image: "gcr.io/kpt-fn/set-namespace:v0.1",
								Name:  "set-namespace",
								ConfigMap: map[string]string{
									"namespace": "my-ns",
								},
							},
						},
					},
				},
			},
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        mutators:
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/set-namespace-0
          image: gcr.io/kpt-fn/set-namespace:v0.1
          configMap:
            namespace: my-ns
        - image: gcr.io/kpt-fn/set-labels:v0.1
          name: set-labels
          configMap:
            app: foo
        - image: gcr.io/kpt-fn/set-annotations:v0.1
          name: set-annotations
`[1:],
		},
		"add two mutators with existing": {
			initialPipeline: `
        mutators:
          - image: gcr.io/kpt-fn/set-labels:v0.1
            name: set-labels
            configMap:
              app: foo
          - image: gcr.io/kpt-fn/set-annotations:v0.1
            name: set-annotations`[1:],
			pvPipeline: []api.Mutation{
				{
					Type: api.MutationTypePrependPipeline,
					Name: "prepend-functions",
					PrependPipeline: &kptfile.Pipeline{
						Mutators: []kptfile.Function{
							{
								Image: "gcr.io/kpt-fn/set-namespace:v0.1",
								Name:  "set-namespace",
								ConfigMap: map[string]string{
									"namespace": "my-ns",
								},
							},
							{
								Image: "gcr.io/kpt-fn/format:unstable",
								Name:  "format",
							},
						},
					},
				},
			},
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        mutators:
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/set-namespace-0
          image: gcr.io/kpt-fn/set-namespace:v0.1
          configMap:
            namespace: my-ns
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/format-1
          image: gcr.io/kpt-fn/format:unstable
        - image: gcr.io/kpt-fn/set-labels:v0.1
          name: set-labels
          configMap:
            app: foo
        - image: gcr.io/kpt-fn/set-annotations:v0.1
          name: set-annotations
`[1:],
		},
		"add one mutator with none existing": {
			initialPipeline: "",
			pvPipeline: []api.Mutation{
				{
					Type: api.MutationTypePrependPipeline,
					Name: "prepend-functions",
					PrependPipeline: &kptfile.Pipeline{
						Mutators: []kptfile.Function{
							{
								Image: "gcr.io/kpt-fn/set-namespace:v0.1",
								Name:  "set-namespace",
								ConfigMap: map[string]string{
									"namespace": "my-ns",
								},
							},
						},
					},
				},
			},
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        mutators:
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/set-namespace-0
          image: gcr.io/kpt-fn/set-namespace:v0.1
          configMap:
            namespace: my-ns
`[1:],
		},
		"add none with existing mutators": {
			initialPipeline: `
        mutators:
          - image: gcr.io/kpt-fn/set-labels:v0.1
            name: set-labels
            configMap:
              app: foo
          - image: gcr.io/kpt-fn/set-annotations:v0.1
            name: set-annotations`[1:],
			pvPipeline:  nil,
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        mutators:
        - image: gcr.io/kpt-fn/set-labels:v0.1
          name: set-labels
          configMap:
            app: foo
        - image: gcr.io/kpt-fn/set-annotations:v0.1
          name: set-annotations
`[1:],
		},
		"add one mutator with existing with comments": {
			initialPipeline: `
        mutators:
          - image: gcr.io/kpt-fn/set-labels:v0.1
            # this is a comment
            name: set-labels
            configMap:
              app: foo
          - image: gcr.io/kpt-fn/set-annotations:v0.1
            name: set-annotations`[1:],
			pvPipeline: []api.Mutation{
				{
					Type: api.MutationTypePrependPipeline,
					Name: "prepend-functions",
					PrependPipeline: &kptfile.Pipeline{
						Mutators: []kptfile.Function{
							{
								Image: "gcr.io/kpt-fn/set-namespace:v0.1",
								Name:  "set-namespace",
								ConfigMap: map[string]string{
									"namespace": "my-ns",
								},
							},
						},
					},
				},
			},
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        mutators:
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/set-namespace-0
          image: gcr.io/kpt-fn/set-namespace:v0.1
          configMap:
            namespace: my-ns
        - image: gcr.io/kpt-fn/set-labels:v0.1
          # this is a comment
          name: set-labels
          configMap:
            app: foo
        - image: gcr.io/kpt-fn/set-annotations:v0.1
          name: set-annotations
`[1:],
		},
		"add one validator with existing validators": {
			initialPipeline: `
        validators:
          - image: gcr.io/kpt-fn/gatekeeper-validate:v0.1
            name: gatekeeper-validate`[1:],
			pvPipeline: []api.Mutation{
				{
					Type: api.MutationTypePrependPipeline,
					Name: "prepend-functions",
					PrependPipeline: &kptfile.Pipeline{
						Validators: []kptfile.Function{
							{
								Image: "gcr.io/kpt-fn/validate-name:undefined",
								Name:  "validate-name",
							},
						},
					},
				},
			},
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        validators:
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/validate-name-0
          image: gcr.io/kpt-fn/validate-name:undefined
        - image: gcr.io/kpt-fn/gatekeeper-validate:v0.1
          name: gatekeeper-validate
`[1:],
		},

		"add two validators with existing validators": {
			initialPipeline: `
        validators:
          - image: gcr.io/kpt-fn/gatekeeper-validate:v0.1
            name: gatekeeper-validate`[1:],

			pvPipeline: []api.Mutation{
				{
					Type: api.MutationTypePrependPipeline,
					Name: "prepend-functions",
					PrependPipeline: &kptfile.Pipeline{
						Validators: []kptfile.Function{
							{
								Image: "gcr.io/kpt-fn/validate-name:undefined",
								Name:  "validate-name",
							},
						},
					},
				},
			},
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        validators:
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/validate-name-0
          image: gcr.io/kpt-fn/validate-name:undefined
        - image: gcr.io/kpt-fn/gatekeeper-validate:v0.1
          name: gatekeeper-validate
`[1:],
		},
		"add none with existing validator": {
			initialPipeline: `
        validators:
          - image: gcr.io/kpt-fn/gatekeeper-validate:v0.1
            name: gatekeeper-validate`[1:],
			pvPipeline:  nil,
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        validators:
        - image: gcr.io/kpt-fn/gatekeeper-validate:v0.1
          name: gatekeeper-validate
`[1:],
		},
		"add validator and mutator with existing": {
			initialPipeline: `
        validators:
          - image: gcr.io/val1
            name: val1
          - image: gcr.io/val2
            name: val2
        mutators:
          - image: gcr.io/mut1
            name: mut1
          - image: gcr.io/mut2
            name: mut2`[1:],
			pvPipeline: []api.Mutation{
				{
					Type: api.MutationTypePrependPipeline,
					Name: "prepend-functions",
					PrependPipeline: &kptfile.Pipeline{
						Validators: []kptfile.Function{
							{
								Image: "gcr.io/val3",
								Name:  "val3",
							},
							{
								Image: "gcr.io/val4",
								Name:  "val4",
							},
						},
						Mutators: []kptfile.Function{
							{
								Image: "gcr.io/mut3",
								Name:  "mut3",
							},
							{
								Image: "gcr.io/mut4",
								Name:  "mut4",
							},
						},
					},
				},
			},
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        validators:
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/val3-0
          image: gcr.io/val3
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/val4-1
          image: gcr.io/val4
        - image: gcr.io/val1
          name: val1
        - image: gcr.io/val2
          name: val2
        mutators:
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/mut3-0
          image: gcr.io/mut3
        - name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/mut4-1
          image: gcr.io/mut4
        - image: gcr.io/mut1
          name: mut1
        - image: gcr.io/mut2
          name: mut2
`[1:],
		},
		"remove pv mutator": {
			initialPipeline: `
        mutators:
        - image: gcr.io/mut:v1
          name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/mut-0`[1:],
			pvPipeline:  nil,
			expectedErr: "",
			expectedPrr: prrBase + "\n",
		},
		"remove pv validator": {
			initialPipeline: `
        validators:
        - image: gcr.io/val:v1
          name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/val-0`[1:],
			pvPipeline:  nil,
			expectedErr: "",
			expectedPrr: prrBase + "\n",
		},
		"remove pv validator, keep prr one": {
			initialPipeline: `
        validators:
        - image: gcr.io/val:v1
          name: PackageVariant.ng.porch.kpt.dev/my-ns/my-pv//prepend-functions/val-0
        - image: gcr.io/val:v1
          name: non-pv-val`[1:],
			pvPipeline:  nil,
			expectedErr: "",
			expectedPrr: prrBase + `
      pipeline:
        validators:
        - image: gcr.io/val:v1
          name: non-pv-val
`[1:],
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			locPrrBase := prrBase
			if tc.initialPipeline != "" {
				locPrrBase += "      pipeline:\n"
			}
			var prr porchapi.PackageRevisionResources
			require.NoError(t, yaml.Unmarshal([]byte(locPrrBase+tc.initialPipeline), &prr))
			pv := withMutation(&pvBase, tc.pvPipeline...)
			_, actualErr := ensureMutations(context.TODO(), nil, pv, &prr)
			if tc.expectedErr == "" {
				require.NoError(t, actualErr)
			} else {
				require.EqualError(t, actualErr, tc.expectedErr)
			}
			var expectedPRR porchapi.PackageRevisionResources
			require.NoError(t, yaml.Unmarshal([]byte(tc.expectedPrr), &expectedPRR))

			require.Equal(t, expectedPRR, prr)

			// test idempotence
			_, idemErr := ensureMutations(context.TODO(), nil, pv, &prr)
			if tc.expectedErr == "" {
				require.NoError(t, idemErr)
			} else {
				require.EqualError(t, idemErr, tc.expectedErr)
			}
			require.Equal(t, expectedPRR, prr) // check that prr still matches expected
		})
	}
}
