package apiserver

import (
	"context"
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

var (
	_ registryrest.Lister          = &REST{}
	_ registryrest.Getter          = &REST{}
	_ registryrest.Creater         = &REST{}
	_ registryrest.Updater         = &REST{}
	_ registryrest.GracefulDeleter = &REST{}
)

type REST struct {
	resource      resource.Resource
	groupResource schema.GroupResource
	store         storage.Interface
	registryrest.TableConvertor
}

func NewREST(res resource.Resource, store storage.Interface) *REST {
	gr := res.GetGroupVersionResource().GroupResource()
	return &REST{
		resource:       res,
		groupResource:  gr,
		store:          store,
		TableConvertor: registryrest.NewDefaultTableConvertor(gr),
	}
}

func (r *REST) Store() storage.Interface {
	return r.store
}

func (r *REST) New() runtime.Object {
	return r.resource.New()
}

func (r *REST) NewList() runtime.Object {
	return r.resource.NewList()
}

func (r *REST) NamespaceScoped() bool {
	return false
}

func (r *REST) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	l := r.NewList()
	err := r.store.List(l)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *REST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (w watch.Interface, err error) {
	w, err = r.store.Watch(ctx, options.ResourceVersion)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	o := r.resource.New()
	err := r.store.Get(name, o)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation registryrest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	err := createValidation(ctx, obj)
	if err != nil {
		return nil, err
	}
	m, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	err = r.store.Create(m.GetName(), obj.(resource.Resource))
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (r *REST) Update(ctx context.Context, key string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	obj := r.resource.New()
	// TODO: delete resource when deletionTimestamp set and finalizers cleared?!
	err := r.store.Update(key, obj, func() (resource.Resource, error) {
		updatedObj, err := objInfo.UpdatedObject(ctx, obj)
		if err != nil {
			return nil, fmt.Errorf("get updated object: %w", err)
		}
		if updateValidation != nil { // TODO: is this condition really needed?
			if err := updateValidation(ctx, updatedObj, obj); err != nil {
				return nil, err
			}
		}
		obj = updatedObj.(resource.Resource)
		return obj, nil
	})
	if err != nil {
		return nil, false, err
	}
	return obj, false, nil
}

func (r *REST) Delete(ctx context.Context, key string, deleteValidation registryrest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	res := r.New().(resource.Resource)
	err := r.store.Delete(key, res, func() error {
		return deleteValidation(ctx, res)
	})
	if err != nil {
		return nil, false, err
	}
	return res, false, err
}
