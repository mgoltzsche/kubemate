package storage

import (
	"context"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type Interface interface {
	Watch(context.Context) watch.Interface
	List(l runtime.Object) error
	Get(key string, o resource.Resource) error
	Create(key string, o resource.Resource) error
	Delete(key string) error
	Update(key string, res resource.Resource, modify func() (resource.Resource, error)) error
}
