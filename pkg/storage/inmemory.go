package storage

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/mgoltzsche/k3spi/pkg/pubsub"
	"github.com/mgoltzsche/k3spi/pkg/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

type store struct {
	items  map[string]resource.Resource
	pubsub *pubsub.PubSub
	mutex  *sync.RWMutex
	seq    int64
}

func InMemory() Interface {
	return &store{
		mutex:  &sync.RWMutex{},
		items:  map[string]resource.Resource{},
		pubsub: pubsub.New(),
	}
}

func (r *store) Watch(ctx context.Context) pubsub.Interface {
	return r.pubsub.Subscribe(ctx)
}

func (r *store) List(l runtime.Object) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	v, err := getListPrt(l)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(r.items))
	for k := range r.items {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		appendItem(v, r.items[k])
	}
	return nil
}

func (r *store) Get(key string, res resource.Resource) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	item := r.items[key]
	if item == nil {
		return errors.NewNotFound(res.GetGroupVersionResource().GroupResource(), key)
	}
	return item.DeepCopyIntoResource(res)
}

func (r *store) Create(key string, res resource.Resource) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	existing := r.items[key]
	if existing != nil {
		return errors.NewAlreadyExists(res.GetGroupVersionResource().GroupResource(), key)
	}
	r.setResourceVersion(res)
	r.items[key] = res
	r.emit(pubsub.Added, res)
	return nil
}

func (r *store) Delete(key string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	existing := r.items[key]
	delete(r.items, key)
	if existing != nil {
		r.emit(pubsub.Deleted, existing)
	}
	return nil
}

func (r *store) Update(key string, res resource.Resource, modify func() (resource.Resource, error)) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	existing := r.items[key]
	if existing == nil {
		return errors.NewNotFound(res.GetGroupVersionResource().GroupResource(), key)
	}
	err := existing.DeepCopyIntoResource(res)
	if err != nil {
		return err
	}
	res, err = modify()
	if err != nil {
		return err
	}
	if existing.GetResourceVersion() != res.GetResourceVersion() {
		err := fmt.Errorf("resource was changed concurrently, please fetch the latest resource version and apply your changes again")
		return errors.NewConflict(res.GetGroupVersionResource().GroupResource(), key, err)
	}
	r.setResourceVersion(res)
	r.items[key] = res.DeepCopyObject().(resource.Resource)
	r.emit(pubsub.Modified, res)
	return nil
}

func (r *store) setResourceVersion(o resource.Resource) {
	r.seq++
	o.SetResourceVersion(fmt.Sprintf("%d", r.seq))
}

func (r *store) emit(action pubsub.EventType, res resource.Resource) {
	r.pubsub.Publish(pubsub.Event{Type: pubsub.Added, Object: res})
}

func appendItem(v reflect.Value, obj runtime.Object) {
	v.Set(reflect.Append(v, reflect.ValueOf(obj).Elem()))
}

func getListPrt(listObj runtime.Object) (reflect.Value, error) {
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return reflect.Value{}, err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("need ptr to slice: %v", err)
	}
	return v, nil
}
