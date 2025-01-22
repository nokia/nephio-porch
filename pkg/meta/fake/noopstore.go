package fake

import (
	"context"

	configapi "github.com/nephio-project/porch/api/porchconfig/v1alpha1"
	"github.com/nephio-project/porch/pkg/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type NoopMetadataStore struct {
	Metas []metav1.ObjectMeta
}

var _ meta.MetadataStore = &NoopMetadataStore{}

func (ms *NoopMetadataStore) Get(ctx context.Context, namespacedName types.NamespacedName) (metav1.ObjectMeta, error) {
	return metav1.ObjectMeta{Name: namespacedName.Name, Namespace: namespacedName.Namespace}, nil
}

func (ms *NoopMetadataStore) List(context.Context, *configapi.Repository) ([]metav1.ObjectMeta, error) {
	return ms.Metas, nil
}

func (ms *NoopMetadataStore) Create(ctx context.Context, pkgRevMeta metav1.ObjectMeta, repoName string, pkgRevUID types.UID) (metav1.ObjectMeta, error) {
	return pkgRevMeta, nil
}

func (ms *NoopMetadataStore) Update(ctx context.Context, pkgRevMeta metav1.ObjectMeta) (metav1.ObjectMeta, error) {
	return pkgRevMeta, nil
}

func (ms *NoopMetadataStore) Delete(ctx context.Context, namespacedName types.NamespacedName, clearFinalizer bool) (metav1.ObjectMeta, error) {
	return metav1.ObjectMeta{Name: namespacedName.Name, Namespace: namespacedName.Namespace}, nil
}
