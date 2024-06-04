package controller

import (
	"context"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	paddyV1 "github.com/paddy/api/v1"
	appV1 "k8s.io/api/apps/v1"
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
	logger.Info("find target resource", "deployment", deployment.Name)
	return
}
