package apiserver

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	generatedopenapi "github.com/mgoltzsche/kubemate/pkg/generated/openapi"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/union"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/options"
	clientgoinformers "k8s.io/client-go/informers"
	clientgoclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/authentication/request/bearertoken"
	"k8s.io/apiserver/pkg/authentication/token/tokenfile"
	"k8s.io/apiserver/pkg/authentication/user"
)

type ServerOptions struct {
	DeviceName   string
	HTTPSAddress string
	HTTPSPort    int
	WebDir       string
	ManifestDir  string
	DataDir      string
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
		WebDir:       "/usr/share/kubemate/web",
		ManifestDir:  "/usr/share/kubemate/manifests",
		DataDir:      "/var/lib/kubemate",
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
	tlsOpts.ServerCert.CertDirectory = filepath.Join(o.DataDir, "certificates")
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
		token, err := generateRandomString(8)
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
	ips, err = publicIPs()
	if err != nil {
		return nil, err
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
	discovery := NewDeviceDiscovery(o.DeviceName, o.HTTPSPort)
	deviceREST := NewDeviceREST(o.DeviceName, discovery.Discover)
	deviceTokenREST, err := NewDeviceTokenREST(filepath.Join(o.DataDir, "devicetokens"), scheme, o.DeviceName)
	if err != nil {
		return nil, err
	}
	installDeviceDiscovery(genericServer, discovery, deviceREST.rest.Store)
	apiPaths := []string{"/api", "/apis", "/readyz", "/healthz", "/livez", "/metrics", "/openapi", "/.well-known", "/version"}
	var handler http.Handler = NewWebUIHandler(o.WebDir, genericServer.Handler.FullHandlerChain, apiPaths)
	/*handler = &corsHandler{
		Config: func() (c CorsConfig) {
			var d deviceapi.Device
			_ = deviceREST.rest.Store.Get(o.DeviceName, &d)
			if d.Spec.AllowOrigin {
				c.AllowedOrigins = append(c.AllowedOrigins, d.Name, d.Status.Address)
			}
			if d.Spec.Server != "" {
				c.AllowedOrigins = []string{d.Spec.Server}
				_ = deviceREST.rest.Store.Get(d.Name, &d)
				if d.Status.Address != "" {
					c.AllowedOrigins = append(c.AllowedOrigins, d.Status.Address)
				}
			}
			fmt.Printf("## %+v\n", c.AllowedOrigins)
			return
		},
		Delegate: handler,
	}*/
	genericServer.Handler.FullHandlerChain = handler
	genericServer.ExternalAddress = fmt.Sprintf("%s:%d", ips[0], o.HTTPSPort)
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
	installDeviceController(genericServer, deviceREST.rest.Store, deviceTokenREST.Store, o.DeviceName, discovery, k3sDataDir, o.ManifestDir, o.Docker)
	/*if o.Docker {
		// TODO Move into server.Prepare method
		// TODO: start cri-dockerd binary
		go func() {
			err := cridocker.RunCriDockerd(&cridockerdopts.DockerCRIFlags{}, context.Background())
			if err != nil {
				logrus.Error(fmt.Errorf("cri-dockerd: %w", err))
			}
		}()
	}*/
	return genericServer, nil
}

func installDeviceDiscovery(genericServer *genericapiserver.GenericAPIServer, discovery *DeviceDiscovery, devices storage.Interface) {
	genericServer.AddPostStartHookOrDie("device-discovery", func(ctx genericapiserver.PostStartHookContext) error {
		return discovery.Discover(devices)
	})
	genericServer.AddPreShutdownHookOrDie("device-discovery", discovery.Close)
}
