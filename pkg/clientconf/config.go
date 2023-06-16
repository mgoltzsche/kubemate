package clientconf

import (
	"path/filepath"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func New(k3sDir string, m deviceapi.DeviceMode) (*rest.Config, error) {
	kubeconfigPath := filepath.Join(k3sDir, "server", "cred", "admin.kubeconfig")
	if m == deviceapi.DeviceModeAgent {
		kubeconfigPath = filepath.Join(k3sDir, "agent", "kubelet.kubeconfig")
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
}
