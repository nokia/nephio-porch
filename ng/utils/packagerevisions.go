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
	"context"
	"fmt"
	"strconv"
	"strings"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PackageRevisions represents a list of PackageRevisions with useful methods
type PackageRevisions []porchapi.PackageRevision

// The key used to store PackageRevisions in a Context
type PackageRevisionsContextKey struct{}

var packageRevisionsContextKey PackageRevisionsContextKey

// PackageRevisionsFromContextOrDie extracts the PackageRevisions object from the context
func PackageRevisionsFromContextOrDie(ctx context.Context) PackageRevisions {
	prs, ok := ctx.Value(packageRevisionsContextKey).(PackageRevisions)
	if !ok {
		panic("Logical error: PackageRevisions object is missing from the context")
	}
	return prs
}

// WithPackageRevisions returns a new context with the given PackageRevisions object
func WithPackageRevisions(ctx context.Context, prs PackageRevisions) context.Context {
	return context.WithValue(ctx, packageRevisionsContextKey, prs)
}

// OfPackage filters the list for revisions of a particular package
func (prs PackageRevisions) OfPackage(repo string, pkg string) PackageRevisions {
	var ret PackageRevisions
	for _, pr := range prs {
		if pr.Spec.RepositoryName == repo && pr.Spec.PackageName == pkg {
			ret = append(ret, pr)
		}
	}
	return ret
}

// Drafts filters the list for PackageRevisions whose lifecycle is "draft"
func (prs PackageRevisions) Drafts() PackageRevisions {
	var ret PackageRevisions
	for _, pr := range prs {
		if pr.Spec.Lifecycle == porchapi.PackageRevisionLifecycleDraft {
			ret = append(ret, pr)
		}
	}
	return ret
}

// Latest returns with the first PackageRevisions in the list with the "latest" label,
// and returns with nil if there are none
func (prs PackageRevisions) Latest() *porchapi.PackageRevision {
	for _, pr := range prs {
		if metav1.HasLabel(pr.ObjectMeta, porchapi.LatestPackageRevisionKey) &&
			pr.Labels[porchapi.LatestPackageRevisionKey] == porchapi.LatestPackageRevisionValue {
			return &pr
		}
	}
	return nil
}

// Revision returns with the first PackageRevision with the given revision, or nil if it doesn't exist
func (prs PackageRevisions) Revision(revision string) *porchapi.PackageRevision {
	if revision == "" {
		return nil
	}
	for _, pr := range prs {
		if pr.Spec.Revision == revision {
			return &pr
		}
	}
	return nil
}

// SingleItem returns with the single PackageRevision in the list, or an error if there are none or more than one
func (prs PackageRevisions) SingleItem() (*porchapi.PackageRevision, error) {
	if len(prs) > 1 {
		return nil, fmt.Errorf("more than one (%v) PackageRevision was found, but exactly 1 is expected", len(prs))
	}
	if len(prs) == 0 {
		return nil, fmt.Errorf("PackageRevision wasn't found")
	}
	return &prs[0], nil
}

// NewWorkspaceName generates a new unique workspace name based on the given prefix
func (prs PackageRevisions) NewWorkspaceName(prefix string) porchapi.WorkspaceName {
	maxWsNum := 0
	for _, pr := range prs {
		wsName := string(pr.Spec.WorkspaceName)
		if !strings.HasPrefix(wsName, prefix) {
			continue
		}
		wsNum, _ := strconv.Atoi(strings.TrimPrefix(wsName, prefix))
		if wsNum > maxWsNum {
			maxWsNum = wsNum
		}
	}
	return porchapi.WorkspaceName(fmt.Sprintf("%s%d", prefix, maxWsNum+1))
}

// NewDraftPR creates a new draft PackageRevision for a package in a repository
func NewDraftPR(
	ctx context.Context, client client.Client,
	namespace string, repo string, pkg string, workspace porchapi.WorkspaceName,
) (*porchapi.PackageRevision, error) {

	l := log.FromContext(ctx)
	newPR := &porchapi.PackageRevision{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageRevision",
			APIVersion: porchapi.SchemeGroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: porchapi.PackageRevisionSpec{
			RepositoryName: repo,
			PackageName:    pkg,
			Revision:       "",
			WorkspaceName:  workspace,
		},
	}
	if err := client.Create(ctx, newPR); err != nil {
		return nil, err
	}
	l.Info(fmt.Sprintf("-> Created new draft package revision (%v) for package %q", newPR.Name, pkg))
	return newPR, nil
}
