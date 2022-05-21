package apiserver

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Resource interface {
	New() runtime.Object
	NewList() runtime.Object
	GetGroupVersionResource() schema.GroupVersionResource
}
