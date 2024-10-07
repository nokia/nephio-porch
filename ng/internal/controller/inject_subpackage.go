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
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	configapi "github.com/nephio-project/porch/api/porchconfig/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"github.com/nephio-project/porch/ng/utils"
	kptfileapi "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	"github.com/nephio-project/porch/third_party/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

// injectSubPackage is a mutator that injects a sub-package into the target package revision.
// It is used to implement the InjectPackageRevision and InjectLatestPackageRevision mutations.
// It uses the KptFile of the injected sub-package to track who injected it and from which upstream source,
// so it can recognize if the sub-package was already injected with the right parameters, without checking the file contents.
// The InjectPackageRevision type mutation injects the sub-package once and allows changes to it afterwards.
// The InjectLatestPackageRevision type mutation injects the latest sub-package revision and automatically re-injects it,
// if a newer revision of the sub-package is published.
type injectSubPackage struct {
	mutation *api.Mutation
	client   client.Client
	pvKey    types.NamespacedName
}

var _ mutator = &injectSubPackage{}

func (m *injectSubPackage) Apply(ctx context.Context, prr *porchapi.PackageRevisionResources) error {
	l := log.FromContext(ctx)
	prs := utils.PackageRevisionsFromContextOrDie(ctx)

	// get the parameters of injection
	var cfg api.InjectPackageRevision
	switch m.mutation.Type {
	case api.MutationTypeInjectPackageRevision:
		cfg = *m.mutation.InjectPackageRevision
	case api.MutationTypeInjectLatestPackageRevision:
		cfg.Repo = m.mutation.InjectLatestPackageRevision.Repo
		cfg.Package = m.mutation.InjectLatestPackageRevision.Package
		pr := prs.OfPackage(cfg.Repo, cfg.Package).Latest()
		if pr == nil {
			return fmt.Errorf("couldn't find latest package revision to inject from %v/%v", cfg.Repo, cfg.Package)
		}
		cfg.Revision = pr.Spec.Revision
		cfg.Subdir = m.mutation.InjectLatestPackageRevision.Subdir
	default:
		return fmt.Errorf("unsupported mutation type for package injection: %s", m.mutation.Type)
	}
	if cfg.Subdir == "" {
		cfg.Subdir = cfg.Package
	}

	// get the Repository of the sub-package to inject
	var repo configapi.Repository
	if err := m.client.Get(ctx, client.ObjectKey{Name: cfg.Repo, Namespace: m.pvKey.Namespace}, &repo); err != nil {
		return fmt.Errorf("couldn't read Repository %q: %w", cfg.Repo, err)
	}
	if repo.Spec.Git == nil {
		return fmt.Errorf("injecting sub-packages is only supported from git repositories, but %q is not a Git repository", cfg.Repo)
	}

	// find sub-packages that was injected by us previously
	kobjs, _, err := utils.ReadKubeObjects(prr.Spec.Resources)
	if err != nil {
		return fmt.Errorf("couldn't read KubeObjects from PackageRevisionResources %q: %w", client.ObjectKeyFromObject(prr), err)
	}
	kptfilesInjectedByUs := kobjs.
		Where(fn.IsGroupKind(kptfileapi.KptFileGVK().GroupKind())).
		Where(fn.HasAnnotations(map[string]string{
			InjectedByResourceAnnotation: m.pvKey.String(),
			InjectedByMutationAnnotation: m.mutation.Id(),
		}))

	// check if a sub-package was already injected with the current parameters
	injectionDone := false
	subdirsToDelete := make([]string, 0)
	for _, kptfile := range kptfilesInjectedByUs {
		if !strings.Contains(kptfile.PathAnnotation(), "/") {
			l.Info("! warning: KptFile resource injected by PackageVariant has no / in its path annotation")
			continue
		}
		if injectedBySameMutation(kptfile, &cfg, &repo) {
			// we found a KptFile that we injected before with the current parameters
			injectionDone = true
		} else {
			// delete packages previously injected by us that doesn't match the current injection parameters
			subdirsToDelete = append(subdirsToDelete, filepath.Dir(kptfile.PathAnnotation()))
		}
	}

	if !injectionDone {
		// delete everything from the target subdir if we are going to inject a new package
		subdirsToDelete = append(subdirsToDelete, cfg.Subdir)
	}
	prr.Spec.Resources = deleteSubDirs(subdirsToDelete, prr.Spec.Resources)

	// quit if we have nothing left to do
	if injectionDone {
		return nil
	}

	// Fetch the sub-package to inject
	prToInject := prs.OfPackage(cfg.Repo, cfg.Package).Revision(cfg.Revision)
	if prToInject == nil {
		return fmt.Errorf("couldn't find package revision to inject: %v/%v/%v", cfg.Repo, cfg.Package, cfg.Revision)
	}
	if !porchapi.LifecycleIsPublished(prToInject.Spec.Lifecycle) {
		return fmt.Errorf("package revision to inject (%v/%v/%v) must be published, but it's lifecycle state is %s", cfg.Repo, cfg.Package, cfg.Revision, prToInject.Spec.Lifecycle)
	}
	if prToInject.Spec.Revision == repo.Spec.Git.Branch {
		return fmt.Errorf("injecting the %q revision is deliberately not supported", repo.Spec.Git.Branch)
	}
	var prrToInject porchapi.PackageRevisionResources
	if err := m.client.Get(ctx, client.ObjectKeyFromObject(prToInject), &prrToInject); err != nil {
		return fmt.Errorf("couldn't read the package revision that should be inserted (%s/%s/%s): %w", cfg.Repo, cfg.Package, cfg.Revision, err)
	}

	// Inject the resources
	kptfileFound := false
	for filename, content := range prrToInject.Spec.Resources {
		if filename == kptfileapi.KptFileName {
			// register that we injected this package
			kptfile, err := fn.ParseKubeObject([]byte(content))
			if err != nil {
				return fmt.Errorf("couldn't parse KptFile of package revision to be injected (%s/%s/%s): %w", cfg.Repo, cfg.Package, cfg.Revision, err)
			}
			kptfile.SetAnnotation(InjectedByResourceAnnotation, m.pvKey.String())
			kptfile.SetAnnotation(InjectedByMutationAnnotation, m.mutation.Id())
			upstream := upstreamGit(&cfg, &repo)
			kptfile.SetNestedString(upstream.Repo, "upstream", "git", "repo")
			kptfile.SetNestedString(upstream.Directory, "upstream", "git", "directory")
			kptfile.SetNestedString(upstream.Ref, "upstream", "git", "ref")
			content = kptfile.String()
			kptfileFound = true
		}
		prr.Spec.Resources[cfg.Subdir+"/"+filename] = content
	}
	if !kptfileFound {
		return fmt.Errorf("couldn't find KptFile in the package revision to be injected (%s/%s/%s)", cfg.Repo, cfg.Package, cfg.Revision)
	}
	return nil
}

// injectedBySameMutation checks if the sub-package of the given KptFile was injected
// by a mutation with the same parameters as `cfg`
func injectedBySameMutation(kptfile *fn.KubeObject, cfg *api.InjectPackageRevision, repo *configapi.Repository) bool {
	expectedUpstream := upstreamGit(cfg, repo)
	if cfg.Subdir != filepath.Dir(kptfile.PathAnnotation()) {
		return false
	}
	gitRepo, found, err := kptfile.NestedString("upstream", "git", "repo")
	if err != nil || !found || gitRepo != expectedUpstream.Repo {
		return false
	}
	dir, found, err := kptfile.NestedString("upstream", "git", "directory")
	if err != nil || !found || dir != expectedUpstream.Directory {
		return false
	}
	ref, found, err := kptfile.NestedString("upstream", "git", "ref")
	if err != nil || !found || ref != expectedUpstream.Ref {
		return false
	}
	return true
}

// upstreamGit returns the upstream field of the Kptfile of a package injected by the given parameters
func upstreamGit(cfg *api.InjectPackageRevision, repo *configapi.Repository) kptfileapi.Git {
	return kptfileapi.Git{
		Repo:      repo.Spec.Git.Repo,
		Directory: strings.TrimLeft(fmt.Sprintf("%s/%s", repo.Spec.Git.Directory, cfg.Package), "/"),
		Ref:       strings.TrimLeft(fmt.Sprintf("%s/%s/%s", repo.Spec.Git.Directory, cfg.Package, cfg.Revision), "/"),
	}
}
