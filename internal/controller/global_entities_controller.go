package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

type globalEntityOperations struct {
	name       string
	find       func(context.Context, *harborclient.Client, string) (id int, found, builtin bool, err error)
	create     func(context.Context, *harborclient.Client, any) (int, error)
	get        func(context.Context, *harborclient.Client, string, int) (any, error)
	update     func(context.Context, *harborclient.Client, string, int, any) error
	delete     func(context.Context, *harborclient.Client, string, int) error
	equivalent func(desired, current any) bool
	builtin    func(current any) bool
}

// GlobalHarborEntityReconciler owns Harbor-global identities through a
// ClusterHarborConnection. Resource-specific API behavior is supplied by ops.
type GlobalHarborEntityReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Options      OperatorOptions
	Object       client.Object
	ListObject   client.ObjectList
	Spec         func(client.Object) *harborv1alpha1.HarborSpecBase
	Status       func(client.Object) *harborv1alpha1.HarborStatusBase
	ID           func(client.Object) int
	SetID        func(client.Object, int) bool
	Policy       func(client.Object) harborv1alpha1.CreationPolicy
	Desired      func(context.Context, *harborclient.Client, client.Object) (any, error)
	Validate     func(context.Context, client.Object) error
	Changed      func(client.Object, any) bool
	Applied      func(client.Object, any) bool
	Observed     func(client.Object, any) bool
	BeforeDelete func(context.Context, client.Object) error
	ops          globalEntityOperations
	logger       logr.Logger
}

// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborroles;harborusergroups;preheatinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborroles/status;harborusergroups/status;preheatinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborroles/finalizers;harborusergroups/finalizers;preheatinstances/finalizers,verbs=update
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=harborconnections;clusterharborconnections,verbs=get;list;watch
// +kubebuilder:rbac:groups=harbor.harbor-entity-operator.io,resources=preheatpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *GlobalHarborEntityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.logger = log.FromContext(ctx).WithName(fmt.Sprintf("[%s:%s]", r.ops.name, req.Name))
	obj := r.Object.DeepCopyObject().(client.Object)
	if found, err := loadResource(ctx, r.Client, req.NamespacedName, obj, r.logger); err != nil {
		return ctrl.Result{}, err
	} else if !found {
		return ctrl.Result{}, nil
	}

	status := r.Status(obj)
	if err := markReconcilingIfNeeded(ctx, r.Client, obj, status, obj.GetGeneration()); err != nil {
		return ctrl.Result{}, err
	}
	hc, done, err := r.prepareResource(ctx, obj)
	if done {
		return ctrl.Result{}, err
	}
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), err)
	}
	return r.reconcileDesired(ctx, obj, hc)
}

func (r *GlobalHarborEntityReconciler) prepareResource(ctx context.Context, obj client.Object) (*harborclient.Client, bool, error) {
	spec := r.Spec(obj)
	status := r.Status(obj)

	hc, err := getHarborClientForObject(ctx, r.Options, r.Client, obj, status, obj.GetNamespace(), spec.HarborConnectionRef)
	if err != nil {
		if done, finalErr := finalizeWithoutHarborConnection(ctx, r.Client, obj, spec.GetDeletionPolicy(), true, err); done {
			return nil, true, finalErr
		}
		return nil, true, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), err)
	}
	identityOwner, identityConflict, err := r.globalIdentityOwner(ctx, obj)
	if !obj.GetDeletionTimestamp().IsZero() && identityConflict && !identityOwner {
		_, finalErr := finalizeIfDeleting(ctx, r.Client, obj, spec.GetDeletionPolicy(), nil)
		return nil, true, finalErr
	}
	if err != nil {
		return nil, true, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), err)
	}
	if obj.GetDeletionTimestamp().IsZero() && !identityOwner {
		return nil, true, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), fmt.Errorf("%s %s is not the owner of its Harbor identity", r.ops.name, client.ObjectKeyFromObject(obj)))
	}
	if !obj.GetDeletionTimestamp().IsZero() && r.BeforeDelete != nil {
		if err := r.BeforeDelete(ctx, obj); err != nil {
			return nil, true, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), err)
		}
	}
	if done, err := finalizeIfDeleting(ctx, r.Client, obj, spec.GetDeletionPolicy(), func() error {
		if r.ID(obj) == 0 {
			return nil
		}
		return r.ops.delete(ctx, hc, obj.GetName(), r.ID(obj))
	}); done {
		return nil, true, err
	}
	if err := ensureFinalizer(ctx, r.Client, obj); err != nil {
		return nil, true, err
	}
	if r.Validate != nil {
		if err := r.Validate(ctx, obj); err != nil {
			return nil, true, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), err)
		}
	}
	return hc, false, nil
}

func (r *GlobalHarborEntityReconciler) reconcileDesired(ctx context.Context, obj client.Object, hc *harborclient.Client) (ctrl.Result, error) {
	status := r.Status(obj)
	spec := r.Spec(obj)
	desired, err := r.Desired(ctx, hc, obj)
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), err)
	}
	id, err := r.ensureHarborIdentity(ctx, obj, hc, desired)
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), redactDesiredError(err, desired))
	}

	current, err := r.ops.get(ctx, hc, obj.GetName(), id)
	if err != nil {
		if harborclient.IsNotFound(err) {
			return requeueOnRemoteNotFound(ctx, r.Client, obj, status, obj.GetGeneration(), func() { r.SetID(obj, 0) }, r.ops.name+" not found in Harbor")
		}
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), redactDesiredError(err, desired))
	}
	if r.ops.builtin != nil && r.ops.builtin(current) {
		err := fmt.Errorf("built-in Harbor %s %q cannot be managed", r.ops.name, obj.GetName())
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), err)
	}
	current, err = r.applyDesired(ctx, obj, hc, id, desired, current)
	if err != nil {
		return ctrl.Result{}, setErrorStatus(ctx, r.Client, obj, status, obj.GetGeneration(), redactDesiredError(err, desired))
	}
	statusChanged := false
	if r.Applied != nil {
		statusChanged = r.Applied(obj, desired) || statusChanged
	}
	if r.Observed != nil {
		statusChanged = r.Observed(obj, current) || statusChanged
	}
	if changed := markReady(status, obj.GetGeneration(), "Reconciled", r.ops.name+" reconciled"); changed || statusChanged {
		if err := r.Client.Status().Update(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
	}
	return returnWithDriftDetection(r.Options, spec)
}

func (r *GlobalHarborEntityReconciler) ensureHarborIdentity(ctx context.Context, obj client.Object, hc *harborclient.Client, desired any) (int, error) {
	id := r.ID(obj)
	if id != 0 {
		return id, nil
	}
	id, err := r.acquire(ctx, hc, obj, desired)
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, fmt.Errorf("%s %s returned no Harbor ID", r.ops.name, obj.GetName())
	}
	if r.SetID(obj, id) {
		if err := r.Client.Status().Update(ctx, obj); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (r *GlobalHarborEntityReconciler) applyDesired(ctx context.Context, obj client.Object, hc *harborclient.Client, id int, desired, current any) (any, error) {
	if r.ops.equivalent(desired, current) && (r.Changed == nil || !r.Changed(obj, desired)) {
		return current, nil
	}
	if err := r.ops.update(ctx, hc, obj.GetName(), id, desired); err != nil {
		return nil, err
	}
	return r.ops.get(ctx, hc, obj.GetName(), id)
}

func (r *GlobalHarborEntityReconciler) globalIdentityOwner(ctx context.Context, current client.Object) (owner, conflict bool, err error) {
	connection, err := resolveHarborConnection(ctx, r.Options, r.Client, current.GetNamespace(), r.Spec(current).HarborConnectionRef)
	if err != nil {
		return false, false, err
	}
	list := r.ListObject.DeepCopyObject().(client.ObjectList)
	if err := r.List(ctx, list); err != nil {
		return false, false, err
	}
	peers := []singletonCandidate{}
	if err := apimeta.EachListItem(list, func(raw runtime.Object) error {
		peer, ok := raw.(client.Object)
		if !ok || client.ObjectKeyFromObject(peer) == client.ObjectKeyFromObject(current) || !peer.GetDeletionTimestamp().IsZero() {
			return nil
		}
		if !strings.EqualFold(peer.GetName(), current.GetName()) {
			return nil
		}
		peerConnection, err := resolveHarborConnection(ctx, r.Options, r.Client, peer.GetNamespace(), r.Spec(peer).HarborConnectionRef)
		if err != nil || normalizeBaseURL(peerConnection.baseURL) != normalizeBaseURL(connection.baseURL) {
			return nil
		}
		peers = append(peers, singletonCandidate{name: client.ObjectKeyFromObject(peer), createdAt: peer.GetCreationTimestamp()})
		return nil
	}); err != nil {
		return false, false, err
	}
	if len(peers) == 0 {
		return true, false, nil
	}
	conflictErr := singletonOwnerConflict(current, connection.baseURL, peers, r.ops.name)
	return conflictErr == nil, true, conflictErr
}

func redactDesiredError(err error, desired any) error {
	instance, ok := desired.(*harborclient.PreheatInstance)
	if !ok || len(instance.AuthInfo) == 0 {
		return err
	}
	values := make([]string, 0, len(instance.AuthInfo))
	for _, value := range instance.AuthInfo {
		if value != "" {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	message := err.Error()
	for _, value := range values {
		message = strings.ReplaceAll(message, value, "[redacted]")
	}
	return errors.New(message)
}

func (r *GlobalHarborEntityReconciler) acquire(ctx context.Context, hc *harborclient.Client, obj client.Object, desired any) (int, error) {
	id, found, builtin, err := r.ops.find(ctx, hc, obj.GetName())
	if err != nil {
		return 0, err
	}
	if found {
		if builtin {
			return 0, fmt.Errorf("built-in Harbor %s %q cannot be adopted", r.ops.name, obj.GetName())
		}
		if !allowsAdoption(r.Options, r.Policy(obj)) {
			return 0, fmt.Errorf("harbor %s %q already exists; set creationPolicy to Adopt or CreateOrAdopt to manage it", r.ops.name, obj.GetName())
		}
		return id, nil
	}
	if err := requireCreationAllowed(r.Options, r.Policy(obj)); err != nil {
		return 0, err
	}
	id, err = r.ops.create(ctx, hc, desired)
	if err != nil {
		return 0, err
	}
	if id != 0 {
		return id, nil
	}
	id, found, _, err = r.ops.find(ctx, hc, obj.GetName())
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("harbor accepted %s %q but it was not visible in the subsequent list", r.ops.name, obj.GetName())
	}
	return id, nil
}

func (r *GlobalHarborEntityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder, err := setupHarborBackedController(
		mgr, r.Options, r.Object,
		func() client.ObjectList { return r.ListObject.DeepCopyObject().(client.ObjectList) },
		func(obj client.Object) *harborv1alpha1.HarborConnectionReference {
			return r.Spec(obj).HarborConnectionRef
		},
		r.ops.name,
	)
	if err != nil {
		return err
	}
	if r.ops.name == "preheatinstance" {
		builder = watchSecretReferences(builder, r.Client,
			func() client.ObjectList { return &harborv1alpha1.PreheatInstanceList{} },
			func(obj client.Object) []harborv1alpha1.SecretReference {
				ref := obj.(*harborv1alpha1.PreheatInstance).Spec.AuthInfoSecretRef
				if ref == nil {
					return nil
				}
				return []harborv1alpha1.SecretReference{{Name: ref.Name, Namespace: ref.Namespace}}
			},
		)
	}
	return builder.Complete(r)
}

func NewHarborRoleReconciler(c client.Client, scheme *runtime.Scheme, options OperatorOptions) *GlobalHarborEntityReconciler {
	return &GlobalHarborEntityReconciler{
		Client: c, Scheme: scheme, Options: options,
		Object: &harborv1alpha1.HarborRole{}, ListObject: &harborv1alpha1.HarborRoleList{},
		Spec: func(obj client.Object) *harborv1alpha1.HarborSpecBase {
			return &obj.(*harborv1alpha1.HarborRole).Spec.HarborSpecBase
		},
		Status: func(obj client.Object) *harborv1alpha1.HarborStatusBase {
			return &obj.(*harborv1alpha1.HarborRole).Status.HarborStatusBase
		},
		ID: func(obj client.Object) int { return obj.(*harborv1alpha1.HarborRole).Status.HarborRoleID },
		SetID: func(obj client.Object, id int) bool {
			x := obj.(*harborv1alpha1.HarborRole)
			changed := x.Status.HarborRoleID != id
			x.Status.HarborRoleID = id
			return changed
		},
		Policy: func(obj client.Object) harborv1alpha1.CreationPolicy {
			return obj.(*harborv1alpha1.HarborRole).Spec.CreationPolicy
		},
		Desired: func(_ context.Context, _ *harborclient.Client, obj client.Object) (any, error) {
			x := obj.(*harborv1alpha1.HarborRole)
			permissions := make([]harborclient.HarborRolePermission, 0, len(x.Spec.Permissions))
			for _, permission := range x.Spec.Permissions {
				access := make([]harborclient.HarborRoleAccess, 0, len(permission.Access))
				for _, rule := range permission.Access {
					access = append(access, harborclient.HarborRoleAccess{Resource: rule.Resource, Action: rule.Action, Effect: rule.Effect})
				}
				permissions = append(permissions, harborclient.HarborRolePermission{Kind: permission.Kind, Namespace: permission.Namespace, Access: access})
			}
			return &harborclient.HarborRole{Name: x.Name, Description: x.Spec.Description, Permissions: permissions}, nil
		},
		ops: harborRoleOperations(),
	}
}

func harborRoleOperations() globalEntityOperations {
	return globalEntityOperations{
		name: "harborrole",
		find: func(ctx context.Context, hc *harborclient.Client, name string) (int, bool, bool, error) {
			roles, err := hc.ListHarborRoles(ctx)
			if err != nil {
				return 0, false, false, err
			}
			for _, role := range roles {
				if strings.EqualFold(role.Name, name) {
					return role.ID, true, role.IsBuiltin, nil
				}
			}
			return 0, false, false, nil
		},
		create: func(ctx context.Context, hc *harborclient.Client, desired any) (int, error) {
			return hc.CreateHarborRole(ctx, *desired.(*harborclient.HarborRole))
		},
		get: func(ctx context.Context, hc *harborclient.Client, _ string, id int) (any, error) {
			return hc.GetHarborRole(ctx, id)
		},
		update: func(ctx context.Context, hc *harborclient.Client, _ string, id int, desired any) error {
			role := *desired.(*harborclient.HarborRole)
			role.ID = id
			return hc.UpdateHarborRole(ctx, id, role)
		},
		delete: func(ctx context.Context, hc *harborclient.Client, _ string, id int) error {
			return hc.DeleteHarborRole(ctx, id)
		},
		equivalent: func(desired, current any) bool {
			a, b := desired.(*harborclient.HarborRole), current.(*harborclient.HarborRole)
			return a.Name == b.Name && a.Description == b.Description && reflect.DeepEqual(a.Permissions, b.Permissions)
		},
		builtin: func(current any) bool { return current.(*harborclient.HarborRole).IsBuiltin },
	}
}

func NewHarborUserGroupReconciler(c client.Client, scheme *runtime.Scheme, options OperatorOptions) *GlobalHarborEntityReconciler {
	r := &GlobalHarborEntityReconciler{
		Client: c, Scheme: scheme, Options: options,
		Object: &harborv1alpha1.HarborUserGroup{}, ListObject: &harborv1alpha1.HarborUserGroupList{},
		Spec: func(obj client.Object) *harborv1alpha1.HarborSpecBase {
			return &obj.(*harborv1alpha1.HarborUserGroup).Spec.HarborSpecBase
		},
		Status: func(obj client.Object) *harborv1alpha1.HarborStatusBase {
			return &obj.(*harborv1alpha1.HarborUserGroup).Status.HarborStatusBase
		},
		ID: func(obj client.Object) int { return obj.(*harborv1alpha1.HarborUserGroup).Status.HarborUserGroupID },
		SetID: func(obj client.Object, id int) bool {
			x := obj.(*harborv1alpha1.HarborUserGroup)
			changed := x.Status.HarborUserGroupID != id
			x.Status.HarborUserGroupID = id
			return changed
		},
		Policy: func(obj client.Object) harborv1alpha1.CreationPolicy {
			return obj.(*harborv1alpha1.HarborUserGroup).Spec.CreationPolicy
		},
		Desired: func(_ context.Context, _ *harborclient.Client, obj client.Object) (any, error) {
			x := obj.(*harborv1alpha1.HarborUserGroup)
			return &harborclient.UserGroup{GroupName: x.Name, GroupType: x.Spec.GroupType, LDAPGroupDN: x.Spec.LDAPGroupDN}, nil
		},
		ops: harborUserGroupOperations(),
	}
	r.BeforeDelete = func(ctx context.Context, obj client.Object) error {
		group := obj.(*harborv1alpha1.HarborUserGroup)
		groupConnection, err := resolveHarborConnection(ctx, options, c, "", group.Spec.HarborConnectionRef)
		if err != nil {
			return err
		}
		var claims harborv1alpha1.UserGroupClaimList
		if err := c.List(ctx, &claims); err != nil {
			return err
		}
		for i := range claims.Items {
			claim := &claims.Items[i]
			if !strings.EqualFold(claim.Spec.GroupName, group.Name) {
				continue
			}
			claimConnection, err := resolveHarborConnection(ctx, options, c, claim.Namespace, claim.Spec.HarborConnectionRef)
			if err != nil {
				return err
			}
			if normalizeBaseURL(claimConnection.baseURL) == normalizeBaseURL(groupConnection.baseURL) {
				return fmt.Errorf("HarborUserGroup %q is still claimed by UserGroupClaim %s/%s", group.Name, claim.Namespace, claim.Name)
			}
		}
		return nil
	}
	return r
}

func harborUserGroupOperations() globalEntityOperations {
	return globalEntityOperations{
		name: "harborusergroup",
		find: func(ctx context.Context, hc *harborclient.Client, name string) (int, bool, bool, error) {
			groups, err := hc.ListUserGroups(ctx)
			if err != nil {
				return 0, false, false, err
			}
			for _, group := range groups {
				if strings.EqualFold(group.GroupName, name) {
					return group.ID, true, false, nil
				}
			}
			return 0, false, false, nil
		},
		create: func(ctx context.Context, hc *harborclient.Client, desired any) (int, error) {
			return hc.CreateUserGroup(ctx, *desired.(*harborclient.UserGroup))
		},
		get: func(ctx context.Context, hc *harborclient.Client, _ string, id int) (any, error) {
			return hc.GetUserGroup(ctx, id)
		},
		update: func(ctx context.Context, hc *harborclient.Client, _ string, id int, desired any) error {
			group := *desired.(*harborclient.UserGroup)
			group.ID = id
			return hc.UpdateUserGroup(ctx, id, group)
		},
		delete: func(ctx context.Context, hc *harborclient.Client, _ string, id int) error {
			return hc.DeleteUserGroup(ctx, id)
		},
		equivalent: func(desired, current any) bool {
			a, b := desired.(*harborclient.UserGroup), current.(*harborclient.UserGroup)
			return a.GroupName == b.GroupName && a.GroupType == b.GroupType && a.LDAPGroupDN == b.LDAPGroupDN
		},
	}
}

func NewPreheatInstanceReconciler(c client.Client, scheme *runtime.Scheme, options OperatorOptions) *GlobalHarborEntityReconciler {
	r := &GlobalHarborEntityReconciler{
		Client: c, Scheme: scheme, Options: options,
		Object: &harborv1alpha1.PreheatInstance{}, ListObject: &harborv1alpha1.PreheatInstanceList{},
		Spec: func(obj client.Object) *harborv1alpha1.HarborSpecBase {
			return &obj.(*harborv1alpha1.PreheatInstance).Spec.HarborSpecBase
		},
		Status: func(obj client.Object) *harborv1alpha1.HarborStatusBase {
			return &obj.(*harborv1alpha1.PreheatInstance).Status.HarborStatusBase
		},
		ID: func(obj client.Object) int { return obj.(*harborv1alpha1.PreheatInstance).Status.HarborInstanceID },
		SetID: func(obj client.Object, id int) bool {
			x := obj.(*harborv1alpha1.PreheatInstance)
			changed := x.Status.HarborInstanceID != id
			x.Status.HarborInstanceID = id
			return changed
		},
		Policy: func(obj client.Object) harborv1alpha1.CreationPolicy {
			return obj.(*harborv1alpha1.PreheatInstance).Spec.CreationPolicy
		},
		Desired: func(ctx context.Context, _ *harborclient.Client, obj client.Object) (any, error) {
			x := obj.(*harborv1alpha1.PreheatInstance)
			var authInfo map[string]string
			if ref := x.Spec.AuthInfoSecretRef; ref != nil {
				secret, err := readSecretData(ctx, options, c, *ref)
				if err != nil {
					return nil, fmt.Errorf("failed to read authInfoSecretRef: %w", err)
				}
				authInfo = secret
			}
			if x.Spec.AuthMode != "NONE" && len(authInfo) == 0 {
				return nil, fmt.Errorf("authInfoSecretRef must contain at least one credential when authMode is not NONE")
			}
			return &harborclient.PreheatInstance{Name: x.Name, Description: x.Spec.Description, Vendor: x.Spec.Vendor, Endpoint: x.Spec.Endpoint, AuthMode: x.Spec.AuthMode, AuthInfo: authInfo, Enabled: x.Spec.Enabled, Default: x.Spec.Default, Insecure: x.Spec.Insecure}, nil
		},
		Observed: func(obj client.Object, current any) bool {
			x := obj.(*harborv1alpha1.PreheatInstance)
			status := current.(*harborclient.PreheatInstance).Status
			changed := x.Status.HarborStatus != status
			x.Status.HarborStatus = status
			return changed
		},
		ops: preheatInstanceOperations(),
	}
	r.Changed = func(obj client.Object, desired any) bool {
		x := obj.(*harborv1alpha1.PreheatInstance)
		if x.Spec.AuthInfoSecretRef == nil {
			return x.Status.AuthInfoHash != ""
		}
		return x.Status.AuthInfoHash != hashParts(secretDataCanonical(desired.(*harborclient.PreheatInstance).AuthInfo))
	}
	r.Applied = func(obj client.Object, desired any) bool {
		x := obj.(*harborv1alpha1.PreheatInstance)
		oldHash := x.Status.AuthInfoHash
		if x.Spec.AuthInfoSecretRef == nil {
			x.Status.AuthInfoHash = ""
		} else {
			x.Status.AuthInfoHash = hashParts(secretDataCanonical(desired.(*harborclient.PreheatInstance).AuthInfo))
		}
		return oldHash != x.Status.AuthInfoHash
	}
	r.BeforeDelete = func(ctx context.Context, obj client.Object) error {
		var policies harborv1alpha1.PreheatPolicyList
		if err := c.List(ctx, &policies); err != nil {
			return err
		}
		instance := obj.(*harborv1alpha1.PreheatInstance)
		for i := range policies.Items {
			policy := &policies.Items[i]
			if policy.Spec.ProviderRef.Name == instance.Name {
				return fmt.Errorf("PreheatInstance %q is still referenced by PreheatPolicy %s/%s", instance.Name, policy.Namespace, policy.Name)
			}
		}
		return nil
	}
	return r
}

func preheatInstanceOperations() globalEntityOperations {
	return globalEntityOperations{
		name: "preheatinstance",
		find: func(ctx context.Context, hc *harborclient.Client, name string) (int, bool, bool, error) {
			instances, err := hc.ListPreheatInstances(ctx)
			if err != nil {
				return 0, false, false, err
			}
			for _, instance := range instances {
				if strings.EqualFold(instance.Name, name) {
					return instance.ID, true, false, nil
				}
			}
			return 0, false, false, nil
		},
		create: func(ctx context.Context, hc *harborclient.Client, desired any) (int, error) {
			return hc.CreatePreheatInstance(ctx, *desired.(*harborclient.PreheatInstance))
		},
		get: func(ctx context.Context, hc *harborclient.Client, name string, _ int) (any, error) {
			return hc.GetPreheatInstance(ctx, name)
		},
		update: func(ctx context.Context, hc *harborclient.Client, name string, id int, desired any) error {
			instance := *desired.(*harborclient.PreheatInstance)
			instance.ID = id
			return hc.UpdatePreheatInstance(ctx, name, instance)
		},
		delete: func(ctx context.Context, hc *harborclient.Client, name string, _ int) error {
			return hc.DeletePreheatInstance(ctx, name)
		},
		equivalent: func(desired, current any) bool {
			a, b := desired.(*harborclient.PreheatInstance), current.(*harborclient.PreheatInstance)
			return a.Name == b.Name && a.Description == b.Description && a.Vendor == b.Vendor && a.Endpoint == b.Endpoint && a.AuthMode == b.AuthMode && a.Enabled == b.Enabled && a.Default == b.Default && a.Insecure == b.Insecure
		},
	}
}

func readSecretData(ctx context.Context, options OperatorOptions, c client.Client, ref harborv1alpha1.SecretObjectReference) (map[string]string, error) {
	if ref.Name == "" || ref.Namespace == "" {
		return nil, fmt.Errorf("auth info Secret reference must include name and namespace")
	}
	var secret corev1.Secret
	reader := client.Reader(c)
	if options.secretReader != nil {
		reader = options.secretReader
	}
	if err := reader.Get(ctx, types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, &secret); err != nil {
		return nil, err
	}
	data := make(map[string]string, len(secret.Data))
	for key, value := range secret.Data {
		if key == "" || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("auth info Secret contains an empty key")
		}
		data[key] = string(value)
	}
	return data, nil
}

func secretDataCanonical(data map[string]string) string {
	// encoding/json sorts map keys and avoids ambiguity between key/value pairs.
	serialized, _ := json.Marshal(data)
	return string(serialized)
}
