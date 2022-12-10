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
	"github.com/mgoltzsche/kubemate/pkg/rest"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/tokengen"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
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

type ServerOptions struct {
	DeviceName      string
	HTTPSAddress    string
	HTTPSPort       int
	AdvertiseIfaces []string
	WebDir          string
	ManifestDir     string
	DataDir         string
	KubeletArgs     []string
	Docker          bool
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
		WebDir:       "/usr/share/kubemate/web",
		ManifestDir:  "/usr/share/kubemate/manifests",
		DataDir:      "/var/lib/kubemate",
	}
}

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
	metav1.AddToGroupVersion(scheme, deviceapi.GroupVersion)
	scheme.AddKnownTypes(deviceapi.GroupVersion,
		&deviceapi.Device{}, &deviceapi.DeviceList{},
		&deviceapi.DeviceToken{}, &deviceapi.DeviceTokenList{},
		&deviceapi.WifiNetwork{}, &deviceapi.WifiNetworkList{},
		&deviceapi.WifiPassword{}, &deviceapi.WifiPasswordList{},
	)
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
	serverConfig.LoopbackClientConfig = &restclient.Config{
		Host: serverConfig.ExternalAddress,
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
		generatedToken := map[string]*user.DefaultInfo{
			token: &user.DefaultInfo{
				Name:   "admin",
				UID:    "admin",
				Groups: []string{adminGroup},
				Extra:  map[string][]string{},
			},
		}
		authz = authenticatorfactory.NewFromTokens(generatedToken, audiences)
	} else {
		authz = bearertoken.New(tokens)
	}
	serverConfig.Authentication.Authenticator = union.New(
		authz,
		anonymous.NewAuthenticator(),
	)
	serverConfig.Authorization.Authorizer = NewDeviceAuthorizer()
	k3sDataDir := filepath.Join(o.DataDir, "k3s")
	delegate := newReverseProxy("127.0.0.1:6443", filepath.Join(k3sDataDir, "server", "tls"))
	genericServer, err := serverConfig.Complete().New("kubemate", delegate)
	if err != nil {
		return nil, err
	}
	discovery := discovery.NewDeviceDiscovery(o.DeviceName, o.HTTPSPort, o.AdvertiseIfaces)
	deviceConfigDir := filepath.Join(o.DataDir, "deviceconfig")
	deviceREST, err := rest.NewDeviceREST(o.DeviceName, deviceConfigDir, scheme, discovery.Discover)
	if err != nil {
		return nil, err
	}
	joinTokenDir := filepath.Join(o.DataDir, "devicetokens")
	deviceTokenREST, err := rest.NewDeviceTokenREST(joinTokenDir, scheme, o.DeviceName)
	if err != nil {
		return nil, err
	}
	logger := logrus.NewEntry(logrus.StandardLogger())
	wifi := wifi.New(logger)
	wifi.DHCPLeaseFile = filepath.Join(o.DataDir, "dhcpd.leases")
	wifiPasswordDir := filepath.Join(o.DataDir, "wifipasswords")
	wifiPasswordREST, err := rest.NewWifiPasswordREST(wifiPasswordDir, scheme)
	if err != nil {
		return nil, err
	}
	// TODO: scan for devices only when requested via rest api.
	installDeviceDiscovery(genericServer, discovery, deviceREST.Store())
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
				"devices":       deviceREST,
				"devicetokens":  deviceTokenREST,
				"wifipasswords": wifiPasswordREST,
				"wifinetworks":  rest.NewWifiNetworkREST(wifi),
			},
		},
	}
	err = genericServer.InstallAPIGroup(apiGroup)
	if err != nil {
		return nil, fmt.Errorf("install apigroup: %w", err)
	}
	controller.InstallDeviceController(genericServer, o.DeviceName, deviceREST.Store(), deviceTokenREST.Store(), discovery, wifi, wifiPasswordREST.Store(), k3sDataDir, o.ManifestDir, o.Docker, o.KubeletArgs, ingressRouter)
	return genericServer, nil
}

func installDeviceDiscovery(genericServer *genericapiserver.GenericAPIServer, discovery *discovery.DeviceDiscovery, devices storage.Interface) {
	genericServer.AddPostStartHookOrDie("device-discovery", func(ctx genericapiserver.PostStartHookContext) error {
		return discovery.Discover(devices)
	})
	genericServer.AddPreShutdownHookOrDie("device-discovery", discovery.Close)
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
