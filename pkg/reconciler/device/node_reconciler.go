package device

import (
	"context"

	"github.com/google/uuid"
	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"github.com/mgoltzsche/kubemate/pkg/clientconf"
	"github.com/mgoltzsche/kubemate/pkg/drain"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"sigs.k8s.io/controller-runtime/pkg/builder"
	//"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	nodeDrainAnnotation      = deviceapi.NodeDrainAnnotation
	nodeShutdownAnnotation   = "kubemate.mgoltzsche.github.com/shutdown"
	nodeUncordonAnnotation   = "kubemate.mgoltzsche.github.com/uncordon"
	nodeRestartedAnnotation  = "kubemate.mgoltzsche.github.com/restarted"
	nodeTerminatedAnnotation = "kubemate.mgoltzsche.github.com/terminated"
)

// NodeReconciler reconciles a Node object.
type NodeReconciler struct {
	DeviceName  string
	DeviceStore storage.Interface
	K3sDir      string
	Shutdown    func() error
	client.Client
	scheme   *runtime.Scheme
	rebootID string
}

func (r *NodeReconciler) AddToScheme(s *runtime.Scheme) error {
	err := corev1.AddToScheme(s)
	if err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = mgr.GetScheme()
	r.Client = mgr.GetClient()
	r.rebootID = uuid.New().String()
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		/*For(&corev1.Node{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(o client.Object) bool {
			m, ok := o.(metav1.Object)
			return ok && m.GetName() == r.DeviceName
		}))).*/
		Complete(r)
}

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	// Fetch Node
	n := corev1.Node{}
	err = r.Client.Get(ctx, req.NamespacedName, &n)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return requeue(err)
	}
	a := n.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}

	logger.V(1).Info("reconcile node")

	// Fetch Device resource
	d := deviceapi.Device{}
	err = r.DeviceStore.Get(r.DeviceName, &d)
	if err != nil {
		return ctrl.Result{}, err
	}

	// When running on master device
	if d.Spec.Mode == deviceapi.DeviceModeServer {
		// Drain the node when device is master and drain annotation is not empty.
		// Afterwards initiate shutdown by setting an annotation
		// (delegating to the NodeReconciler instance on that node)
		if a[nodeDrainAnnotation] == "true" {
			logger.Info("draining node")
			// TODO: allow to drain multiple nodes in parallel?! make this work using the controller client
			c, err := r.newCoreClient(deviceapi.DeviceModeServer)
			if err != nil {
				return ctrl.Result{}, err
			}
			a[nodeUncordonAnnotation] = "true"
			a[nodeRestartedAnnotation] = "false"
			n.SetAnnotations(a)
			err = r.Client.Update(ctx, &n)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = drain.DrainNode(ctx, n.Name, c)
			if err != nil {
				return ctrl.Result{}, err
			}
			delete(a, nodeDrainAnnotation)
			a[nodeShutdownAnnotation] = "true" // triggering shutdown on agent
			n.SetAnnotations(a)
			err = r.Client.Update(ctx, &n)
			return ctrl.Result{}, err
		}

		// Uncordon node after restart
		if a[nodeUncordonAnnotation] == "true" && a[nodeRestartedAnnotation] == "true" && a[nodeTerminatedAnnotation] == "" {
			c, err := r.newCoreClient(d.Spec.Mode)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = drain.Uncordon(ctx, n.Name, c)
			if err != nil {
				return ctrl.Result{}, err
			}
			delete(a, nodeUncordonAnnotation)
			n.SetAnnotations(a)
			err = r.Client.Update(ctx, &n)
			return ctrl.Result{}, err
		}
	}

	// Execute the following logic only on the corresponding agent/master node.
	if n.Name == r.DeviceName && a[nodeTerminatedAnnotation] != r.rebootID {
		// Shutdown when annotation set to true
		if a[nodeShutdownAnnotation] == "true" {
			logger.Info("terminating node")
			delete(a, nodeShutdownAnnotation)
			a[nodeTerminatedAnnotation] = r.rebootID
			n.SetAnnotations(a)
			err = r.Client.Update(ctx, &n)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = r.Shutdown()
			return ctrl.Result{}, err
		} else if a[nodeRestartedAnnotation] == "false" && a[nodeTerminatedAnnotation] != "" {
			delete(a, nodeTerminatedAnnotation)
			a[nodeRestartedAnnotation] = "true" // triggering uncordon on master
			n.SetAnnotations(a)
			err = r.Client.Update(ctx, &n)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *NodeReconciler) newCoreClient(m deviceapi.DeviceMode) (kubernetes.Interface, error) {
	config, err := clientconf.New(r.K3sDir, m)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}
