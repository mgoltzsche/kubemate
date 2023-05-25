package ingress

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

func NewIngressController(ingressClass string, logger *logrus.Entry) *IngressController {
	return &IngressController{
		ingressClass: ingressClass,
		router:       newEmptyRouter(),
		logger:       logger,
	}
}

type IngressController struct {
	ingressClass string
	router       *router
	logger       *logrus.Entry
	mutex        sync.Mutex
	started      bool
}

func (s *IngressController) Start() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.started {
		return
	}
	s.started = true
	s.logger.Info("start watching ingress resources")
	ctx, cancel := context.WithCancel(context.Background())
	prevCancel := s.router.Cancel
	s.router.Cancel = func() {
		prevCancel()
		cancel()
	}
	go func() {
		for {
			err := s.start(ctx, cancel, prevCancel)
			if ctx.Err() != nil {
				s.logger.Debug("stopped ingress controller reconciliations")
				break
			}
			if err != nil {
				s.logger.WithError(err).Warn("failed to run ingress router")
			}
			time.Sleep(time.Second)
		}
	}()
	return
}

func (s *IngressController) start(ctx context.Context, cancel, prevCancel context.CancelFunc) (err error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("new ingress api client config: %w", err)
	}
	cl, err := newCachedClient(config)
	if err != nil {
		return fmt.Errorf("new ingress api client: %w", err)
	}
	r := &router{
		Handler:      mux.NewRouter(),
		ingressClass: s.ingressClass,
		ctx:          ctx,
		client:       cl.GetClient(),
		logger:       s.logger,
		Cancel:       cancel,
	}
	c := cl.GetCache()
	ch := make(chan struct{}, 10)
	for _, o := range []client.Object{&netv1.Ingress{}, &corev1.Service{}, &corev1.Endpoints{}} {
		var inf cache.Informer
		inf, err = c.GetInformer(ctx, o)
		if err != nil {
			return err
		}
		inf.AddEventHandler(&informer{update: func() {
			defer recover() // don't panic if channel is closed already
			ch <- struct{}{}
		}})
	}
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	go func() {
		for _ = range ch {
			time.Sleep(100 * time.Millisecond)
			r.Update()
		}
	}()
	resultCh := make(chan error)
	go func() {
		err := c.Start(ctx)
		resultCh <- err
		close(resultCh)
	}()
	prevCancel()
	c.WaitForCacheSync(ctx)
	r.Update()
	s.router = r
	return <-resultCh
}

func (s *IngressController) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.started {
		s.logger.Info("stopping watching ingress resources")
		s.router.Cancel()
		s.router = newEmptyRouter()
		s.started = false
	}
}

func (s *IngressController) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.router.ServeHTTP(w, req)
}

type router struct {
	http.Handler
	ingressClass string
	ctx          context.Context
	client       client.Client
	logger       *logrus.Entry
	Cancel       context.CancelFunc
}

func newEmptyRouter() *router {
	return &router{
		Handler: mux.NewRouter(),
		Cancel:  func() {},
	}
}

func (r *router) Update() {
	r.logger.Debug("reconciling ingress routes")
	h, err := newRouter(r.ctx, r.client, r.ingressClass, r.logger)
	if err != nil {
		r.logger.Error(err)
		return
	}
	r.Handler = h
}

func newRouter(ctx context.Context, c client.Client, ingressClass string, logger *logrus.Entry) (http.Handler, error) {
	ingresses := netv1.IngressList{}
	err := c.List(ctx, &ingresses)
	if err != nil {
		return nil, err
	}
	ingressKeys := make([]string, 0, len(ingresses.Items))
	ingressMap := map[string]netv1.Ingress{}
	for _, ing := range ingresses.Items {
		if ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName == ingressClass {
			k := fmt.Sprintf("%s/%s", ing.Namespace, ing.Name)
			ingressKeys = append(ingressKeys, k)
			ingressMap[k] = ing
		}
	}
	sort.Strings(ingressKeys)
	rootMux := mux.NewRouter()
	hosts := map[string]*mux.Router{}
	paths := make(map[string]string, len(ingressKeys))
	for _, k := range ingressKeys {
		ing := ingressMap[k]
		for _, r := range ing.Spec.Rules {
			m := rootMux
			if r.Host != "" {
				m, ok := hosts[r.Host]
				if !ok {
					m = mux.NewRouter()
					rootMux.Host(r.Host).Handler(m)
					hosts[r.Host] = m
				}
			}
			for _, p := range r.HTTP.Paths {
				if err := validateIngressBackend(&p.Backend); err != nil {
					logger.WithField("resource", k).Warn(err.Error())
					continue
				}
				path := fmt.Sprintf("%s%s", r.Host, p.Path)
				otherIngressKey, exists := paths[path]
				if exists {
					// TODO: emit kubernetes event
					return nil, fmt.Errorf("duplicate ingress endpoint %s, ingresses: %s and %s", path, k, otherIngressKey)
				}
				paths[path] = k
				rewriteTargetPath := ""
				backendProtocol := "http"
				if ing.Annotations != nil {
					rewriteTargetPath = ing.Annotations["kubemate.mgoltzsche.github.com/rewrite-target"]
					if rewriteTargetPath == "" {
						rewriteTargetPath = ing.Annotations["nginx.ingress.kubernetes.io/rewrite-target"]
					}
					p := ing.Annotations["kubemate.mgoltzsche.github.com/backend-protocol"]
					if p == "" {
						p = ing.Annotations["nginx.ingress.kubernetes.io/backend-protocol"]
					}
					if p != "" {
						backendProtocol = p
					}
				}
				backendProtocol = strings.ToLower(backendProtocol)
				backendURL, err := endpointURL(ctx, backendProtocol, p.Backend.Service, ing.Namespace, c)
				if err != nil {
					logger.WithField("resource", k).Warn(err.Error())
					continue
				}
				ph := httputil.NewSingleHostReverseProxy(backendURL)
				if backendProtocol == "https" {
					ph.Transport = &http.Transport{
						Dial: (&net.Dialer{
							Timeout:   30 * time.Second,
							KeepAlive: 30 * time.Second,
						}).Dial,
						TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
						TLSHandshakeTimeout: 10 * time.Second,
					}
				}
				h := &ingressBackendHandler{
					proxy:             ph,
					targetPath:        p.Path,
					rewriteTargetPath: rewriteTargetPath,
					ingressName:       k,
					serviceName:       p.Backend.Service.Name,
					logger:            logger,
				}
				pattern := p.Path
				if p.PathType == nil || *p.PathType == netv1.PathTypePrefix || *p.PathType == netv1.PathTypeImplementationSpecific {
					pattern = fmt.Sprintf("%s**", p.Path)
					m.PathPrefix(p.Path).Handler(h)
				} else if *p.PathType == netv1.PathTypeExact {
					m.Handle(p.Path, h)
				} else {
					logger.WithField("resource", k).Warnf("ignoring ingress resource since it specifies an unsupported path type %q", *p.PathType)
					continue
				}
				logger.
					WithField("endpoint", fmt.Sprintf("%s%s", r.Host, pattern)).
					Debug("registered ingress handler")
			}
		}
		if len(ing.Spec.TLS) > 0 {
			logger.WithField("resource", k).Warn("ignoring custom tls certificate for ingress resource")
		}
	}
	// TODO: update ingress status
	return rootMux, nil
}

func validateIngressBackend(b *netv1.IngressBackend) error {
	if b.Resource != nil {
		return fmt.Errorf("ingress resource specifies an unsupported backend resource - only service backends are supported")
	}
	if b.Service == nil {
		return fmt.Errorf("ingress resource does not specify a service")
	}
	if b.Service.Name == "" {
		return fmt.Errorf("ingress resource does not specify a backend service name")
	}
	if b.Service.Port.Name == "" && b.Service.Port.Number == 0 {
		return fmt.Errorf("ingress resource does not specify a backend service port")
	}
	return nil
}

func endpointURL(ctx context.Context, protocol string, svc *netv1.IngressServiceBackend, ns string, c client.Client) (*url.URL, error) {
	var endpoints corev1.Endpoints
	key := types.NamespacedName{
		Name:      svc.Name,
		Namespace: ns,
	}
	err := c.Get(ctx, key, &endpoints)
	if err != nil {
		return nil, err
	}
	portMatched := false
	for _, s := range endpoints.Subsets {
		port := findPort(svc.Port, s.Ports)
		if port > 0 {
			portMatched = true
			for _, a := range s.Addresses {
				if a.IP != "" {
					u, err := url.Parse(fmt.Sprintf("%s://%s:%d", protocol, a.IP, port))
					if err != nil {
						return nil, fmt.Errorf("parse backend endpoint url: %w", err)
					}
					return u, nil
				}
			}
		}
	}
	if portMatched {
		return nil, fmt.Errorf("endpoint is not ready")
	}
	return nil, fmt.Errorf("port does not match endpoint")
}

func findPort(port netv1.ServiceBackendPort, ports []corev1.EndpointPort) int32 {
	for _, p := range ports {
		if p.Port == port.Number || p.Name == port.Name {
			return p.Port
		}
	}
	return -1
}

type ingressBackendHandler struct {
	proxy             *httputil.ReverseProxy
	targetPath        string
	rewriteTargetPath string
	ingressName       string
	serviceName       string
	logger            *logrus.Entry
}

func (h *ingressBackendHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	startTime := time.Now()
	if h.rewriteTargetPath != "" {
		req.URL.Path = path.Clean(fmt.Sprintf("%s%s", h.rewriteTargetPath, req.URL.Path[len(h.targetPath):]))
	}
	w = &responseWriter{
		ResponseWriter: w,
		startTime:      startTime,
		logger: h.logger.WithField("ingress", h.ingressName).
			WithField("host", req.Host).WithField("method", req.Method).
			WithField("path", req.URL.Path),
	}
	h.proxy.ServeHTTP(w, req)
}

type responseWriter struct {
	http.ResponseWriter
	startTime time.Time
	logger    *logrus.Entry
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.logger.
		WithField("status", statusCode).
		WithField("duration", time.Since(w.startTime)).
		Trace("ingress request")
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer cannot be hijacked")
	}
	return h.Hijack()
}

func newCachedClient(config *rest.Config) (cluster.Cluster, error) {
	scheme := runtime.NewScheme()
	err := netv1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	corev1.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	return cluster.New(config, func(o *cluster.Options) {
		o.Scheme = scheme
	})
}

type informer struct {
	update func()
}

func (i *informer) OnAdd(obj interface{}, isInitialList bool) {
	i.update()
}

func (i *informer) OnUpdate(oldObj, newObj interface{}) {
	oldClientObj, ok1 := oldObj.(client.Object)
	newClientObj, ok2 := newObj.(client.Object)
	// TODO: take changed annotations into account
	if ok1 && ok2 && newClientObj.GetGeneration() > oldClientObj.GetGeneration() ||
		mapToString(oldClientObj.GetAnnotations()) != mapToString(newClientObj.GetAnnotations()) {
		i.update()
	}
}

func (i *informer) OnDelete(obj interface{}) {
	i.update()
}

func mapToString(m map[string]string) string {
	s := make([]string, 0, len(m))
	for k, v := range m {
		s = append(s, fmt.Sprintf("%q=%q", k, v))
	}
	return strings.Join(s, ",")
}
