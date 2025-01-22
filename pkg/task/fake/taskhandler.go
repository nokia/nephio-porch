package fake

import (
	"context"

	api "github.com/nephio-project/porch/api/porch/v1alpha1"
	configapi "github.com/nephio-project/porch/api/porchconfig/v1alpha1"
	"github.com/nephio-project/porch/internal/kpt/builtins"
	"github.com/nephio-project/porch/internal/kpt/fnruntime"
	"github.com/nephio-project/porch/pkg/kpt/fn"
	"github.com/nephio-project/porch/pkg/repository"
	"github.com/nephio-project/porch/pkg/task"
)

type FakeTaskHandler struct {
	Runtime fn.FunctionRuntime
}

var _ task.TaskHandler = &FakeTaskHandler{}

func (th *FakeTaskHandler) GetRuntime() fn.FunctionRuntime {
	return th.Runtime
}

func (th *FakeTaskHandler) SetRunnerOptionsResolver(func(namespace string) fnruntime.RunnerOptions) {

}

func (th *FakeTaskHandler) SetRuntime(fn.FunctionRuntime) {

}

func (th *FakeTaskHandler) SetRepoOpener(repository.RepositoryOpener) {

}

func (th *FakeTaskHandler) SetCredentialResolver(repository.CredentialResolver) {

}

func (th *FakeTaskHandler) SetReferenceResolver(repository.ReferenceResolver) {

}

func (th *FakeTaskHandler) ApplyTasks(context.Context, repository.PackageRevisionDraft, *configapi.Repository, *api.PackageRevision, *builtins.PackageConfig) error {
	return nil
}

func (th *FakeTaskHandler) DoPRMutations(context.Context, string, repository.PackageRevision, *api.PackageRevision, *api.PackageRevision, repository.PackageRevisionDraft) error {
	return nil
}

func (th *FakeTaskHandler) DoPRResourceMutations(context.Context, repository.PackageRevision, repository.PackageRevisionDraft, *api.PackageRevisionResources, *api.PackageRevisionResources) (*api.RenderStatus, error) {
	return &api.RenderStatus{}, nil
}
