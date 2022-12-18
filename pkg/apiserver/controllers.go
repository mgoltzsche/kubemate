package apiserver

import (
	"github.com/mgoltzsche/kubemate/pkg/controller"
	"github.com/mgoltzsche/kubemate/pkg/discovery"
	"github.com/mgoltzsche/kubemate/pkg/ingress"
	"github.com/mgoltzsche/kubemate/pkg/reconciler/device"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"github.com/sirupsen/logrus"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/rest"
)

func installDeviceController(genericServer *genericapiserver.GenericAPIServer, deviceName string, devices, clusterTokens storage.Interface, discovery *discovery.DeviceDiscovery, wifi *wifi.Wifi, wifiPasswords storage.Interface, dataDir, manifestDir string, docker bool, kubeletArgs []string, ingressCtrl *ingress.IngressController, logger *logrus.Entry) {
	var config *rest.Config
	configFn := func() (*rest.Config, error) {
		return config, nil
	}
	mgr := controller.NewControllerManager(configFn, logger.WithField("comp", "device-manager"))
	mgr.RegisterReconciler(&device.DeviceReconciler{
		DeviceName:        deviceName,
		DeviceAddress:     genericServer.ExternalAddress,
		DeviceDiscovery:   discovery,
		DataDir:           dataDir,
		ManifestDir:       manifestDir,
		Docker:            docker,
		KubeletArgs:       kubeletArgs,
		Devices:           devices,
		DeviceTokens:      clusterTokens,
		WifiPasswords:     wifiPasswords,
		Wifi:              wifi,
		IngressController: ingressCtrl,
		Logger:            logger,
	})
	genericServer.AddPostStartHookOrDie("device-controller", func(ctx genericapiserver.PostStartHookContext) error {
		// TODO: clean this up: set config as Start() argument?!
		//config = ctx.LoopbackClientConfig
		config = &rest.Config{
			Host:        "127.0.0.1:8080",
			BearerToken: "adminsecret", // TODO: Derive token. Generate separate machine account ideally.
		}
		return mgr.Start()
	})
	genericServer.AddPreShutdownHookOrDie("device-controller", func() error {
		mgr.Stop()
		return nil
	})
}
