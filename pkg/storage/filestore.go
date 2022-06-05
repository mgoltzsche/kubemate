package storage

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/mgoltzsche/kubemate/pkg/pubsub"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	storagecodec "k8s.io/apiserver/pkg/server/storage"
)

type filestore struct {
	mutex *sync.RWMutex
	*inMemoryStore
	codec runtime.Codec
	dir   string
}

func FileStore(dir string, obj resource.Resource, scheme *runtime.Scheme) (Interface, error) {
	codec, _, err := storagecodec.NewStorageCodec(storagecodec.StorageCodecConfig{
		StorageMediaType:  runtime.ContentTypeJSON,
		StorageSerializer: serializer.NewCodecFactory(scheme),
		StorageVersion:    scheme.PrioritizedVersionsForGroup(obj.GetGroupVersionResource().Group)[0],
		MemoryVersion:     scheme.PrioritizedVersionsForGroup(obj.GetGroupVersionResource().Group)[0],
	})
	if err != nil {
		return nil, err
	}
	inmemory, err := loadFromFiles(dir, obj, codec)
	if err != nil {
		return nil, fmt.Errorf("init filestore: read %s: %w", obj.GetGroupVersionResource().Resource, err)
	}
	return &filestore{
		mutex:         &sync.RWMutex{},
		inMemoryStore: inmemory,
		codec:         codec,
		dir:           dir,
	}, nil
}

func loadFromFiles(dir string, obj runtime.Object, codec runtime.Codec) (*inMemoryStore, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		err = os.MkdirAll(dir, 0750)
		if err != nil {
			return nil, err
		}
	}
	inmemory := InMemory()
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".yaml") {
			filePath := filepath.Join(dir, file.Name())
			b, err := ioutil.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			res := obj.DeepCopyObject()
			_, _, err = codec.Decode(b, nil, res)
			if err != nil {
				return nil, err
			}
			fileName := filepath.Base(file.Name())
			err = inmemory.Create(fileName[:len(fileName)-5], res.(resource.Resource))
			if err != nil {
				return nil, err
			}
		}
	}
	return inmemory, nil
}

func (s *filestore) Create(key string, res resource.Resource) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err := s.inMemoryStore.Get(key, res); err == nil {
		return errors.NewAlreadyExists(res.GetGroupVersionResource().GroupResource(), key)
	}
	s.setNameAndCreationTimestamp(res, key)
	s.setResourceVersion(res)
	err := s.writeFile(key, res)
	if err != nil {
		return fmt.Errorf("create resource: %w", err)
	}
	s.setGVK(res)
	s.items[key] = res
	s.emit(pubsub.Added, res)
	return nil
}

func (s *filestore) Delete(key string, res resource.Resource, validate func() error) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.inMemoryStore.Delete(key, res, func() error {
		err := validate()
		if err != nil {
			return err
		}
		err = os.Remove(filepath.Join(s.dir, fmt.Sprintf("%s.yaml", key)))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("delete resource: %w", err)
		}
		return nil
	})
}

func (s *filestore) Update(key string, res resource.Resource, modify func() (resource.Resource, error)) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	existing := s.items[key]
	if existing == nil {
		return errors.NewNotFound(res.GetGroupVersionResource().GroupResource(), key)
	}
	if !specEqual(res, existing) {
		err := s.writeFile(key, res)
		if err != nil {
			return fmt.Errorf("update resource: %w", err)
		}
	}
	return s.inMemoryStore.Update(key, res, modify)
}

func specEqual(a, b runtime.Object) bool {
	ac, aOk := a.DeepCopyObject().(resource.ResourceWithStatus)
	if aOk {
		as := ac.GetStatus()
		bc, bOk := b.DeepCopyObject().(resource.ResourceWithStatus)
		if bOk {
			bs := bc.GetStatus()
			clear(as)
			clear(bs)
			return equality.Semantic.DeepEqual(ac, bc)
		}
	}
	return false
}

func clear(v interface{}) {
	p := reflect.ValueOf(v).Elem()
	p.Set(reflect.Zero(p.Type()))
}

func (s *filestore) writeFile(key string, obj resource.Resource) error {
	obj = obj.DeepCopyObject().(resource.Resource)
	objs, ok := obj.(resource.ResourceWithStatus)
	if ok {
		clear(objs.GetStatus())
	}
	m, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	rv := m.GetResourceVersion()
	m.SetResourceVersion("")
	defer m.SetResourceVersion(rv)
	f, err := ioutil.TempFile(s.dir, ".tmp-")
	if err != nil {
		return err
	}
	err = s.codec.Encode(obj, f)
	if err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	dstFile := filepath.Join(s.dir, fmt.Sprintf("%s.yaml", key))
	return os.Rename(f.Name(), dstFile)
}
