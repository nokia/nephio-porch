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

// this code is based on https://github.com/nephio-project/porch/blob/main/pkg/engine/kio.go

package utils

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ParseKubeObjectsFromPR_FromContext reads KubeObjects from a PackageRevision that is identified by the given repo, pkg, and revision.
// The list of PackageRevisions must be in the context.
func ParseKubeObjectsFromPR_FromContext(
	ctx context.Context,
	client client.Client,
	repo, pkg, revision string,
	hasToBePublished bool,
) (fn.KubeObjects, error) {
	prId := fmt.Sprintf("PackageRevision %s/%s@%s", repo, pkg, revision)
	pr := PackageRevisionsFromContextOrDie(ctx).OfPackage(repo, pkg).Revision(revision)
	if pr == nil {
		return nil, fmt.Errorf("%s not found", prId)
	}
	if hasToBePublished && !porchapi.LifecycleIsPublished(pr.Spec.Lifecycle) {
		return nil, WithReconcileResult(
			fmt.Errorf("%s hasn't been published yet, retrying later", prId),
			ctrl.Result{RequeueAfter: 1 * time.Minute},
		)
	}
	_, kubeObjects, err := ParseKubeObjectsFromPR(ctx, client, pr)
	return kubeObjects, err
}

// ParseKubeObjectsFromPR reads to contents of the given package as KubeObjects
func ParseKubeObjectsFromPR(ctx context.Context, cl client.Client, pr *porchapi.PackageRevision) (
	prr *porchapi.PackageRevisionResources,
	objs fn.KubeObjects,
	err error,
) {
	prrId := fmt.Sprintf("PackageRevisionResources %s/%s@%s", pr.Spec.RepositoryName, pr.Spec.PackageName, pr.Spec.Revision)
	prr = &porchapi.PackageRevisionResources{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(pr), prr); err != nil {
		if apierrors.IsNotFound(err) {
			// It takes some time after creating a new PackageRevision object for the corresponding PackageRevisionResources object
			// to be created, as well. So a missing PRR is handled by a simple retry
			return nil, nil, WithReconcileResult(
				fmt.Errorf("missing %s. this may be temporary, retrying later", prrId),
				ctrl.Result{RequeueAfter: 3 * time.Second},
			)
		}
		return nil, nil, err
	}
	// check if it's a valid PackageRevisionResources object
	if prr.Spec.Resources == nil {
		return nil, nil, fmt.Errorf("nil resources found for %s", prrId)
	}

	// parse all files in the package into one flat KubeObjects list
	objs, _, err = ReadKubeObjects(prr.Spec.Resources)
	return prr, objs, errors.Wrapf(err, "failed to parse %s", prrId)
}

// UpdatePRResources updates the contents of package revision with the given KubeObjects
func UpdatePRResources(ctx context.Context, client client.Client,
	prr *porchapi.PackageRevisionResources, objs fn.KubeObjects) error {

	l := log.FromContext(ctx)
	newContent, err := WriteKubeObjects(objs)
	if err != nil {
		return err
	}

	updateNeeded := false
	for path, content := range newContent {
		if prr.Spec.Resources[path] != content {
			prr.Spec.Resources[path] = content
			updateNeeded = true
		}
	}
	if updateNeeded {
		if err := client.Update(ctx, prr); err != nil {
			return errors.Wrapf(err, "while updating resources of package revision %s/%s", prr.Namespace, prr.Name)
		}
		l.Info(fmt.Sprintf("updated resources of package revision %s/%s", prr.Namespace, prr.Name))
	} else {
		l.Info(fmt.Sprintf("no change in resources of package revision %s/%s", prr.Namespace, prr.Name))
	}
	return nil
}

func ReadKubeObjects(inputFiles map[string]string) (objs fn.KubeObjects, extraFiles map[string]string, err error) {
	extraFiles = make(map[string]string)
	results := []*yaml.RNode{}
	for k, v := range inputFiles {
		base := path.Base(k)
		ext := path.Ext(base)

		// TODO: use authoritative kpt filtering
		if ext != ".yaml" && ext != ".yml" && base != "Kptfile" {
			extraFiles[k] = v
			continue
		}

		reader := &kio.ByteReader{
			Reader: strings.NewReader(v),
			SetAnnotations: map[string]string{
				kioutil.PathAnnotation: k,
			},
			DisableUnwrapping: true,
		}
		var nodes []*yaml.RNode
		nodes, err = reader.Read()
		if err != nil {
			// TODO: fail, or bypass this file too?
			return
		}
		results = append(results, nodes...)
	}

	var rl fn.ResourceList
	for _, node := range results {
		err = rl.UpsertObjectToItems(node, nil, true)
		if err != nil {
			return
		}
	}
	objs = rl.Items
	return
}

func WriteKubeObjects(objs fn.KubeObjects) (map[string]string, error) {
	output := map[string]string{}
	paths := map[string][]*fn.KubeObject{}
	for _, obj := range objs {
		path := getPath(obj)
		paths[path] = append(paths[path], obj)
	}

	// TODO: write directly into the package resources abstraction.
	// For now serializing into memory.
	buf := &bytes.Buffer{}
	for path, objs := range paths {
		bw := kio.ByteWriter{
			Writer: buf,
			ClearAnnotations: []string{
				kioutil.PathAnnotation,
			},
		}

		nodes := []*yaml.RNode{}
		for _, obj := range objs {
			node, err := AsRNode(obj)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
		if err := bw.Write(nodes); err != nil {
			return nil, err
		}
		output[path] = buf.String()
		buf.Reset()
	}
	return output, nil
}

func getPath(node *fn.KubeObject) string {
	ann := node.GetAnnotations()
	if path, ok := ann[kioutil.PathAnnotation]; ok {
		return path
	}
	ns := node.GetNamespace()
	if ns == "" {
		ns = "non-namespaced"
	}
	name := node.GetName()
	if name == "" {
		name = "unnamed"
	}
	// TODO: harden for escaping etc.
	return path.Join(ns, fmt.Sprintf("%s.yaml", name))
}

// AsRNode converts a KubeObject to a yaml.RNode
func AsRNode(obj *fn.KubeObject) (*yaml.RNode, error) {
	// TODO: remove the need for this unnecessary round-trip marshalling by adding a direct conversion method to KubeObject
	return yaml.Parse(obj.String())
}
