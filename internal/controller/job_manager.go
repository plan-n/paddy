package controller

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	paddyV1 "github.com/paddy/api/v1"
	appV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *GridReconciler) dealGrid(ctx context.Context, grid *paddyV1.Grid, req ctrl.Request) (err error) {
	logger := log.FromContext(ctx).WithName(grid.GetLogName())
	if grid.Status.Phase == "" {
		grid.Status.Phase = paddyV1.InitializingGridPhase
		if err = r.Status().Update(ctx, grid); err != nil {
			logger.Error(err, "Failed to update Grid phase")
			return err
		}
		if err = r.Get(ctx, req.NamespacedName, grid); err != nil {
			logger.Error(err, "Failed to re-fetch memcached")
			return err
		}
		r.recordEvent(grid)
	}
	deployment := &appV1.Deployment{}
	deployment, err = r.DeploymentClient.AppsV1().Deployments(grid.Namespace).Get(ctx, grid.Spec.TargetRef.Name, metav1.GetOptions{})
	if err != nil {
		meta.SetStatusCondition(&grid.Status.Conditions, metav1.Condition{Type: typeAvailableGrid, Status: metav1.ConditionFalse, Reason: "JobManage", Message: "Can not find target resource"})
		grid.Status.Phase = paddyV1.FailedGridPhase
		logger.Error(err, "Failed to get target deployment")
		r.recordEvent(grid)
		return err
	}

	logger.Info("Get target deployment success", "deployment", deployment.GetName())
	label, labelValue, err := r.getSelectorLabel(deployment)
	if err != nil {
		logger.Error(err, "get selector label failed")
		return
	}
	primaryLabelValue := fmt.Sprintf("%s-primary", labelValue)
	primary := &appV1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        grid.GetPrimaryName(),
			Namespace:   grid.Namespace,
			Labels:      deployment.Labels,
			Annotations: deployment.Annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(grid, schema.GroupVersionKind{
					Group:   paddyV1.GroupVersion.Group,
					Version: paddyV1.GroupVersion.Version,
					Kind:    paddyV1.GridKind,
				}),
			},
		},
		Spec: appV1.DeploymentSpec{
			ProgressDeadlineSeconds: deployment.Spec.ProgressDeadlineSeconds,
			MinReadySeconds:         deployment.Spec.MinReadySeconds,
			RevisionHistoryLimit:    deployment.Spec.RevisionHistoryLimit,
			Replicas:                deployment.Spec.Replicas,
			Strategy:                deployment.Spec.Strategy,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					label: primaryLabelValue,
				},
			},
			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      deployment.Spec.Template.ObjectMeta.Labels,
					Annotations: deployment.Spec.Template.ObjectMeta.Annotations,
				},
				// update spec with the primary secrets and config maps
				Spec: *deployment.Spec.Template.Spec.DeepCopy(),
			},
		},
	}
	_, err = r.DeploymentClient.AppsV1().Deployments(grid.Namespace).Create(ctx, primary, metav1.CreateOptions{})
	if err != nil {
		grid.Status.Phase = paddyV1.FailedGridPhase
		logger.Error(err, "Failed to create primary deployment")
		r.recordEvent(grid)
		return
	}
	grid.Status.Phase = paddyV1.InitializedGridPhase
	logger.Info("Create primary deployment success", "deployment", primary.ObjectMeta.Name)
	r.recordEvent(grid)
	return
}

// getSelectorLabel returns the selector match label
func (r *GridReconciler) getSelectorLabel(deployment *appV1.Deployment) (string, string, error) {
	for _, l := range []string{"app", "name"} {
		if _, ok := deployment.Spec.Selector.MatchLabels[l]; ok {
			return l, deployment.Spec.Selector.MatchLabels[l], nil
		}
	}

	return "", "", fmt.Errorf(
		"deployment %s.%s spec.selector.matchLabels must contain one of %v",
		deployment.Name, deployment.Namespace, []string{"app", "name"},
	)
}
