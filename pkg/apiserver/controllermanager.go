package apiserver

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/go-logr/logr"
	appsv1 "github.com/mgoltzsche/kubemate/pkg/apis/apps/v1alpha1"
	kubematectrl "github.com/mgoltzsche/kubemate/pkg/controller"
	"github.com/mgoltzsche/kubemate/pkg/logrusadapter"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	//sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	// TODO: make controllers import work - it pulls in the google-api-go-client
	//kustomizecontrollers "github.com/fluxcd/kustomize-controller/controllers"
	//sourcecontrollers "github.com/fluxcd/source-controller/controllers"
)

type controllerManager struct {
	scheme *runtime.Scheme
	cancel context.CancelFunc
	mutex  sync.Mutex
	wg     sync.WaitGroup
	logger logr.Logger
}

func newControllerManager(logger *logrus.Entry) *controllerManager {
	logrusAdapter := logrusadapter.New(logger)
	logrusAdapter = logrusAdapter.WithName("kubemate-controller-manager")
	ctrl.SetLogger(logrusAdapter)
	scheme := runtime.NewScheme()
	err := appsv1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	err = kustomizev1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	return &controllerManager{
		scheme: scheme,
		logger: logrusAdapter,
	}
}

func (m *controllerManager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.cancel != nil {
		return nil // already running
	}
	logrus.Info("starting kubemate controller manager")
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
			err := runControllerManager(ctx, m.scheme, m.logger)
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

func (m *controllerManager) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if c := m.cancel; c != nil {
		logrus.Info("stopping kubemate controller manager")
		m.cancel = nil
		c()
		m.wg.Wait()
	}
}

func runControllerManager(ctx context.Context, scheme *runtime.Scheme, logger logr.Logger) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("kubemate controller manager paniced: %s\nstacktrace:\n%s", e, string(debug.Stack()))
		}
	}()
	config, err := ctrl.GetConfig()
	if err != nil {
		return err
	}
	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Port:                   9443,
		MetricsBindAddress:     ":8981",
		HealthProbeBindAddress: ":8982",
		LeaderElection:         false,
		//LeaderElectionID:       "kubemate-controller-manager",
		Logger: logger,
	})
	if err != nil {
		return err
	}
	err = (&kubematectrl.AppReconciler{}).SetupWithManager(mgr)
	if err != nil {
		return err
	}

	err = mgr.Start(ctx)
	if err != nil {
		return fmt.Errorf("start manager: %w", err)
	}
	return nil
}
