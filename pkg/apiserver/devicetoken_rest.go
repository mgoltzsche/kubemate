package apiserver

import (
	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
)

type deviceTokenREST struct {
	*REST
}

func NewDeviceTokenREST(dir string) (*deviceTokenREST, error) {
	r := NewREST(&deviceapi.DeviceToken{}, storage.InMemory())
	return &deviceTokenREST{REST: r}, nil
}
