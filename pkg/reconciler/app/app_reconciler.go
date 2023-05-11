package app

import (
	"context"
	"fmt"
	"strings"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	appsv1 "github.com/mgoltzsche/kubemate/pkg/apis/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	annotationAppOwner = "kubemate.mgoltzsche.github.com/app"
	finalizerKubemate  = "kubemate.mgoltzsche.github.com"
)

// AppReconciler reconciles an App object.
type AppReconciler struct {
	client.Client
	scheme *runtime.Scheme
}

func (r *AppReconciler) AddToScheme(s *runtime.Scheme) error {
	err := appsv1.AddToScheme(s)
	if err != nil {
		return err
	}
	err = kustomizev1.AddToScheme(s)
	if err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = mgr.GetScheme()
	r.Client = mgr.GetClient()
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.App{}).
		//Owns(&kustomizev1.Kustomization{}).
		Watches(&source.Kind{Type: &kustomizev1.Kustomization{}}, handler.EnqueueRequestsFromMapFunc(func(o client.Object) []ctrl.Request {
			if a := o.GetAnnotations(); a != nil {
				if owner := a[annotationAppOwner]; owner != "" {
					name := strings.SplitN(owner, "/", 2)
					if len(name) == 2 {
						return []ctrl.Request{{NamespacedName: types.NamespacedName{
							Namespace: name[0],
							Name:      name[1],
						}}}
					}
				}
			}
			return nil
		})).
		Complete(r)
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to move the current state of the cluster closer to the desired state.
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	// Fetch App
	a := &appsv1.App{}
	err := r.Client.Get(ctx, req.NamespacedName, a)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if a.Spec.Kustomization == nil {
		logger.Info("app does not specify kustomization")
		return ctrl.Result{}, nil
	}
	logger.V(1).Info("reconcile app")
	// Add finalizer to App
	if !controllerutil.ContainsFinalizer(a, finalizerKubemate) {
		controllerutil.AddFinalizer(a, finalizerKubemate)
		err = r.Client.Update(ctx, a)
		return ctrl.Result{}, err
	}
	// Delete kustomization when App resource gets deleted
	if a.DeletionTimestamp != nil {
		if a.Status.Namespace != "" {
			done, err := r.deleteKustomization(ctx, types.NamespacedName{
				Name:      a.Name,
				Namespace: a.Status.Namespace,
			})
			if err != nil || !done {
				return ctrl.Result{}, err
			}
		}
		controllerutil.RemoveFinalizer(a, finalizerKubemate)
		err = r.Client.Update(ctx, a)
		return ctrl.Result{}, err
	}
	// Delete previous Kustomization if namespace changed
	if a.Status.Namespace != "" && a.Status.Namespace != a.Namespace() {
		done, err := r.deleteKustomization(ctx, types.NamespacedName{
			Name:      a.Name,
			Namespace: a.Status.Namespace,
		})
		if err != nil || !done {
			return ctrl.Result{}, err
		}
	}
	// Reconcile Kustomization (create/update/delete)
	k, err := r.reconcileKustomization(ctx, a)
	// Update App status
	oldStatus := a.Status
	defer func() {
		if a.Status.State != oldStatus.State || a.Status.Message != oldStatus.Message || a.Status.Namespace != oldStatus.Namespace {
			a.Status.ObservedGeneration = a.Generation
			if k != nil {
				a.Status.LastAppliedRevision = k.Status.LastAppliedRevision
				a.Status.LastAttemptedRevision = k.Status.LastAttemptedRevision
			}
			_ = r.Client.Status().Update(ctx, a) // Update App status
		}
	}()
	if err != nil {
		if !errors.IsConflict(err) {
			a.Status.State = appsv1.AppStateError
			a.Status.Message = err.Error()
		}
		return ctrl.Result{}, err
	}
	c := getCondition(k.Status.Conditions, "Ready")
	if a.Spec.Enabled == nil || !*a.Spec.Enabled {
		if k.Generation > 0 {
			a.Status.State = appsv1.AppStateDeinstalling
		} else {
			a.Status.State = appsv1.AppStateNotInstalled
		}
		a.Status.Message = ""
	} else {
		if k.Status.ObservedGeneration == k.Generation {
			if c.Status == metav1.ConditionTrue {
				a.Status.State = appsv1.AppStateInstalled
			} else {
				if k.Status.LastAppliedRevision == k.Status.LastAttemptedRevision {
					a.Status.State = appsv1.AppStateInstalling
				} else {
					a.Status.State = appsv1.AppStateUpgrading
				}
				a.Status.Message = fmt.Sprintf("%s: %s", c.Reason, c.Message)
			}
		} else {
			a.Status.State = appsv1.AppStateInstalling
			a.Status.Message = ""
		}
	}
	return ctrl.Result{}, nil
}

func getCondition(conditions []metav1.Condition, name string) metav1.Condition {
	for _, c := range conditions {
		if c.Type == name {
			return c
		}
	}
	return metav1.Condition{}
}

func (r *AppReconciler) reconcileKustomization(ctx context.Context, a *appsv1.App) (*kustomizev1.Kustomization, error) {
	// Try to fetch Kustomization
	ns := a.Namespace()
	key := types.NamespacedName{
		Name:      a.Name,
		Namespace: ns,
	}
	k := &kustomizev1.Kustomization{}
	found := true
	err := r.Client.Get(ctx, key, k)
	if err != nil {
		if !errors.IsNotFound(err) {
			return k, err
		}
		found = false
	}
	if a.Spec.Enabled != nil && *a.Spec.Enabled {
		// Install
		k.Name = key.Name
		k.Namespace = key.Namespace
		sourceRef := a.Spec.Kustomization.SourceRef
		oldObj := &kustomizev1.Kustomization{}
		k.DeepCopyInto(oldObj)
		k.Spec = kustomizev1.KustomizationSpec{
			SourceRef: kustomizev1.CrossNamespaceSourceReference{
				Kind:      sourceRef.Kind,
				Name:      sourceRef.Name,
				Namespace: sourceRef.Namespace,
			},
			Path:            a.Spec.Kustomization.Path,
			TargetNamespace: a.Spec.Kustomization.TargetNamespace,
			Timeout:         a.Spec.Kustomization.Timeout,
			Prune:           true,
			Wait:            true,
		}
		if k.Annotations == nil {
			k.Annotations = map[string]string{}
		}
		k.Annotations[annotationAppOwner] = fmt.Sprintf("%s/%s", a.ObjectMeta.Namespace, a.Name)
		if k.Namespace == a.ObjectMeta.Namespace {
			err := controllerutil.SetOwnerReference(a, k, r.scheme)
			if err != nil {
				return k, err
			}
		}
		if found {
			// Update Kustomization resource if changed
			if !equality.Semantic.DeepEqual(oldObj.Spec, k.Spec) {
				err = r.Client.Update(ctx, k)
				return k, err
			}
		} else {
			// Create new Kustomization resource
			if a.Status.Namespace != k.Namespace {
				a.Status.Namespace = k.Namespace
				err = r.Client.Status().Update(ctx, k)
				if err != nil {
					return k, err
				}
			}
			err = r.Client.Create(ctx, k)
			return k, err
		}
	} else {
		// Uninstall
		if found {
			err = r.Client.Delete(ctx, k)
			return k, err
		}
	}
	return k, nil
}

func (r *AppReconciler) deleteKustomization(ctx context.Context, key types.NamespacedName) (bool, error) {
	k := &kustomizev1.Kustomization{}
	err := r.Client.Get(ctx, key, k)
	if err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	err = r.Client.Delete(ctx, k)
	if errors.IsNotFound(err) {
		return true, nil
	}
	return false, err
}
