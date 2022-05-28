package apiserver

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/union"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/options"
	clientgoinformers "k8s.io/client-go/informers"
	clientgoclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/token/tokenfile"
	//"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	//"k8s.io/apiserver/pkg/authentication/user"
)

type ServerOptions struct {
	DeviceName   string
	HTTPSAddress string
	HTTPSPort    int
	WebDir       string
	ConfigDir    string
	Docker       bool
}

func NewServerOptions() ServerOptions {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("cannot derive device name from hostname: %s", err)
	}
	return ServerOptions{
		DeviceName:   hostname,
		HTTPSAddress: "0.0.0.0",
		HTTPSPort:    8443,
		WebDir:       "/var/lib/kubemate/web",
	}
}

func NewServer(o ServerOptions) (*genericapiserver.GenericAPIServer, error) {
	if o.DeviceName == "" {
		return nil, fmt.Errorf("no device name specified")
	}
	scheme := runtime.NewScheme()
	metav1.AddToGroupVersion(scheme, deviceapi.GroupVersion)
	scheme.AddKnownTypes(deviceapi.GroupVersion,
		&deviceapi.Device{}, &deviceapi.DeviceList{},
		&deviceapi.DeviceToken{}, &deviceapi.DeviceTokenList{},
	)
	codecs := serializer.NewCodecFactory(scheme)
	paramScheme := runtime.NewScheme()
	paramCodecs := runtime.NewParameterCodec(paramScheme)
	serverConfig := genericapiserver.NewRecommendedConfig(codecs)
	tlsOpts := options.NewSecureServingOptions()
	tlsOpts.BindAddress = net.ParseIP(o.HTTPSAddress)
	tlsOpts.BindPort = o.HTTPSPort
	ips := []net.IP{net.ParseIP("127.0.0.1")}
	err := tlsOpts.MaybeDefaultWithSelfSignedCerts(serverConfig.ExternalAddress, nil, ips)
	if err != nil {
		return nil, err
	}
	err = tlsOpts.ApplyTo(&serverConfig.SecureServing)
	if err != nil {
		return nil, err
	}
	serverConfig.LoopbackClientConfig = &restclient.Config{
		Host: serverConfig.ExternalAddress,
	}
	serverConfig.LongRunningFunc = filters.BasicLongRunningRequestCheck(
		sets.NewString("watch", "proxy"),
		sets.NewString("attach", "exec", "proxy", "log", "portforward"),
	)
	clientgoExternalClient, err := clientgoclientset.NewForConfig(serverConfig.LoopbackClientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create real external clientset: %w", err)
	}
	versionedInformer := clientgoinformers.NewSharedInformerFactory(clientgoExternalClient, 10*time.Minute)
	serverConfig.SharedInformerFactory = versionedInformer
	audiences := []string{adminGroup, "ui"}
	serverConfig.Authentication.APIAudiences = audiences
	tokens, err := tokenfile.NewCSV("/etc/kubemate/tokens")
	if err != nil {
		return nil, err
	}
	/*defaultTokens := map[string]*user.DefaultInfo{
		"secret": &user.DefaultInfo{
			Name:   adminUser,
			UID:    adminUser,
			Groups: []string{adminGroup},
			Extra:  map[string][]string{},
		},
	}*/
	serverConfig.Authentication.Authenticator = union.New(
		//authenticatorfactory.NewFromTokens(defaultTokens, audiences),
		bearertoken.New(tokens),
		anonymous.NewAuthenticator(),
	)
	serverConfig.Authorization.Authorizer = NewDeviceAuthorizer()
	delegate := newReverseProxy("127.0.0.1:6443")
	genericServer, err := serverConfig.Complete().New("kubemate", delegate)
	if err != nil {
		return nil, err
	}
	apiPaths := []string{"/api", "/apis", "/readyz", "/healthz", "/livez", "/metrics", "/openapi", "/.well-known"}
	genericServer.Handler.FullHandlerChain = NewWebUIHandler(o.WebDir, genericServer.Handler.FullHandlerChain, apiPaths)
	deviceREST := NewDeviceREST(o.DeviceName)
	deviceTokenREST, err := NewDeviceTokenREST(o.ConfigDir)
	if err != nil {
		return nil, err
	}
	apiGroup := &genericapiserver.APIGroupInfo{
		PrioritizedVersions:  scheme.PrioritizedVersionsForGroup(deviceapi.GroupVersion.Group),
		Scheme:               scheme,
		ParameterCodec:       paramCodecs,
		NegotiatedSerializer: codecs,
		VersionedResourcesStorageMap: map[string]map[string]registryrest.Storage{
			"v1": map[string]registryrest.Storage{
				"devices":      deviceREST,
				"devicetokens": deviceTokenREST,
			},
		},
	}
	err = genericServer.InstallAPIGroup(apiGroup)
	if err != nil {
		return nil, fmt.Errorf("install apigroup: %w", err)
	}
	installK3sRunner(genericServer, deviceREST.rest.Store, o.DeviceName, o.Docker)
	return genericServer, nil
}

// TODO: support joining nodes to a cluster via the UI. for custom (dynamic) CORS filter, see https://github.com/kubernetes/apiserver/blob/master/pkg/server/filters/cors.go

func installK3sRunner(genericServer *genericapiserver.GenericAPIServer, devices storage.Interface, deviceName string, docker bool) {
	daemon := runner.NewRunner()
	genericServer.AddPostStartHookOrDie("kubemate", func(ctx genericapiserver.PostStartHookContext) error {
		// Update device resource's status
		ch := daemon.Start()
		go func() {
			for cmd := range ch {
				logrus.Printf("k3s %s: %s", cmd.Status.State, cmd.Status.Message)
				device := &deviceapi.Device{}
				err := devices.Update(deviceName, device, func() (resource.Resource, error) {
					// TODO: map properly
					device.Status.State = deviceapi.DeviceState(cmd.Status.State)
					device.Status.Message = cmd.Status.Message
					return device, nil
				})
				if err != nil {
					logrus.WithError(err).Error("failed to update device status")
					continue
				}
			}
		}()
		// Listen for device spec changes
		d := deviceapi.Device{}
		_ = devices.Get(d.Name, &d)
		daemon.SetCommand(runner.CommandSpec{
			Command: "/proc/self/exe",
			Args:    buildK3sArgs(&d.Spec, docker),
		})
		w, err := devices.Watch(context.Background(), "")
		if err != nil {
			return err
		}
		defer w.Stop()
		deviceCh := w.ResultChan()
		go func() {
			for evt := range deviceCh {
				if evt.Type == watch.Modified {
					d := evt.Object.(*deviceapi.Device)
					if d.Name != deviceName {
						continue
					}
					_ = devices.Get(d.Name, d)
					daemon.SetCommand(runner.CommandSpec{
						Command: "/proc/self/exe",
						Args:    buildK3sArgs(&d.Spec, docker),
					})
				}
			}
		}()
		return nil
	})
	genericServer.AddPreShutdownHookOrDie("kubemate", func() error {
		return daemon.Close()
	})
}

func buildK3sArgs(spec *deviceapi.DeviceSpec, docker bool) []string {
	args := []string{
		"server",
		"--disable-cloud-controller",
		"--disable-helm-controller",
		"--no-deploy=servicelb,traefik,metrics-server",
		fmt.Sprintf("--kube-apiserver-arg=--token-auth-file=%s", "/etc/kubemate/tokens"),
	}
	if docker {
		args = append(args, "--docker")
	}
	return args
}
