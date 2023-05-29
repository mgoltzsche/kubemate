package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	appsv1 "github.com/mgoltzsche/kubemate/pkg/apis/apps/v1alpha1"
	"github.com/mgoltzsche/kubemate/pkg/utils"
	corev1 "k8s.io/api/core/v1"
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
)

const (
	finalizerKubemate           = "kubemate.mgoltzsche.github.com"
	labelKustomizationName      = "kustomize.toolkit.fluxcd.io/name"
	labelKustomizationNamespace = "kustomize.toolkit.fluxcd.io/namespace"
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
	err = corev1.AddToScheme(s)
	if err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = mgr.GetScheme()
	r.Client = mgr.GetClient()
	req4ownerApp := handler.EnqueueRequestForOwner(r.scheme, mgr.GetRESTMapper(), &appsv1.App{})
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.App{}).
		//Owns(&kustomizev1.Kustomization{}).
		//Owns(&corev1.Secret{}).
		Watches(&kustomizev1.Kustomization{}, req4ownerApp).
		Watches(&corev1.Secret{}, req4ownerApp).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []ctrl.Request {
			if name := o.GetName(); strings.HasSuffix(name, "-userconfig") {
				return []ctrl.Request{{
					NamespacedName: types.NamespacedName{
						Name:      name[:len(name)-11],
						Namespace: o.GetNamespace(),
					},
				}}
			}
			return nil
		})).
		Watches(&appsv1.AppConfigSchema{}, handler.EnqueueRequestsFromMapFunc(func(_ context.Context, o client.Object) []ctrl.Request {
			if l := o.GetLabels(); l != nil {
				if name := l[labelKustomizationName]; name != "" {
					return []ctrl.Request{{
						NamespacedName: types.NamespacedName{
							Name:      name,
							Namespace: l[labelKustomizationNamespace],
						},
					}}
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
	// Delete Kustomization when App resource gets deleted
	if a.DeletionTimestamp != nil {
		deleted, err := r.deleteKustomization(ctx, req.NamespacedName)
		if err != nil || !deleted {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(a, finalizerKubemate)
		err = r.Client.Update(ctx, a)
		return ctrl.Result{}, err
	}
	// Reconcile Secret, copy Secret with content-addressable name if found
	s, err := r.reconcileSecret(ctx, a)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Reconcile Kustomization (create/update/delete)
	k, err := r.reconcileKustomization(ctx, a)
	// Update App status
	oldStatus := a.Status
	defer func() {
		if a.Status.State != oldStatus.State || a.Status.Message != oldStatus.Message || oldStatus.ConfigSchemaName != a.Status.ConfigSchemaName {
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
	// Load app configuration
	cs, err := r.configSchema(ctx, a)
	if err != nil {
		a.Status.State = appsv1.AppStateError
		a.Status.Message = err.Error()
		return ctrl.Result{}, nil
	}
	var configSchemaName string
	if cs != nil {
		configSchemaName = cs.Name
	}
	a.Status.ConfigSchemaName = configSchemaName

	// Update installation status
	a.Status.Message = ""
	c := getCondition(k.Status.Conditions, "Ready")
	if a.Spec.Enabled == nil || !*a.Spec.Enabled { // disabled
		if k.Generation > 0 {
			a.Status.State = appsv1.AppStateDeinstalling
		} else {
			a.Status.State = appsv1.AppStateNotInstalled
		}
	} else { // enabled
		defaultConfig, err := r.defaultConfigSecret(ctx, a)
		if err != nil {
			return ctrl.Result{}, err
		}
		if c.ObservedGeneration == k.Generation {
			if c.Status == metav1.ConditionTrue {
				a.Status.State = appsv1.AppStateInstalled
			} else if c.Status == metav1.ConditionFalse {
				a.Status.State = appsv1.AppStateError
				a.Status.Message = fmt.Sprintf("%s: %s", c.Reason, c.Message)
			} else {
				if k.Status.LastAppliedRevision == k.Status.LastAttemptedRevision {
					a.Status.State = appsv1.AppStateInstalling
				} else {
					a.Status.State = appsv1.AppStateUpgrading
				}
				msg := ""
				if cs != nil {
					// TODO: check if mandatory field specified or reflect that within status otherwise.
					validateConfiguration(cs, s, defaultConfig, a)
					msg = a.Status.Message
					if msg != "" {
						msg = fmt.Sprintf("%s. ", msg)
					}
				}
				a.Status.Message = fmt.Sprintf("%s: %s%s", c.Reason, msg, c.Message)
				return ctrl.Result{}, nil
			}
		} else {
			a.Status.State = appsv1.AppStateInstalling
		}
		if cs != nil {
			validateConfiguration(cs, s, defaultConfig, a)
		}
	}
	return ctrl.Result{}, nil
}

func (r *AppReconciler) defaultConfigSecret(ctx context.Context, a *appsv1.App) (*corev1.Secret, error) {
	key := types.NamespacedName{
		Name:      defaultConfigSecretName(a),
		Namespace: a.Namespace,
	}
	defaults := &corev1.Secret{}
	err := r.Client.Get(ctx, key, defaults)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	return defaults, nil
}

func defaultConfigSecretName(a *appsv1.App) string {
	return fmt.Sprintf("%s-defaultconfig", a.Name)
}

func validateConfiguration(cs *appsv1.AppConfigSchema, custom, defaults *corev1.Secret, a *appsv1.App) {
	for i, p := range cs.Spec.Params {
		if p.Name == "" {
			a.Status.State = appsv1.AppStateError
			a.Status.Message = fmt.Sprintf("invalid parameter definition: no name specified for app parameter definition %d", i)
			return
		}
		notUserDefined := custom == nil || custom.Data == nil || custom.Data[p.Name] == nil
		notDefault := defaults == nil || defaults.Data == nil || defaults.Data[p.Name] == nil
		if notDefault && notUserDefined {
			a.Status.State = appsv1.AppStateConfigRequired
			title := p.Title
			if title == "" {
				title = p.Name
			}
			a.Status.Message = fmt.Sprintf("%s must be specified", title)
			return
		}
	}
}

func getCondition(conditions []metav1.Condition, name string) metav1.Condition {
	for _, c := range conditions {
		if c.Type == name {
			return c
		}
	}
	return metav1.Condition{}
}

func (r *AppReconciler) configSchema(ctx context.Context, a *appsv1.App) (*appsv1.AppConfigSchema, error) {
	cs := &appsv1.AppConfigSchema{}
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(a), cs)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("load app config schema: %w", err)
	}
	return cs, nil
}

func (r *AppReconciler) reconcileSecret(ctx context.Context, a *appsv1.App) (*corev1.Secret, error) {
	key := types.NamespacedName{
		Name:      fmt.Sprintf("%s-userconfig", a.Name),
		Namespace: a.Namespace,
	}
	src := &corev1.Secret{}
	err := r.Client.Get(ctx, key, src)
	if err != nil {
		if errors.IsNotFound(err) {
			a.Status.ConfigSecretName = defaultConfigSecretName(a)
			return nil, nil
		}
		return nil, err
	}
	dst := &corev1.Secret{}
	key.Name = utils.TruncateName(fmt.Sprintf("%s-%s", key.Name, hash(src.Data)), 63)
	dst.Name = key.Name
	dst.Namespace = key.Namespace
	err = r.Client.Get(ctx, key, dst)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		// Create new Secret with content-addressable name if not exists
		dst.Data = src.Data
		err := controllerutil.SetOwnerReference(a, dst, r.scheme)
		if err != nil {
			return nil, err
		}
		err = r.Client.Create(ctx, dst)
		if err != nil {
			return nil, err
		}
		a.Status.ConfigSecretName = key.Name
		return src, nil
	}
	a.Status.ConfigSecretName = key.Name
	return src, nil
}

func hash(o map[string][]byte) string {
	b, _ := json.Marshal(o)
	h := sha256.New()
	_, _ = h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func (r *AppReconciler) reconcileKustomization(ctx context.Context, a *appsv1.App) (*kustomizev1.Kustomization, error) {
	// Try to fetch Kustomization
	key := types.NamespacedName{Name: a.Name, Namespace: a.Namespace}
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
			Path:    a.Spec.Kustomization.Path,
			Timeout: a.Spec.Kustomization.Timeout,
			Prune:   true,
			Wait:    true,
			PostBuild: &kustomizev1.PostBuild{
				Substitute: map[string]string{
					"APP_NAME":               a.Name,
					"APP_CONFIG_SECRET_NAME": a.Status.ConfigSecretName,
				},
			},
		}
		if k.Annotations == nil {
			k.Annotations = map[string]string{}
		}
		err := controllerutil.SetOwnerReference(a, k, r.scheme)
		if err != nil {
			return k, err
		}
		if found {
			// Update Kustomization resource if changed
			if !equality.Semantic.DeepEqual(oldObj.Spec, k.Spec) {
				err = r.Client.Update(ctx, k)
				return k, err
			}
		} else {
			// Create new Kustomization resource
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
