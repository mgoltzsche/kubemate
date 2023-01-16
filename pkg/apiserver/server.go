package apiserver

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/controller"
	"github.com/mgoltzsche/kubemate/pkg/discovery"
	generatedopenapi "github.com/mgoltzsche/kubemate/pkg/generated/openapi"
	"github.com/mgoltzsche/kubemate/pkg/ingress"
	"github.com/mgoltzsche/kubemate/pkg/networkifaces"
	devicectrl "github.com/mgoltzsche/kubemate/pkg/reconciler/device"
	"github.com/mgoltzsche/kubemate/pkg/rest"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/tokengen"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/token/tokenfile"
	"k8s.io/apiserver/pkg/authentication/user"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/options"
	clientgoinformers "k8s.io/client-go/informers"
	clientgoclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

// ServerOptions defines the configuration options for the server.
type ServerOptions struct {
	DeviceName      string
	HTTPSAddress    string
	HTTPSPort       int
	HTTPPort        int
	AdvertiseIfaces []string
	WebDir          string
	ManifestDir     string
	DataDir         string
	KubeletArgs     []string
	Docker          bool
}

// NewServerOptions creates server options with defaults.
func NewServerOptions() ServerOptions {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("cannot derive device name from hostname: %s", err)
	}
	return ServerOptions{
		DeviceName:   hostname,
		HTTPSAddress: "0.0.0.0",
		HTTPSPort:    8443,
		WebDir:       "/usr/share/kubemate/web",
		ManifestDir:  "/usr/share/kubemate/manifests",
		DataDir:      "/var/lib/kubemate",
	}
}

// NewServer creates a new server.
func NewServer(o ServerOptions) (*genericapiserver.GenericAPIServer, error) {
	if o.DeviceName == "" {
		return nil, fmt.Errorf("no device name specified")
	}
	if len(o.AdvertiseIfaces) == 0 {
		ifaces, err := detectIfaces()
		if err != nil {
			return nil, fmt.Errorf("detect default network interfaces: %w", err)
		}
		o.AdvertiseIfaces = ifaces
		if len(ifaces) == 0 {
			logrus.Warn("could not detect default advertise network interfaces - advertising on all interfaces")
		}
	}
	scheme := runtime.NewScheme()
	deviceapi.AddToScheme(scheme)
	codecs := serializer.NewCodecFactory(scheme)
	paramScheme := runtime.NewScheme()
	paramCodecs := runtime.NewParameterCodec(paramScheme)
	serverConfig := genericapiserver.NewRecommendedConfig(codecs)
	tlsOpts := options.NewSecureServingOptions()
	tlsOpts.BindAddress = net.ParseIP(o.HTTPSAddress)
	tlsOpts.BindPort = o.HTTPSPort
	tlsOpts.ServerCert.CertDirectory = filepath.Join(o.DataDir, "certificates")
	if o.HTTPSPort == 443 {
		serverConfig.ExternalAddress = fmt.Sprintf("%s", o.DeviceName)
	} else {
		serverConfig.ExternalAddress = fmt.Sprintf("%s:%d", o.DeviceName, o.HTTPSPort)
	}
	tlsCertIPs := []net.IP{net.ParseIP("127.0.0.1")}
	// TODO: use hostname as external address
	err := tlsOpts.MaybeDefaultWithSelfSignedCerts(serverConfig.ExternalAddress, nil, tlsCertIPs)
	if err != nil {
		return nil, err
	}
	err = tlsOpts.ApplyTo(&serverConfig.SecureServing)
	if err != nil {
		return nil, err
	}
	var localAddr string
	if o.HTTPPort == 80 {
		localAddr = "http://127.0.0.1"
	} else {
		localAddr = fmt.Sprintf("http://127.0.0.1:%d", o.HTTPPort)
	}
	serverConfig.LoopbackClientConfig = &restclient.Config{
		Host: localAddr,
	}
	serverConfig.LongRunningFunc = filters.BasicLongRunningRequestCheck(
		sets.NewString("watch", "proxy"),
		sets.NewString("attach", "exec", "proxy", "log", "portforward"),
	)
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(scheme))
	serverConfig.OpenAPIConfig.Info.Title = "kubemate"
	clientgoExternalClient, err := clientgoclientset.NewForConfig(serverConfig.LoopbackClientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create real external clientset: %w", err)
	}
	versionedInformer := clientgoinformers.NewSharedInformerFactory(clientgoExternalClient, 10*time.Minute)
	serverConfig.SharedInformerFactory = versionedInformer
	audiences := []string{adminGroup, "ui"}
	serverConfig.Authentication.APIAudiences = audiences
	accountsFile := "/etc/kubemate/tokens"
	var authz authenticator.Request
	tokens, err := tokenfile.NewCSV(accountsFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		logrus.Warnf("Accounts file not found at %s - generating token...", accountsFile)
		token, err := tokengen.GenerateRandomString(8)
		if err != nil {
			return nil, fmt.Errorf("generate admin token: %w", err)
		}
		logrus.Infof("Generated token: %s", token)
		tokens = tokenfile.New(map[string]*user.DefaultInfo{
			token: &user.DefaultInfo{
				Name:   "admin",
				UID:    "admin",
				Groups: []string{adminGroup},
				Extra:  map[string][]string{},
			},
		})
	}
	authz = bearertoken.New(tokens)
	controllerToken, err := tokengen.GenerateRandomString(16)
	if err != nil {
		return nil, err
	}
	ctrlAuthz := bearertoken.New(tokenfile.New(map[string]*user.DefaultInfo{
		controllerToken: &user.DefaultInfo{
			Name:   "controller",
			UID:    "controller",
			Groups: []string{adminGroup},
			Extra:  map[string][]string{},
		},
	}))
	serverConfig.Authentication.Authenticator = union.New(
		authz,
		ctrlAuthz,
		anonymous.NewAuthenticator(),
	)
	serverConfig.Authorization.Authorizer = NewDeviceAuthorizer()
	k3sDataDir := filepath.Join(o.DataDir, "k3s")
	k3sProxyEnabled := false
	delegate := newReverseProxy("127.0.0.1:6443", filepath.Join(k3sDataDir, "server", "tls"), &k3sProxyEnabled)
	genericServer, err := serverConfig.Complete().New("kubemate", delegate)
	if err != nil {
		return nil, err
	}
	logger := logrus.NewEntry(logrus.StandardLogger())
	netConfigDir := filepath.Join(o.DataDir, "netconfig")
	ifaceStore, err := storage.FileStore(netConfigDir, &deviceapi.NetworkInterface{}, scheme)
	if err != nil {
		return nil, err
	}
	ifaceREST := rest.NewNetworkInterfaceREST(ifaceStore)
	discoveryStore := storage.InMemory(scheme)
	discovery := discovery.NewDeviceDiscovery(o.DeviceName, o.HTTPSPort, o.AdvertiseIfaces, discoveryStore, logger)
	discoveryREST := rest.NewDeviceDiscoveryREST(discovery.Store())
	deviceConfigDir := filepath.Join(o.DataDir, "deviceconfig")
	deviceREST, err := rest.NewDeviceREST(o.DeviceName, deviceConfigDir, scheme)
	if err != nil {
		return nil, err
	}
	joinTokenDir := filepath.Join(o.DataDir, "devicetokens")
	deviceTokenREST, err := rest.NewDeviceTokenREST(joinTokenDir, scheme, o.DeviceName)
	if err != nil {
		return nil, err
	}
	wifi := wifi.New(logger, o.DataDir, func(cmd runner.Command) {
		time.Sleep(time.Second)
		l := deviceapi.NetworkInterfaceList{}
		err := ifaceStore.List(&l)
		if err != nil {
			logger.WithError(err).Error("cannot trigger network interface reconciliation")
			return
		}
		for _, iface := range l.Items {
			if iface.Status.Link.Type == deviceapi.NetworkInterfaceTypeWifi {
				logger.WithField("iface", iface.Name).Debug("triggering NetworkInterface reconciliation")
				err = ifaceStore.Update(iface.Name, &iface, func() error { return nil })
				if err != nil {
					logger.WithError(err).Error("cannot trigger network interface reconciliation")
				}
			}
		}
	})
	wifiNetworkREST := rest.NewWifiNetworkREST(wifi, scheme)
	wifiPasswordDir := filepath.Join(o.DataDir, "wifipasswords")
	wifiPasswordREST, err := rest.NewWifiPasswordREST(wifiPasswordDir, scheme)
	if err != nil {
		return nil, err
	}
	installDeviceDiscovery(genericServer, discovery)
	ingressRouter := ingress.NewIngressController("kubemate", logrus.WithField("comp", "ingress-controller"))
	apiPaths := []string{"/api", "/apis", "/readyz", "/healthz", "/livez", "/metrics", "/openapi", "/.well-known", "/version"}
	var handler http.Handler = NewWebUIHandler(o.WebDir, apiPaths, genericServer.Handler.FullHandlerChain, ingressRouter)
	genericServer.Handler.FullHandlerChain = handler
	apiGroup := &genericapiserver.APIGroupInfo{
		PrioritizedVersions:  scheme.PrioritizedVersionsForGroup(deviceapi.GroupVersion.Group),
		Scheme:               scheme,
		ParameterCodec:       paramCodecs,
		NegotiatedSerializer: codecs,
		VersionedResourcesStorageMap: map[string]map[string]registryrest.Storage{
			"v1": map[string]registryrest.Storage{
				"networkinterfaces": ifaceREST,
				"devices":           deviceREST,
				"devicediscovery":   discoveryREST,
				"devicetokens":      deviceTokenREST,
				"wifipasswords":     wifiPasswordREST,
				"wifinetworks":      wifiNetworkREST,
			},
		},
	}
	err = genericServer.InstallAPIGroup(apiGroup)
	if err != nil {
		return nil, fmt.Errorf("install apigroup: %w", err)
	}
	installDeviceControllers(genericServer, logger,
		&devicectrl.NetworkInterfaceReconciler{
			DeviceName:        o.DeviceName,
			NetworkInterfaces: o.AdvertiseIfaces,
			Store:             ifaceStore,
			WifiNetworks:      wifiNetworkREST.Store(),
			WifiPasswords:     wifiPasswordREST.Store(),
			Wifi:              wifi,
		},
		&devicectrl.DeviceReconciler{
			DeviceName:        o.DeviceName,
			DeviceAddress:     genericServer.ExternalAddress,
			DeviceDiscovery:   discovery,
			DataDir:           k3sDataDir,
			ManifestDir:       o.ManifestDir,
			ExternalPort:      o.HTTPSPort,
			Docker:            o.Docker,
			KubeletArgs:       o.KubeletArgs,
			Devices:           deviceREST.Store(),
			DeviceTokens:      deviceTokenREST.Store(),
			NetworkInterfaces: ifaceStore,
			IngressController: ingressRouter,
			K3sProxyEnabled:   &k3sProxyEnabled,
			Logger:            logger,
		})
	return genericServer, nil
}

func installNetworkInterfaceSync(genericServer *genericapiserver.GenericAPIServer, sync *networkifaces.NetworkIfaceSync) {
	genericServer.AddPostStartHookOrDie("networkiface-sync", func(ctx genericapiserver.PostStartHookContext) error {
		return sync.Start()
	})
	genericServer.AddPreShutdownHookOrDie("networkiface-sync", sync.Stop)
}

func installDeviceDiscovery(genericServer *genericapiserver.GenericAPIServer, discovery *discovery.DeviceDiscovery) {
	genericServer.AddPostStartHookOrDie("device-discovery", func(ctx genericapiserver.PostStartHookContext) error {
		err := discovery.Discover()
		if err != nil {
			logrus.WithError(err).Error("device discovery post start hook failed")
		}
		return nil
	})
	genericServer.AddPreShutdownHookOrDie("device-discovery", discovery.Close)
}

func installDeviceControllers(genericServer *genericapiserver.GenericAPIServer, logger *logrus.Entry, rl ...controller.Reconciler) {
	var config *restclient.Config
	configFn := func() (*restclient.Config, error) {
		return config, nil
	}
	mgr := controller.NewControllerManager(configFn, logger.WithField("comp", "device-manager"))
	for _, r := range rl {
		mgr.RegisterReconciler(r)
	}
	genericServer.AddPostStartHookOrDie("device-controller", func(ctx genericapiserver.PostStartHookContext) error {
		// TODO: clean this up: set config as Start() argument?!
		//config = ctx.LoopbackClientConfig
		config = &restclient.Config{
			Host:        genericServer.LoopbackClientConfig.Host,
			BearerToken: "adminsecret", // TODO: Derive token. Generate separate machine account ideally.
		}
		return mgr.Start()
	})
	genericServer.AddPreShutdownHookOrDie("device-controller", func() error {
		mgr.Stop()
		return nil
	})
}

func detectIfaces() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, 2)
	for _, iface := range ifaces {
		name := iface.Name
		if strings.HasPrefix(name, "enp") || strings.HasPrefix(name, "wlp") || strings.HasPrefix(name, "eth") || strings.HasPrefix(name, "wlan") {
			names = append(names, name)
		}
	}
	return names, nil
}
