package drain

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/drain"
)

// DrainNode marks a k8s node as unschedulable and terminates all Pods that run on the node.
// See https://github.com/kubernetes/kubectl/blob/v0.27.2/pkg/cmd/drain/drain.go#L295
func DrainNode(ctx context.Context, nodeName string, c kubernetes.Interface) error {
	logrus.WithField("node", nodeName).Info("draining node")
	err := cordonOrUncordon(ctx, nodeName, true, c)
	if err != nil {
		return fmt.Errorf("drain node: %w", err)
	}
	var out, errOut bytes.Buffer
	drainer := &drain.Helper{
		Out:                 &out,
		ErrOut:              &errOut,
		ChunkSize:           cmdutil.DefaultChunkSize,
		Client:              c,
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true,
		Force:               true,
		GracePeriodSeconds:  20,
		Timeout:             45 * time.Second,
		DisableEviction:     true, // force Pod deletion, bypassing PodDisruptionBudget check
	}
	list, errs := drainer.GetPodsForDeletion(nodeName)
	if errs != nil {
		return fmt.Errorf("get pods for deletion: %w", utilerrors.NewAggregate(errs))
	}
	if warnings := list.Warnings(); warnings != "" {
		logrus.Warn(warnings)
	}

	if err := drainer.DeleteOrEvictPods(list.Pods()); err != nil {
		pendingList, newErrs := drainer.GetPodsForDeletion(nodeName)
		if pendingList != nil {
			pods := pendingList.Pods()
			if len(pods) != 0 {
				logrus.WithError(err).Errorf("There are pending pods in node %q when an error occurred during node draining", nodeName)
				for _, pendingPod := range pods {
					logrus.WithField("pod", pendingPod.Name).Warn("pending pod")
				}
			}
		}
		if newErrs != nil {
			return fmt.Errorf("drain: get pending pods: %w", utilerrors.NewAggregate(newErrs))
		}
		return fmt.Errorf("delete or evict pods on node: %w", err)
	}
	return nil
}

func Uncordon(ctx context.Context, nodeName string, c kubernetes.Interface) error {
	logrus.WithField("node", nodeName).Info("uncordon node")
	return cordonOrUncordon(ctx, nodeName, false, c)
}

// cordonOrUncordon marks a node as un/schedulable.
func cordonOrUncordon(ctx context.Context, nodeName string, cordon bool, c kubernetes.Interface) error {
	n := &corev1.Node{}
	n.Name = nodeName
	h := drain.NewCordonHelper(n)
	err, patchErr := h.PatchOrReplaceWithContext(ctx, c, false)
	if patchErr != nil {
		return patchErr
	}
	if err != nil {
		return err
	}
	return nil
}
