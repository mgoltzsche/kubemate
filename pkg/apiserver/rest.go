package apiserver

import (
	"context"
	"fmt"

	"github.com/mgoltzsche/k3spi/pkg/resource"
	store "github.com/mgoltzsche/k3spi/pkg/storage"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
)

var (
	_ registryrest.Lister  = &REST{}
	_ registryrest.Getter  = &REST{}
	_ registryrest.Updater = &REST{}
)

type REST struct {
	resource      resource.Resource
	groupResource schema.GroupResource
	Store         store.Interface
	registryrest.TableConvertor
}

func NewREST(res resource.Resource) *REST {
	gr := res.GetGroupVersionResource().GroupResource()
	return &REST{
		resource:       res,
		groupResource:  gr,
		Store:          store.InMemory(),
		TableConvertor: registryrest.NewDefaultTableConvertor(gr),
	}
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
	err := r.Store.List(l)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (r *REST) Watch(ctx context.Context, options *metainternalversion.ListOptions) (w watch.Interface, err error) {
	return r.Store.Watch(ctx), nil
}

func (r *REST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	o := r.resource.New()
	err := r.Store.Get(name, o)
	if err != nil {
		return nil, err
	}
	return o, nil
}

func (r *REST) Update(ctx context.Context, key string, objInfo registryrest.UpdatedObjectInfo, createValidation registryrest.ValidateObjectFunc, updateValidation registryrest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	obj := r.resource.New()
	// TODO: delete resource when deletionTimestamp set and finalizers cleared?!
	err := r.Store.Update(key, obj, func() (resource.Resource, error) {
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
