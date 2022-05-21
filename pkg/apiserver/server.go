package apiserver

import (
	"fmt"
	"time"

	deviceapi "github.com/mgoltzsche/k3spi/pkg/apis/devices/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/user"
	registryrest "k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/filters"
	clientgoinformers "k8s.io/client-go/informers"
	clientgoclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	//genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	//"k8s.io/apiserver/pkg/util/notfoundhandler"
)

type ServerOptions struct {
	Address string
	WebDir  string
}

func NewServerOptions() ServerOptions {
	return ServerOptions{
		Address: "127.0.0.1:8080",
		WebDir:  "./web",
	}
}

func NewServer(o ServerOptions) (*genericapiserver.GenericAPIServer, error) {
	scheme := runtime.NewScheme()
	metav1.AddToGroupVersion(scheme, deviceapi.GroupVersion)
	scheme.AddKnownTypes(deviceapi.GroupVersion, &deviceapi.Device{}, &deviceapi.DeviceList{})
	codecs := serializer.NewCodecFactory(scheme)
	paramScheme := runtime.NewScheme()
	paramCodecs := runtime.NewParameterCodec(paramScheme)
	serverConfig := genericapiserver.NewRecommendedConfig(codecs)
	serverConfig.ExternalAddress = o.Address
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
	tokens := map[string]*user.DefaultInfo{
		"secret": &user.DefaultInfo{
			Name:   adminUser,
			UID:    adminUser,
			Groups: []string{adminUser},
			Extra:  map[string][]string{},
		},
	}
	audiences := []string{adminUser, "ui"}
	serverConfig.Authentication.APIAudiences = audiences
	serverConfig.Authentication.Authenticator = union.New(
		authenticatorfactory.NewFromTokens(tokens, audiences),
		anonymous.NewAuthenticator(),
	)
	serverConfig.Authorization.Authorizer = NewDeviceAuthorizer()
	delegate := newReverseProxy("127.0.0.1:6443")
	genericServer, err := serverConfig.Complete().New("k3s-connect", delegate)
	if err != nil {
		return nil, err
	}
	apiPaths := []string{"/api", "/apis", "/readyz", "/healthz", "/livez", "/metrics", "/openapi", "/.well-known"}
	genericServer.Handler.FullHandlerChain = NewWebUIHandler(o.WebDir, genericServer.Handler.FullHandlerChain, apiPaths)
	apiGroup := &genericapiserver.APIGroupInfo{
		PrioritizedVersions:  scheme.PrioritizedVersionsForGroup(deviceapi.GroupVersion.Group),
		Scheme:               scheme,
		ParameterCodec:       paramCodecs,
		NegotiatedSerializer: codecs,
		VersionedResourcesStorageMap: map[string]map[string]registryrest.Storage{
			"v1": map[string]registryrest.Storage{
				"devices": NewDeviceREST("thisdevice"),
			},
		},
	}
	err = genericServer.InstallAPIGroup(apiGroup)
	if err != nil {
		return nil, fmt.Errorf("install apigroup: %w", err)
	}
	return genericServer, nil
}
