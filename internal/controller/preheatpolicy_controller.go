package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

type PreheatPolicyReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Options OperatorOptions
	logger  logr.Logger
}

// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=preheatpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=preheatpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=preheatpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=projects;preheatinstances,verbs=get;list;watch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborconnections;clusterharborconnections,verbs=get;list;watch

func (r *PreheatPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = log.FromContext(ctx).WithName(fmt.Sprintf("[PreheatPolicy:%s]", req.NamespacedName))
	var policy harborv1alpha1.PreheatPolicy
	if found, err := loadResource(ctx, r.Client, req.NamespacedName, &policy, r.logger); err != nil {
		return ctrl.Result{}, err
	} else if !found {
		return ctrl.Result{}, nil
	}
	previousProjectID := policy.Status.HarborProjectID
	previousPolicyID := policy.Status.HarborPreheatPolicyID
	status := &policy.Status.HarborStatusBase
	if err := markReconcilingIfNeeded(ctx, r.Client, &policy, status, policy.Generation); err != nil {
		return ctrl.Result{}, err
	}
	hc, done, err := r.preparePolicy(ctx, &policy)
	if done {
		return ctrl.Result{}, err
	}
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &policy, status, policy.Generation, err)
	}
	projectID, provider, result, done, err := r.resolvePolicyDependencies(ctx, &policy, hc)
	if done {
		return result, err
	}
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, &policy, status, policy.Generation, err)
	}
	return r.reconcilePolicy(ctx, &policy, hc, projectID, provider, previousProjectID, previousPolicyID)
}

func (r *PreheatPolicyReconciler) preparePolicy(ctx context.Context, policy *harborv1alpha1.PreheatPolicy) (*harborclient.Client, bool, error) {
	status := &policy.Status.HarborStatusBase
	hc, err := getHarborClientForObject(ctx, r.Options, r.Client, policy, status, policy.Namespace, policy.Spec.HarborConnectionRef)
	if err != nil {
		if done, finalErr := finalizeWithoutHarborConnection(ctx, r.Client, policy, policy.Spec.GetDeletionPolicy(), true, err); done {
			return nil, true, finalErr
		}
		return nil, true, setErrorStatus(ctx, r.Client, policy, status, policy.Generation, err)
	}
	if done, err := finalizeIfDeleting(ctx, r.Client, policy, policy.Spec.GetDeletionPolicy(), func() error {
		if policy.Spec.ProjectRef.Name == "" || policy.Status.HarborProjectID == 0 {
			return nil
		}
		return hc.DeletePreheatPolicy(ctx, policy.Spec.ProjectRef.Name, policy.Name)
	}); done {
		return nil, true, err
	}
	if err := ensureFinalizer(ctx, r.Client, policy); err != nil {
		return nil, true, err
	}
	if err := validatePreheatPolicyJSON(policy.Spec); err != nil {
		return nil, true, setErrorStatus(ctx, r.Client, policy, status, policy.Generation, err)
	}
	return hc, false, nil
}

func (r *PreheatPolicyReconciler) resolvePolicyDependencies(ctx context.Context, policy *harborv1alpha1.PreheatPolicy, hc *harborclient.Client) (int, *harborv1alpha1.PreheatInstance, ctrl.Result, bool, error) {
	projectID, err := r.resolveProject(ctx, policy)
	if err != nil {
		result, reconcileErr := r.dependencyError(ctx, policy, err)
		return 0, nil, result, true, reconcileErr
	}
	provider, err := r.resolveProvider(ctx, policy, hc)
	if err != nil {
		result, reconcileErr := r.dependencyError(ctx, policy, err)
		return 0, nil, result, true, reconcileErr
	}
	return projectID, provider, ctrl.Result{}, false, nil
}

func (r *PreheatPolicyReconciler) dependencyError(ctx context.Context, policy *harborv1alpha1.PreheatPolicy, err error) (ctrl.Result, error) {
	status := &policy.Status.HarborStatusBase
	if isDependencyNotReady(err) {
		return waitForDependency(ctx, r.Client, policy, status, policy.Generation, err)
	}
	return ctrl.Result{}, setErrorStatus(ctx, r.Client, policy, status, policy.Generation, err)
}

func (r *PreheatPolicyReconciler) reconcilePolicy(ctx context.Context, policy *harborv1alpha1.PreheatPolicy, hc *harborclient.Client, projectID int, provider *harborv1alpha1.PreheatInstance, previousProjectID, previousPolicyID int) (ctrl.Result, error) {
	status := &policy.Status.HarborStatusBase
	desired := harborclient.PreheatPolicy{
		Name: policy.Name, Description: policy.Spec.Description,
		ProjectID: projectID, ProviderID: provider.Status.HarborInstanceID,
		ProviderName: provider.Name, Filters: policy.Spec.Filters,
		Trigger: policy.Spec.Trigger, Enabled: policy.Spec.Enabled,
		ExtraAttrs: policy.Spec.ExtraAttrs,
	}
	projectPath := policy.Spec.ProjectRef.Name
	_, err := r.ensurePolicyID(ctx, policy, hc, projectPath, desired)
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, policy, status, policy.Generation, err)
	}
	policy.Status.HarborProjectID = projectID
	current, err := hc.GetPreheatPolicy(ctx, projectPath, policy.Name)
	if err != nil {
		if harborclient.IsNotFound(err) {
			return requeueOnRemoteNotFound(ctx, r.Client, policy, status, policy.Generation, func() { policy.Status.HarborPreheatPolicyID = 0 }, "PreheatPolicy not found in Harbor")
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, policy, status, policy.Generation, err)
	}
	if !samePreheatPolicy(desired, *current) {
		updated := desired
		updated.ID = current.ID
		if err := hc.UpdatePreheatPolicy(ctx, projectPath, policy.Name, updated); err != nil {
			return ctrl.Result{}, setErrorStatus(ctx, r.Client, policy, status, policy.Generation, err)
		}
	}
	statusChanged := previousProjectID != projectID || previousPolicyID != current.ID
	policy.Status.HarborPreheatPolicyID = current.ID
	conditionChanged := markReady(status, policy.Generation, "Reconciled", "Preheat policy reconciled")
	if statusChanged || conditionChanged {
		if err := r.Status().Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
	}
	return returnWithDriftDetection(r.Options, &policy.Spec.HarborSpecBase)
}

func (r *PreheatPolicyReconciler) ensurePolicyID(ctx context.Context, policy *harborv1alpha1.PreheatPolicy, hc *harborclient.Client, projectPath string, desired harborclient.PreheatPolicy) (int, error) {
	if policy.Status.HarborPreheatPolicyID != 0 {
		return policy.Status.HarborPreheatPolicyID, nil
	}
	policies, err := hc.ListPreheatPolicies(ctx, projectPath)
	if err != nil {
		return 0, err
	}
	for i := range policies {
		if policies[i].Name == policy.Name {
			if !allowsAdoption(r.Options, policy.Spec.CreationPolicy) {
				return 0, fmt.Errorf("harbor preheat policy %q already exists; set creationPolicy to Adopt or CreateOrAdopt to manage it", policy.Name)
			}
			return policies[i].ID, nil
		}
	}
	if err := requireCreationAllowed(r.Options, policy.Spec.CreationPolicy); err != nil {
		return 0, err
	}
	id, err := hc.CreatePreheatPolicy(ctx, projectPath, desired)
	if err != nil {
		return 0, err
	}
	if id != 0 {
		return id, nil
	}
	policies, err = hc.ListPreheatPolicies(ctx, projectPath)
	if err != nil {
		return 0, err
	}
	for i := range policies {
		if policies[i].Name == policy.Name {
			return policies[i].ID, nil
		}
	}
	return 0, fmt.Errorf("harbor accepted preheat policy %q but did not return a visible policy ID", policy.Name)
}

func validatePreheatPolicyJSON(spec harborv1alpha1.PreheatPolicySpec) error {
	if spec.Filters != "" {
		var filters []json.RawMessage
		if err := json.Unmarshal([]byte(spec.Filters), &filters); err != nil || filters == nil {
			return fmt.Errorf("spec.filters must be a JSON array")
		}
	}
	for field, value := range map[string]string{"trigger": spec.Trigger, "extraAttrs": spec.ExtraAttrs} {
		if value == "" {
			continue
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal([]byte(value), &object); err != nil || object == nil {
			return fmt.Errorf("spec.%s must be a JSON object", field)
		}
	}
	return nil
}

func (r *PreheatPolicyReconciler) resolveProject(ctx context.Context, policy *harborv1alpha1.PreheatPolicy) (int, error) {
	namespace := policy.Spec.ProjectRef.Namespace
	if namespace == "" {
		namespace = policy.Namespace
	}
	if err := validateReferenceNamespace(r.Options, policy.Namespace, namespace, harborProjectResourceKind); err != nil {
		return 0, err
	}
	var project harborv1alpha1.Project
	key := types.NamespacedName{Namespace: namespace, Name: policy.Spec.ProjectRef.Name}
	if err := r.Get(ctx, key, &project); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return 0, &dependencyNotReadyError{kind: harborProjectResourceKind, key: key, reason: dependencyResourceNotFoundReason}
		}
		return 0, err
	}
	if project.Status.HarborProjectID == 0 || project.Status.ResolvedHarborConnection == nil {
		return 0, &dependencyNotReadyError{kind: harborProjectResourceKind, key: key, reason: "project is not Ready"}
	}
	projectConnection, err := resolveHarborConnection(ctx, r.Options, r.Client, project.Namespace, project.Spec.HarborConnectionRef)
	if err != nil {
		return 0, err
	}
	policyConnection, err := resolveHarborConnection(ctx, r.Options, r.Client, policy.Namespace, policy.Spec.HarborConnectionRef)
	if err != nil {
		return 0, err
	}
	if normalizeBaseURL(projectConnection.baseURL) != normalizeBaseURL(policyConnection.baseURL) {
		return 0, fmt.Errorf("project %s/%s and PreheatPolicy resolve to different Harbor instances", project.Namespace, project.Name)
	}
	return project.Status.HarborProjectID, nil
}

func (r *PreheatPolicyReconciler) resolveProvider(ctx context.Context, policy *harborv1alpha1.PreheatPolicy, hc *harborclient.Client) (*harborv1alpha1.PreheatInstance, error) {
	if policy.Spec.ProviderRef == "" {
		return nil, fmt.Errorf("spec.providerRef is required")
	}
	var instance harborv1alpha1.PreheatInstance
	if err := r.Get(ctx, types.NamespacedName{Name: policy.Spec.ProviderRef}, &instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, &dependencyNotReadyError{kind: "PreheatInstance", key: types.NamespacedName{Name: policy.Spec.ProviderRef}, reason: dependencyResourceNotFoundReason}
		}
		return nil, err
	}
	if instance.Status.HarborInstanceID == 0 || instance.Status.ResolvedHarborConnection == nil {
		return nil, &dependencyNotReadyError{kind: "PreheatInstance", key: types.NamespacedName{Name: instance.Name}, reason: "provider instance is not Ready"}
	}
	instanceConnection, err := resolveHarborConnection(ctx, r.Options, r.Client, "", instance.Spec.HarborConnectionRef)
	if err != nil {
		return nil, err
	}
	if normalizeBaseURL(instanceConnection.baseURL) != strings.TrimRight(hc.BaseURL, "/") {
		return nil, fmt.Errorf("PreheatInstance %q and PreheatPolicy resolve to different Harbor instances", instance.Name)
	}
	return &instance, nil
}

func samePreheatPolicy(a, b harborclient.PreheatPolicy) bool {
	return a.Name == b.Name && a.Description == b.Description && a.ProjectID == b.ProjectID &&
		a.ProviderID == b.ProviderID && a.ProviderName == b.ProviderName && a.Filters == b.Filters &&
		a.Trigger == b.Trigger && a.Enabled == b.Enabled && a.ExtraAttrs == b.ExtraAttrs
}

func (r *PreheatPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder, err := setupHarborBackedController(
		mgr, r.Options, &harborv1alpha1.PreheatPolicy{},
		func() client.ObjectList { return &harborv1alpha1.PreheatPolicyList{} },
		func(obj client.Object) *harborv1alpha1.HarborConnectionReference {
			return obj.(*harborv1alpha1.PreheatPolicy).Spec.HarborConnectionRef
		},
		"preheatpolicy",
	)
	if err != nil {
		return err
	}
	builder.Watches(&harborv1alpha1.Project{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
		var policies harborv1alpha1.PreheatPolicyList
		if err := r.List(ctx, &policies); err != nil {
			return nil
		}
		requests := []reconcile.Request{}
		for i := range policies.Items {
			policy := &policies.Items[i]
			if policy.Spec.ProjectRef.Name != object.GetName() {
				continue
			}
			namespace := policy.Spec.ProjectRef.Namespace
			if namespace == "" {
				namespace = policy.Namespace
			}
			if namespace == object.GetNamespace() {
				requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(policy)})
			}
		}
		return requests
	}))
	builder.Watches(&harborv1alpha1.PreheatInstance{}, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []reconcile.Request {
		var policies harborv1alpha1.PreheatPolicyList
		if err := r.List(ctx, &policies); err != nil {
			return nil
		}
		requests := []reconcile.Request{}
		for i := range policies.Items {
			if policies.Items[i].Spec.ProviderRef == object.GetName() {
				requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&policies.Items[i])})
			}
		}
		return requests
	}))
	return builder.Complete(r)
}
