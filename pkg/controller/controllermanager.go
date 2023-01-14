package controller

import (
	"context"
	"fmt"
	"io"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/mgoltzsche/kubemate/pkg/logrusadapter"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	//sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	// TODO: make controllers import work - it pulls in the google-api-go-client
	//kustomizecontrollers "github.com/fluxcd/kustomize-controller/controllers"
	//sourcecontrollers "github.com/fluxcd/source-controller/controllers"
)

type Reconciler interface {
	SetupWithManager(mgr ctrl.Manager) error
}

type SchemeBuilder interface {
	AddToScheme(s *runtime.Scheme) error
}

type ConfigFunc func() (*rest.Config, error)

type ControllerManager struct {
	configFn    ConfigFunc
	reconcilers []Reconciler
	scheme      *runtime.Scheme
	cancel      context.CancelFunc
	mutex       sync.Mutex
	wg          sync.WaitGroup
	logger      logr.Logger
}

func NewControllerManager(configFn ConfigFunc, logger *logrus.Entry) *ControllerManager {
	logrusAdapter := logrusadapter.New(logger)
	ctrl.SetLogger(logrusAdapter)
	return &ControllerManager{
		configFn: configFn,
		logger:   logrusAdapter,
	}
}

func (m *ControllerManager) RegisterReconciler(r Reconciler) {
	m.reconcilers = append(m.reconcilers, r)
}

func (m *ControllerManager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.cancel != nil {
		return nil // already running
	}
	logrus.Info("starting kubemate controller manager")
	m.scheme = runtime.NewScheme()
	for _, r := range m.reconcilers {
		sb, ok := r.(SchemeBuilder)
		if ok {
			err := sb.AddToScheme(m.scheme)
			if err != nil {
				return fmt.Errorf("add scheme of reconciler %T: %w", r, err)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer cancel()
		attempt := 0
		logLevel := logrus.DebugLevel
		for {
			if err := ctx.Err(); err != nil {
				logrus.Debug("stopping kubemate controller manager restart loop")
				return
			}
			err := runControllerManager(ctx, m.configFn, m.scheme, m.reconcilers, m.logger)
			if err != nil {
				if e := ctx.Err(); e == nil {
					logrus.WithError(err).Log(logLevel, "kubemate controller manager failed")
				}
			}
			time.Sleep(time.Second)
			attempt++
			if attempt > 10 {
				logLevel = logrus.WarnLevel
			}
		}
	}()
	return nil
}

func (m *ControllerManager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if c := m.cancel; c != nil {
		m.logger.Info("stopping controller manager")
		m.cancel = nil
		c()
		m.wg.Wait()
		m.closeReconcilers()
	}
}

func (m *ControllerManager) closeReconcilers() {
	for _, r := range m.reconcilers {
		rc, ok := r.(io.Closer)
		if ok {
			err := rc.Close()
			if err != nil {
				m.logger.Error(err, fmt.Sprintf("failed to close reconciler of type %T", r))
			}
		}
	}
}

func runControllerManager(ctx context.Context, cfg ConfigFunc, scheme *runtime.Scheme, reconcilers []Reconciler, logger logr.Logger) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("kubemate controller manager paniced: %s\nstacktrace:\n%s", e, string(debug.Stack()))
		}
	}()
	config, err := cfg()
	if err != nil {
		return err
	}
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Port:                   9443,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
		LeaderElection:         false,
		Logger:                 logger,
	})
	if err != nil {
		return err
	}
	for _, r := range reconcilers {
		err := r.SetupWithManager(mgr)
		if err != nil {
			return fmt.Errorf("reconciler %T setup: %w", r, err)
		}
	}
	err = mgr.Start(ctx)
	if err != nil {
		return fmt.Errorf("start manager: %w", err)
	}
	return nil
}
