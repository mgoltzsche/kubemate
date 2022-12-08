package storage

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"

	"github.com/mgoltzsche/kubemate/pkg/pubsub"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

type inMemoryStore struct {
	items  map[string]resource.Resource
	pubsub *pubsub.PubSub
	mutex  *sync.RWMutex
	seq    int64
}

func InMemory() *inMemoryStore {
	return &inMemoryStore{
		mutex:  &sync.RWMutex{},
		items:  map[string]resource.Resource{},
		pubsub: pubsub.New(),
	}
}

func (s *inMemoryStore) Watch(ctx context.Context, resourceVersion string) (pubsub.Interface, error) {
	if resourceVersion != "" && resourceVersion != fmt.Sprintf("%d", s.seq) {
		return nil, errors.NewGone(fmt.Sprintf("provided resource version %q is outdated", resourceVersion))
	}
	return s.pubsub.Subscribe(ctx), nil
}

func (s *inMemoryStore) List(l runtime.Object) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	v, err := getListPrt(l)
	if err != nil {
		return err
	}
	m, err := meta.ListAccessor(l)
	if err != nil {
		return err
	}
	m.SetResourceVersion(fmt.Sprintf("%d", s.seq))
	keys := make([]string, 0, len(s.items))
	for k := range s.items {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		res := s.items[k].DeepCopyObject().(resource.Resource)
		s.setGVK(res)
		appendItem(v, res)
	}
	return nil
}

func (s *inMemoryStore) Get(key string, res resource.Resource) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	item := s.items[key]
	if item == nil {
		return errors.NewNotFound(res.GetGroupVersionResource().GroupResource(), key)
	}
	return item.DeepCopyIntoResource(res)
}

func (s *inMemoryStore) Create(key string, res resource.Resource) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	existing := s.items[key]
	if existing != nil {
		return errors.NewAlreadyExists(res.GetGroupVersionResource().GroupResource(), key)
	}
	s.setGVK(res)
	s.setNameAndCreationTimestamp(res, key)
	s.setResourceVersion(res)
	s.items[key] = res
	s.emit(pubsub.Added, res)
	return nil
}

func (s *inMemoryStore) Delete(key string, o resource.Resource, validate func() error) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	existing := s.items[key]
	if existing == nil {
		return errors.NewNotFound(o.GetGroupVersionResource().GroupResource(), key)
	}
	err := existing.DeepCopyIntoResource(o)
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	err = validate()
	if err != nil {
		return fmt.Errorf("delete resource: %w", err)
	}
	delete(s.items, key)
	s.emit(pubsub.Deleted, existing)
	return nil
}

func (s *inMemoryStore) Update(key string, res resource.Resource, modify func() (resource.Resource, error)) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	existing := s.items[key]
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
	if res == nil {
		return fmt.Errorf("inmemory store update: modify() returned nil %T", res)
	}
	if existing.GetResourceVersion() != res.GetResourceVersion() {
		err := fmt.Errorf("resource was changed concurrently, please fetch the latest resource version and apply your changes again")
		return errors.NewConflict(res.GetGroupVersionResource().GroupResource(), key, err)
	}
	updated := res.DeepCopyObject().(resource.Resource)
	s.setGVK(updated)
	s.setResourceVersion(updated)
	s.items[key] = updated
	s.emit(pubsub.Modified, updated)
	return nil
}

func (s *inMemoryStore) setResourceVersion(o resource.Resource) {
	s.seq++
	o.SetResourceVersion(fmt.Sprintf("%d", s.seq))
}

func (s *inMemoryStore) setNameAndCreationTimestamp(o resource.Resource, name string) {
	m, err := meta.Accessor(o)
	if err != nil {
		return
	}
	m.SetName(name)
	t := m.GetCreationTimestamp()
	if t.IsZero() {
		m.SetCreationTimestamp(metav1.Now())
	}
}

func (s *inMemoryStore) setGVK(res resource.Resource) {
	m, err := meta.TypeAccessor(res)
	if err != nil {
		return
	}
	gvk := res.GetObjectKind().GroupVersionKind()
	m.SetAPIVersion(gvk.GroupVersion().String())
	m.SetKind(gvk.Kind)
}

func (s *inMemoryStore) emit(action pubsub.EventType, res resource.Resource) {
	s.pubsub.Publish(pubsub.Event{Type: action, Object: res})
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
