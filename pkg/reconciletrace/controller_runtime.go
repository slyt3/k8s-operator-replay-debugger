//go:build controller_runtime
// +build controller_runtime

package reconciletrace

import (
	"context"

	"github.com/slyt3/kubestep/pkg/storage"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type wrappedReconciler struct {
	inner         reconcile.Reconciler
	store         storage.ReconcileSpanStore
	sessionID     string
	actorID       string
	gvk           schema.GroupVersionKind
	triggerReason string
}

// WrapReconciler wraps a controller-runtime Reconciler to record spans.
func WrapReconciler(
	r reconcile.Reconciler,
	store storage.ReconcileSpanStore,
	sessionID string,
	actorID string,
	gvk schema.GroupVersionKind,
	triggerReason string,
) reconcile.Reconciler {
	return &wrappedReconciler{
		inner:         r,
		store:         store,
		sessionID:     sessionID,
		actorID:       actorID,
		gvk:           gvk,
		triggerReason: triggerReason,
	}
}

func (w *wrappedReconciler) Reconcile(
	ctx context.Context,
	req reconcile.Request,
) (reconcile.Result, error) {
	spanID, spanCtx := Start(
		ctx,
		w.store,
		w.sessionID,
		w.actorID,
		w.gvk,
		req.Namespace,
		req.Name,
		"",
		"",
		w.triggerReason,
	)

	result, err := w.inner.Reconcile(spanCtx, req)
	End(spanCtx, w.store, spanID, err)
	return result, err
}
