// Copyright 2022, 2024 The kpt and Nephio Authors
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

package porch

import (
	"context"

	"github.com/nephio-project/porch/pkg/repository"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewReferenceResolver(coreClient client.Reader) repository.ReferenceResolver {
	return &referenceResolver{
		coreClient: coreClient,
	}
}

type referenceResolver struct {
	coreClient client.Reader
}

var _ repository.ReferenceResolver = &referenceResolver{}

func (r *referenceResolver) ResolveReference(ctx context.Context, namespace, name string, result repository.Object) error {
	return r.coreClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, result)
}
