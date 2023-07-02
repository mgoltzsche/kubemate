package apiserver

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"github.com/mgoltzsche/kubemate/pkg/auth/authserver"
	"github.com/mgoltzsche/kubemate/pkg/auth/resourceserver"
	"github.com/mgoltzsche/kubemate/pkg/controller"
	"github.com/mgoltzsche/kubemate/pkg/discovery"
	generatedopenapi "github.com/mgoltzsche/kubemate/pkg/generated/openapi"
	"github.com/mgoltzsche/kubemate/pkg/ingress"
	"github.com/mgoltzsche/kubemate/pkg/middleware"
	"github.com/mgoltzsche/kubemate/pkg/networkifaces"
	devicectrl "github.com/mgoltzsche/kubemate/pkg/reconciler/device"
	"github.com/mgoltzsche/kubemate/pkg/rest"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/tokengen"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	DeviceName          string
	HTTPSAddress        string
	HTTPSPort           int
	HTTPAddress         string
	HTTPPort            int
	AdvertiseIfaces     []string
	WebDir              string
	ManifestDir         string
	DataDir             string
	KubeletArgs         []string
	Docker              bool
	WriteHostResolvConf bool
	Shutdown            func() error
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
		HTTPSPort:    443,
		HTTPAddress:  "0.0.0.0",
		HTTPPort:     80,
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
	for _, dir := range []string{"rancher"} {
		err := os.MkdirAll(filepath.Join(o.DataDir, dir), 0755)
		if err != nil {
			return nil, err
		}
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
	utilruntime.Must(deviceapi.AddToScheme(scheme))
	codecs := serializer.NewCodecFactory(scheme)
	paramScheme := runtime.NewScheme()
	paramCodecs := runtime.NewParameterCodec(paramScheme)
	serverConfig := genericapiserver.NewRecommendedConfig(codecs)
	tlsOpts := options.NewSecureServingOptions()
	tlsOpts.BindAddress = net.ParseIP(o.HTTPSAddress)
	tlsOpts.BindPort = o.HTTPSPort
	tlsOpts.ServerCert.CertDirectory = filepath.Join(o.DataDir, "certificates")
	var externalAddr string
	if o.HTTPSPort == 443 {
		externalAddr = fmt.Sprintf("%s", o.DeviceName)
	} else {
		externalAddr = fmt.Sprintf("%s:%d", o.DeviceName, o.HTTPSPort)
	}
	serverConfig.ExternalAddress = externalAddr
	tlsCertIPs := []net.IP{net.ParseIP("127.0.0.1")}
	// TODO: use hostname as external address
	err := tlsOpts.MaybeDefaultWithSelfSignedCerts(externalAddr, []string{o.DeviceName}, tlsCertIPs)
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
	apiProxy := newAPIServerProxy("127.0.0.1:6443", filepath.Join(k3sDataDir, "server", "tls"), &k3sProxyEnabled)
	genericServer, err := serverConfig.Complete().New("kubemate", apiProxy.DelegationTarget())
	if err != nil {
		return nil, err
	}
	logger := logrus.NewEntry(logrus.StandardLogger())
	netConfigDir := filepath.Join(o.DataDir, "netconfig")
	ifaceStore, err := storage.FileStore(netConfigDir, &deviceapi.NetworkInterface{}, scheme)
	if err != nil {
		return nil, err
	}
	caCert, _ := serverConfig.SecureServing.Cert.CurrentCertKeyContent()
	certREST := rest.NewCertificateREST(scheme, caCert)
	userAccountREST, err := rest.NewUserAccountREST(filepath.Join(o.DataDir, "useraccounts"), scheme)
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
	wifi.WriteHostResolvConf = o.WriteHostResolvConf
	wifi.DNSKeyFile = filepath.Join(o.DataDir, "k3s", "dns", "zone.key")
	wifi.CaptivePortalURL = fmt.Sprintf("https://%s", externalAddr)
	wifiNetworkREST := rest.NewWifiNetworkREST(wifi, scheme)
	wifiPasswordDir := filepath.Join(o.DataDir, "wifipasswords")
	wifiPasswordREST, err := rest.NewWifiPasswordREST(wifiPasswordDir, scheme)
	if err != nil {
		return nil, err
	}
	installDeviceDiscovery(genericServer, discovery)

	oidcIssuer := fmt.Sprintf("https://%s", serverConfig.ExternalAddress)
	/*handler, err := oidc.NewIdentityProvider(context.TODO(), oidcIssuer, genericServer.Handler.FullHandlerChain)
	if err != nil {
		return nil, fmt.Errorf("oidc identity provider: %w", err)
	}
	rsProvider, err := rs.NewProvider(issuer, keyPath)
	if err != nil {
		return nil, fmt.Errorf("oidc resource server authenticator: %w", err)
	}
	var handler http.Handler = idp*/

	httpsClient, err := resourceserver.HTTPClient(filepath.Join(o.DataDir, "certificates", "apiserver.crt"))
	if err != nil {
		return nil, fmt.Errorf("load oauth2 http client: %w", err)
	}

	callbackPath := "/oauth2/callback"
	var oauth2ClientConf = resourceserver.Config{
		Config: oauth2.Config{
			ClientID:     "my-client",
			ClientSecret: "foobar",
			Scopes:       []string{"fosite"},
			RedirectURL:  fmt.Sprintf("%s%s", oidcIssuer, callbackPath),
			Endpoint: oauth2.Endpoint{
				AuthURL:  fmt.Sprintf("%s/oauth2/auth", oidcIssuer),
				TokenURL: fmt.Sprintf("%s/oauth2/token", oidcIssuer),
			},
		},
		IDPURL:       oidcIssuer,
		CallbackPath: callbackPath,
		Client:       httpsClient,
	}

	router := http.NewServeMux()
	idp := authserver.NewIdentityProvider(oidcIssuer)
	idp.RegisterHTTPRoutes(router, logger)
	router.Handle(callbackPath, http.HandlerFunc(resourceserver.CallbackHandler(oauth2ClientConf)))

	ingressRouter := ingress.NewIngressController("kubemate", oauth2ClientConf, logrus.WithField("comp", "ingress-controller"))
	apiPaths := []string{"/api", "/apis", "/readyz", "/healthz", "/livez", "/metrics", "/openapi", "/.well-known", "/version"}
	handler := genericServer.Handler.FullHandlerChain
	handler = apiProxy.APIGroupListCompletionFilter(handler)
	handler = NewWebUIHandler(o.WebDir, apiPaths, handler, ingressRouter)
	handler = middleware.ForceHTTPS(handler)
	// TODO: don't redirect ingress hosts
	handler = middleware.ForceHTTPSHost(externalAddr, handler)
	//handler = middleware.WithAccessLog(handler, logger)
	router.Handle("/", handler)
	genericServer.Handler.FullHandlerChain = router
	apiGroup := &genericapiserver.APIGroupInfo{
		PrioritizedVersions:  scheme.PrioritizedVersionsForGroup(deviceapi.GroupVersion.Group),
		Scheme:               scheme,
		ParameterCodec:       paramCodecs,
		NegotiatedSerializer: codecs,
		VersionedResourcesStorageMap: map[string]map[string]registryrest.Storage{
			"v1alpha1": map[string]registryrest.Storage{
				"networkinterfaces": ifaceREST,
				"certificates":      certREST,
				"useraccounts":      userAccountREST,
				"devices":           deviceREST,
				"devices/shutdown":  rest.NewDeviceShutdownREST(o.DeviceName, deviceREST.Store(), k3sDataDir),
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
			DeviceAddress:     externalAddr,
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
			Shutdown:          o.Shutdown,
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
