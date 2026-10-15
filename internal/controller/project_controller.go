package controller

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

type ProjectReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Options OperatorOptions
	logger  logr.Logger
}

// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=projects,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=projects/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=projects/finalizers,verbs=update
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=registries,verbs=get;list;watch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=preheatpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborconnections;clusterharborconnections,verbs=get;list;watch

func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = log.FromContext(ctx).WithName(fmt.Sprintf("[Project:%s]", req.NamespacedName))

	// Load CR
	var cr harborv1alpha1.Project
	if found, err := loadResource(ctx, r.Client, req.NamespacedName, &cr, r.logger); err != nil {
		return ctrl.Result{}, err
	} else if !found {
		return ctrl.Result{}, nil
	}

	if err := markReconcilingIfNeeded(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation); err != nil {
		return ctrl.Result{}, err
	}

	// Resolve Harbor connection + typed client
	hc, err := getHarborClientForObject(ctx, r.Options, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Namespace, cr.Spec.HarborConnectionRef)
	if err != nil {
		if done, finalErr := finalizeWithoutHarborConnection(ctx, r.Client, &cr, cr.Spec.GetDeletionPolicy(), true, err); done {
			return ctrl.Result{}, finalErr
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}
	if !cr.DeletionTimestamp.IsZero() {
		if err := r.ensureNoPreheatPolicies(ctx, &cr); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
	}

	// Handle deletion
	if done, err := finalizeIfDeleting(ctx, r.Client, &cr, cr.Spec.GetDeletionPolicy(), func() error {
		return r.deleteProject(ctx, hc, &cr)
	}); done {
		return ctrl.Result{}, err
	}

	// Ensure finalizer
	if err := ensureFinalizer(ctx, r.Client, &cr); err != nil {
		return ctrl.Result{}, err
	}

	// Defaults & adoption
	if cr.Status.HarborProjectID == 0 && allowsAdoption(r.Options, cr.Spec.CreationPolicy) {
		if adopted, err := r.adoptExisting(ctx, hc, &cr); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		} else if adopted {
			r.logger.Info("Adopted existing project",
				"Name", cr.Name, "ID", cr.Status.HarborProjectID)
		}
	}

	// Desired payload
	createReq, err := r.buildCreateReq(ctx, &cr)
	if err != nil {
		if isDependencyNotReady(err) {
			return waitForDependency(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}

	// Create / Update path
	if cr.Status.HarborProjectID == 0 {
		if err := requireCreationAllowed(r.Options, cr.Spec.CreationPolicy); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		// create
		newID, err := hc.CreateProject(ctx, createReq)
		if err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		cr.Status.HarborProjectID = newID
		if err := setReadyStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, "Created", "Project created"); err != nil {
			return ctrl.Result{}, err
		}
		r.logger.Info("Created project", "ID", newID)
		return returnWithDriftDetection(r.Options, &cr.Spec.HarborSpecBase)
	}

	// get current state
	current, err := hc.GetProjectByID(ctx, cr.Status.HarborProjectID)
	if err != nil {
		if harborclient.IsNotFound(err) {
			return requeueOnRemoteNotFound(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, func() {
				cr.Status.HarborProjectID = 0
			}, "Project not found in Harbor")
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
	}

	// compare desired vs. current
	if projectNeedsUpdate(createReq, *current) {
		// update
		if err := hc.UpdateProject(ctx, current.ProjectID, createReq); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, err)
		}
		r.logger.Info("Updated project", "ID", current.ProjectID)
	}
	if err := setReadyStatus(ctx, r.Client, &cr, &cr.Status.HarborStatusBase, cr.Generation, "Reconciled", "Project reconciled"); err != nil {
		return ctrl.Result{}, err
	}
	return returnWithDriftDetection(r.Options, &cr.Spec.HarborSpecBase)
}

func (r *ProjectReconciler) ensureNoPreheatPolicies(ctx context.Context, project *harborv1alpha1.Project) error {
	var policies harborv1alpha1.PreheatPolicyList
	if err := r.List(ctx, &policies); err != nil {
		return err
	}
	for i := range policies.Items {
		policy := &policies.Items[i]
		if policy.Spec.ProjectRef.Name != project.Name {
			continue
		}
		namespace := policy.Spec.ProjectRef.Namespace
		if namespace == "" {
			namespace = policy.Namespace
		}
		if namespace == project.Namespace {
			return fmt.Errorf("project %s/%s is still referenced by PreheatPolicy %s/%s; delete the policy first", project.Namespace, project.Name, policy.Namespace, policy.Name)
		}
	}
	return nil
}

func (r *ProjectReconciler) deleteProject(ctx context.Context, hc *harborclient.Client,
	cr *harborv1alpha1.Project) error {

	if cr.Status.HarborProjectID == 0 {
		return nil
	}
	err := hc.DeleteProject(ctx, cr.Status.HarborProjectID)
	if harborclient.IsNotFound(err) {
		r.logger.V(1).Info("Project already gone", "ID", cr.Status.HarborProjectID)
		return nil
	}
	return err
}

// adoption by name
func (r *ProjectReconciler) adoptExisting(ctx context.Context, hc *harborclient.Client, cr *harborv1alpha1.Project) (bool, error) {

	project, err := hc.FindProjectByName(ctx, cr.Name)
	if err != nil {
		return false, err
	}
	if project == nil {
		return false, nil
	}
	cr.Status.HarborProjectID = project.ProjectID
	return true, r.Status().Update(ctx, cr)
}

func (r *ProjectReconciler) buildCreateReq(ctx context.Context, cr *harborv1alpha1.Project) (harborclient.CreateProjectRequest, error) {
	var meta harborclient.ProjectMetadata
	if m := cr.Spec.Metadata; m != nil {
		meta = harborclient.ProjectMetadata{
			EnableContentTrust:        m.EnableContentTrust,
			EnableContentTrustCosign:  m.EnableContentTrustCosign,
			PreventVul:                m.PreventVul,
			Severity:                  m.Severity,
			AutoScan:                  m.AutoScan,
			AutoSBOMGeneration:        m.AutoSBOMGeneration,
			ReuseSysCVEAllowlist:      m.ReuseSysCVEAllowlist,
			RetentionID:               m.RetentionID,
			ProxySpeedKB:              m.ProxySpeedKB,
			MaxUpstreamConn:           m.MaxUpstreamConn,
			ProxyCacheLocalOnNotFound: m.ProxyCacheLocalOnNotFound,
			ProxyReferrerAPI:          m.ProxyReferrerAPI,
			ProxyCacheFilterPattern:   m.ProxyCacheFilterPattern,
			ProxyCacheFilterKind:      m.ProxyCacheFilterKind,
		}
	}
	meta.Public = strconv.FormatBool(cr.Spec.Public)

	var allow *harborclient.CVEAllowlist
	if a := cr.Spec.CVEAllowlist; a != nil {
		allow = &harborclient.CVEAllowlist{Items: make([]harborclient.CVEAllowlistItem, len(a.Items))}
		if a.ExpiresAt != nil {
			allow.ExpiresAt = a.ExpiresAt
		}
		for i, item := range a.Items {
			allow.Items[i].CveID = item.CveID
		}
	}

	var registryID *int
	if cr.Spec.RegistryRef != nil {
		id, err := resolveRegistryID(ctx, r.Options, r.Client, cr.Namespace, cr.Spec.RegistryRef)
		if err != nil {
			return harborclient.CreateProjectRequest{}, err
		}
		registryID = &id
	}

	return harborclient.CreateProjectRequest{
		ProjectName:  cr.Name,
		Public:       cr.Spec.Public,
		Owner:        cr.Spec.Owner,
		Metadata:     meta,
		CVEAllowlist: allow,
		RegistryID:   registryID,
	}, nil
}

func projectNeedsUpdate(desired harborclient.CreateProjectRequest,
	current harborclient.Project) bool {

	if desired.ProjectName != current.Name {
		return true
	}

	if desired.Metadata.Public != current.Metadata.Public {
		return true
	}

	if desired.Owner != "" && !strings.EqualFold(desired.Owner, current.OwnerName) {
		return true
	}

	if desired.RegistryID == nil && current.RegistryID != 0 {
		return true
	}
	if desired.RegistryID != nil && *desired.RegistryID != current.RegistryID {
		return true
	}

	mw := desired.Metadata
	mc := current.Metadata
	switch {
	case projectMetadataFieldChanged(mw.EnableContentTrust, mc.EnableContentTrust),
		projectMetadataFieldChanged(mw.EnableContentTrustCosign, mc.EnableContentTrustCosign),
		projectMetadataFieldChanged(mw.PreventVul, mc.PreventVul),
		projectMetadataFieldChanged(mw.Severity, mc.Severity),
		projectMetadataFieldChanged(mw.AutoScan, mc.AutoScan),
		projectMetadataFieldChanged(mw.AutoSBOMGeneration, mc.AutoSBOMGeneration),
		projectMetadataFieldChanged(mw.ReuseSysCVEAllowlist, mc.ReuseSysCVEAllowlist),
		projectMetadataFieldChanged(mw.RetentionID, mc.RetentionID),
		projectMetadataFieldChanged(mw.ProxySpeedKB, mc.ProxySpeedKB),
		projectMetadataFieldChanged(mw.MaxUpstreamConn, mc.MaxUpstreamConn),
		projectMetadataFieldChanged(mw.ProxyCacheLocalOnNotFound, mc.ProxyCacheLocalOnNotFound),
		projectMetadataFieldChanged(mw.ProxyReferrerAPI, mc.ProxyReferrerAPI),
		projectMetadataFieldChanged(mw.ProxyCacheFilterPattern, mc.ProxyCacheFilterPattern),
		projectMetadataFieldChanged(mw.ProxyCacheFilterKind, mc.ProxyCacheFilterKind):
		return true
	}

	if aw := desired.CVEAllowlist; aw != nil {
		ac := current.CVEAllowlist
		if optionalInt64Value(aw.ExpiresAt) != optionalInt64Value(ac.ExpiresAt) || len(aw.Items) != len(ac.Items) {
			return true
		}
		for i := range aw.Items {
			if aw.Items[i].CveID != ac.Items[i].CveID {
				return true
			}
		}
	}
	return false
}

func projectMetadataFieldChanged(desired, current *string) bool {
	if desired == nil {
		return false
	}
	if current == nil {
		return *desired != ""
	}
	return *desired != *current
}

func optionalInt64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder, err := setupHarborBackedController(
		mgr,
		r.Options,
		&harborv1alpha1.Project{},
		func() client.ObjectList { return &harborv1alpha1.ProjectList{} },
		func(obj client.Object) *harborv1alpha1.HarborConnectionReference {
			return obj.(*harborv1alpha1.Project).Spec.HarborConnectionRef
		},
		"project",
	)
	if err != nil {
		return err
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &harborv1alpha1.Project{}, projectRegistryRefIndex, func(raw client.Object) []string {
		project := raw.(*harborv1alpha1.Project)
		if key := registryReferenceKey(project.Namespace, project.Spec.RegistryRef); key != "" {
			return []string{key}
		}
		return nil
	}); err != nil {
		return err
	}
	builder = builder.Watches(
		&harborv1alpha1.Registry{},
		handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
			return requestsForIndexedObjects(ctx, mgr, func() client.ObjectList { return &harborv1alpha1.ProjectList{} }, projectRegistryRefIndex, client.ObjectKeyFromObject(object).String())
		}),
	)
	return builder.Complete(r)
}
