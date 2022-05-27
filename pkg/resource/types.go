package resource

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Resource interface {
	runtime.Object
	New() Resource
	NewList() runtime.Object
	GetGroupVersionResource() schema.GroupVersionResource
	GetStatus() SubResource
	DeepCopyIntoResource(Resource) error
	GetResourceVersion() string
	SetResourceVersion(string)
}

type SubResource interface{}
