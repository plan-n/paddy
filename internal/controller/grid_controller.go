/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	paddyv1 "github.com/paddy/api/v1"
)

// Definitions to manage status conditions
const (
	// typeAvailableGrid represents the status of the Deployment reconciliation
	typeAvailableGrid = "Available"
	// typeDegradedGrid represents the status used when the custom resource is deleted and the finalizer operations are yet to occur.
	typeDegradedGrid = "Degraded"
)

// GridReconciler reconciles a Grid object
type GridReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	NodeName string
}

//+kubebuilder:rbac:groups=paddy.io,resources=grids,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=paddy.io,resources=grids/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=paddy.io,resources=grids/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Grid object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *GridReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName(req.NamespacedName.String())
	grid := &paddyv1.Grid{}
	err := r.Get(ctx, req.NamespacedName, grid)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Grid resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Grid")
		return ctrl.Result{}, err
	}

	if grid.Status.Conditions == nil || len(grid.Status.Conditions) == 0 {
		meta.SetStatusCondition(&grid.Status.Conditions, metav1.Condition{Type: typeAvailableGrid, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciling"})
		if err = r.Status().Update(ctx, grid); err != nil {
			logger.Error(err, "Failed to update Grid status")
			return ctrl.Result{}, err
		}
		if err = r.Get(ctx, req.NamespacedName, grid); err != nil {
			logger.Error(err, "Failed to re-fetch memcached")
			return ctrl.Result{}, err
		}
	}
	if grid.Status.Phase == "" {
		grid.Status.Phase = paddyv1.InitializingGridPhase
		if err = r.Status().Update(ctx, grid); err != nil {
			logger.Error(err, "Failed to update Grid phase")
			return ctrl.Result{}, err
		}
		if err = r.Get(ctx, req.NamespacedName, grid); err != nil {
			logger.Error(err, "Failed to re-fetch memcached")
			return ctrl.Result{}, err
		}
		r.recordEvent(grid)
	}

	return ctrl.Result{}, nil
}

func (r *GridReconciler) recordEvent(grid *paddyv1.Grid) {
	switch grid.Status.Phase {
	case paddyv1.InitializingGridPhase:
		r.Recorder.Event(grid, v1.EventTypeNormal, "Initializing", "Grid is initializing")
	case paddyv1.InitializedGridPhase:
		r.Recorder.Event(grid, v1.EventTypeNormal, "Initialized", "Grid is initialized")
	case paddyv1.WaitingGridPhase:
		r.Recorder.Event(grid, v1.EventTypeNormal, "Updated", "Grid is waiting for progress, target has changed")
	case paddyv1.ProgressingGridPhase:
		r.Recorder.Event(grid, v1.EventTypeNormal, "Scheduled", "Grid is progressing")
	case paddyv1.SucceededGridPhase:
		r.Recorder.Event(grid, v1.EventTypeNormal, "Succeeded", "Grid is succeeded")
	case paddyv1.FailedGridPhase:
		r.Recorder.Event(grid, v1.EventTypeWarning, "Failed", "Grid is failed")
	case paddyv1.TerminatingGridPhase:
		r.Recorder.Event(grid, v1.EventTypeWarning, "Terminating", "Grid is terminating")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GridReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paddyv1.Grid{}).
		Complete(r)
}
