package controller

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

type HarborCVEAllowlistReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Options OperatorOptions
	logger  logr.Logger
}

// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborcveallowlists,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborcveallowlists/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborcveallowlists/finalizers,verbs=update
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborconnections;clusterharborconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *HarborCVEAllowlistReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = log.FromContext(ctx).WithName(fmt.Sprintf("[HarborCVEAllowlist:%s]", req.Name))
	var allowlist harborv1alpha1.HarborCVEAllowlist
	if found, err := loadResource(ctx, r.Client, req.NamespacedName, &allowlist, r.logger); err != nil {
		return ctrl.Result{}, err
	} else if !found {
		return ctrl.Result{}, nil
	}
	status := &allowlist.Status.HarborStatusBase
	if err := markReconcilingIfNeeded(ctx, r.Client, &allowlist, status, allowlist.Generation); err != nil {
		return ctrl.Result{}, err
	}
	hc, err := getHarborClientForObject(ctx, r.Options, r.Client, &allowlist, status, "", allowlist.Spec.HarborConnectionRef)
	if err != nil {
		if done, finalErr := finalizeWithoutHarborConnection(ctx, r.Client, &allowlist, allowlist.Spec.GetDeletionPolicy(), true, err); done {
			return ctrl.Result{}, finalErr
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &allowlist, status, allowlist.Generation, err)
	}
	if err := ensureSingletonOwner(ctx, r.Options, r.Client, &allowlist, &harborv1alpha1.HarborCVEAllowlistList{}, func(obj client.Object) *harborv1alpha1.HarborConnectionReference {
		return obj.(*harborv1alpha1.HarborCVEAllowlist).Spec.HarborConnectionRef
	}, "HarborCVEAllowlist"); err != nil {
		var conflict *singletonOwnershipConflictError
		if !allowlist.DeletionTimestamp.IsZero() && errors.As(err, &conflict) {
			_, finalErr := finalizeIfDeleting(ctx, r.Client, &allowlist, allowlist.Spec.GetDeletionPolicy(), nil)
			return ctrl.Result{}, finalErr
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &allowlist, status, allowlist.Generation, err)
	}
	if done, err := finalizeIfDeleting(ctx, r.Client, &allowlist, allowlist.Spec.GetDeletionPolicy(), func() error {
		return hc.UpdateSystemCVEAllowlist(ctx, harborclient.SystemCVEAllowlist{Items: []harborclient.SystemCVEAllowlistItem{}})
	}); done {
		return ctrl.Result{}, err
	}
	if err := ensureFinalizer(ctx, r.Client, &allowlist); err != nil {
		return ctrl.Result{}, err
	}
	desired := cveAllowlistFromSpec(allowlist.Spec)
	current, err := hc.GetSystemCVEAllowlist(ctx)
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &allowlist, status, allowlist.Generation, err)
	}
	if !sameCVEAllowlist(desired, *current) {
		if err := hc.UpdateSystemCVEAllowlist(ctx, desired); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &allowlist, status, allowlist.Generation, err)
		}
		current, err = hc.GetSystemCVEAllowlist(ctx)
		if err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &allowlist, status, allowlist.Generation, err)
		}
	}

	var updatedAt *metav1.Time
	if current.UpdateTime != "" {
		parsed, parseErr := time.Parse(time.RFC3339, current.UpdateTime)
		if parseErr != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &allowlist, status, allowlist.Generation, fmt.Errorf("parse Harbor CVE allowlist update time: %w", parseErr))
		}
		converted := metav1.NewTime(parsed)
		updatedAt = &converted
	}
	statusChanged := !sameTime(allowlist.Status.HarborUpdatedAt, updatedAt)
	allowlist.Status.HarborUpdatedAt = updatedAt
	conditionChanged := markReady(status, allowlist.Generation, "Reconciled", "System CVE allowlist reconciled")
	if statusChanged || conditionChanged {
		if err := r.Status().Update(ctx, &allowlist); err != nil {
			return ctrl.Result{}, err
		}
	}
	return returnWithDriftDetection(r.Options, &allowlist.Spec)
}

func (r *HarborCVEAllowlistReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder, err := setupHarborBackedController(
		mgr, r.Options, &harborv1alpha1.HarborCVEAllowlist{},
		func() client.ObjectList { return &harborv1alpha1.HarborCVEAllowlistList{} },
		func(obj client.Object) *harborv1alpha1.HarborConnectionReference {
			return obj.(*harborv1alpha1.HarborCVEAllowlist).Spec.HarborConnectionRef
		},
		"harborcveallowlist",
	)
	if err != nil {
		return err
	}
	return builder.Complete(r)
}

func cveAllowlistFromSpec(spec harborv1alpha1.HarborCVEAllowlistSpec) harborclient.SystemCVEAllowlist {
	items := make([]harborclient.SystemCVEAllowlistItem, 0, len(spec.Items))
	for _, item := range spec.Items {
		items = append(items, harborclient.SystemCVEAllowlistItem{CVEID: item.CVEID})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CVEID < items[j].CVEID })
	var expiresAt *int64
	if spec.ExpiresAt != nil {
		unix := spec.ExpiresAt.Unix()
		expiresAt = &unix
	}
	return harborclient.SystemCVEAllowlist{ExpiresAt: expiresAt, Items: items}
}

func sameCVEAllowlist(a, b harborclient.SystemCVEAllowlist) bool {
	if (a.ExpiresAt == nil) != (b.ExpiresAt == nil) || (a.ExpiresAt != nil && *a.ExpiresAt != *b.ExpiresAt) {
		return false
	}
	aItems := append([]harborclient.SystemCVEAllowlistItem(nil), a.Items...)
	bItems := append([]harborclient.SystemCVEAllowlistItem(nil), b.Items...)
	sort.Slice(aItems, func(i, j int) bool { return aItems[i].CVEID < aItems[j].CVEID })
	sort.Slice(bItems, func(i, j int) bool { return bItems[i].CVEID < bItems[j].CVEID })
	if len(aItems) != len(bItems) {
		return false
	}
	for i := range aItems {
		if aItems[i].CVEID != bItems[i].CVEID {
			return false
		}
	}
	return true
}

func sameTime(a, b *metav1.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(b)
}
