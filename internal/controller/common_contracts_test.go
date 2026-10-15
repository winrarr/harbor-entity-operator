package controller

import (
	"context"
	"fmt"
	"testing"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func TestDependencyErrorClassificationAndConnectionSecretReferences(t *testing.T) {
	dependencyErr := &dependencyNotReadyError{kind: "Project", key: types.NamespacedName{Namespace: "team", Name: "demo"}}
	if !isDependencyNotReady(fmt.Errorf("waiting: %w", dependencyErr)) {
		t.Fatal("wrapped dependency error was not recognized")
	}
	if isDependencyNotReady(fmt.Errorf("unrelated failure")) || isDependencyNotReady(nil) {
		t.Fatal("unrelated or nil error was classified as a missing dependency")
	}

	credentials := &harborv1alpha1.Credentials{
		UsernameSecretRef: &harborv1alpha1.SecretReference{Name: "user", Key: "username"},
		PasswordSecretRef: harborv1alpha1.SecretReference{Name: "password", Key: "token"},
	}
	refs := harborConnectionSecretReferences(harborv1alpha1.HarborConnectionSpec{
		CABundleSecretRef: &harborv1alpha1.SecretReference{Name: "ca", Key: "ca.crt"},
		Credentials:       credentials,
	})
	if len(refs) != 3 || refs[0].Name != "ca" || refs[1].Name != "user" || refs[2].Name != "password" {
		t.Fatalf("connection Secret references = %#v, want CA, username, and password references", refs)
	}
	refs = harborConnectionSecretReferences(harborv1alpha1.HarborConnectionSpec{Credentials: &harborv1alpha1.Credentials{}})
	if len(refs) != 0 {
		t.Fatalf("empty credentials produced Secret references: %#v", refs)
	}
}

func TestReferenceKeysResolveNamespaceDefaultsAndEmptyReferences(t *testing.T) {
	if got, want := referenceKey("workloads", "registry", ""), "workloads/registry"; got != want {
		t.Fatalf("defaulted reference key = %q, want %q", got, want)
	}
	if got, want := referenceKey("workloads", "registry", "shared"), "shared/registry"; got != want {
		t.Fatalf("explicit reference key = %q, want %q", got, want)
	}
	if registryReferenceKey("workloads", nil) != "" || registryReferenceKey("workloads", &harborv1alpha1.RegistryReference{}) != "" {
		t.Fatal("nil or empty registry references should not create index keys")
	}
	if projectReferenceKey("workloads", nil) != "" || projectReferenceKey("workloads", &harborv1alpha1.ProjectReference{}) != "" {
		t.Fatal("nil or empty project references should not create index keys")
	}
	if got := registryReferenceKey("workloads", &harborv1alpha1.RegistryReference{Name: "registry", Namespace: "shared"}); got != "shared/registry" {
		t.Fatalf("registry reference key = %q", got)
	}
	if got := projectReferenceKey("workloads", &harborv1alpha1.ProjectReference{Name: "project"}); got != "workloads/project" {
		t.Fatalf("project reference key = %q", got)
	}
}

func TestReferenceWatchMappingsReturnOnlyMatchingDependents(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := harborv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add Harbor scheme: %v", err)
	}
	objects := []client.Object{
		&harborv1alpha1.Label{ObjectMeta: metav1.ObjectMeta{Name: "matching", Namespace: "team-a"}},
		&harborv1alpha1.Label{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "team-b"}},
	}
	indexName := "test.dependent-name"
	kubeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&harborv1alpha1.Label{}, indexName, func(obj client.Object) []string {
			return []string{obj.GetName()}
		}).
		WithObjects(objects...).
		Build()
	mgr := clientOnlyManager{Manager: nil, kubeClient: kubeClient}

	indexed := requestsForIndexedObjects(ctx, mgr, func() client.ObjectList { return &harborv1alpha1.LabelList{} }, indexName, "matching")
	if len(indexed) != 1 || indexed[0].NamespacedName != (types.NamespacedName{Namespace: "team-a", Name: "matching"}) {
		t.Fatalf("indexed dependents = %#v, want only team-a/matching", indexed)
	}
	all := requestsForAllHarborBackedObjects(ctx, mgr, func() client.ObjectList { return &harborv1alpha1.LabelList{} })
	if len(all) != 2 {
		t.Fatalf("all dependents = %#v, want two objects", all)
	}
}

func TestResolveUserGroupRequiresReadyClaimAndMatchingHarborConnection(t *testing.T) {
	ctx := context.Background()
	connection := harborv1alpha1.HarborConnectionBinding{
		Kind: harborv1alpha1.HarborConnectionReferenceKindCluster,
		Name: "primary-harbor",
		UID:  "connection-uid",
	}
	readyClaim := &harborv1alpha1.UserGroupClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "developers", Namespace: "team"},
		Spec:       harborv1alpha1.UserGroupClaimSpec{GroupName: "developers", GroupType: 2},
		Status: harborv1alpha1.UserGroupClaimStatus{
			HarborGroupID: 17,
			HarborStatusBase: harborv1alpha1.HarborStatusBase{
				ResolvedHarborConnection: &connection,
			},
		},
	}
	meta.SetStatusCondition(&readyClaim.Status.Conditions, metav1.Condition{
		Type: ConditionReady, Status: metav1.ConditionTrue, Reason: "Reconciled", Message: "ready", ObservedGeneration: 1,
	})
	kubeClient := newAdvancedUnitClient(t, readyClaim)
	ref := harborv1alpha1.UserGroupClaimReference{Name: "developers"}

	group, err := resolveUserGroup(ctx, OperatorOptions{}, kubeClient, "team", ref, &connection)
	if err != nil || group == nil || group.GroupName != "developers" || group.GroupType != 2 {
		t.Fatalf("resolved group = %#v, %v", group, err)
	}
	if _, err := resolveUserGroup(ctx, OperatorOptions{}, kubeClient, "team", ref, nil); err != nil {
		t.Fatalf("claim should resolve without an expected Harbor connection: %v", err)
	}
	wrongConnection := connection
	wrongConnection.UID = "other-uid"
	if _, err := resolveUserGroup(ctx, OperatorOptions{}, kubeClient, "team", ref, &wrongConnection); err == nil {
		t.Fatal("claim bound to another Harbor connection was accepted")
	}

	unready := readyClaim.DeepCopy()
	unready.Status.HarborGroupID = 0
	unready.Status.Conditions = nil
	kubeClient = newAdvancedUnitClient(t, unready)
	if _, err := resolveUserGroup(ctx, OperatorOptions{}, kubeClient, "team", ref, nil); err == nil {
		t.Fatal("claim without a Harbor ID and Ready condition was accepted")
	}

	allowCrossNamespace := false
	kubeClient = newAdvancedUnitClient(t, readyClaim)
	if _, err := resolveUserGroup(ctx, OperatorOptions{allowCrossNamespaceReferences: &allowCrossNamespace}, kubeClient, "consumer", harborv1alpha1.UserGroupClaimReference{Name: "developers", Namespace: "team"}, nil); err == nil {
		t.Fatal("cross-namespace claim reference was accepted while disabled")
	}
}

type clientOnlyManager struct {
	manager.Manager
	kubeClient client.Client
}

func (m clientOnlyManager) GetClient() client.Client { return m.kubeClient }
