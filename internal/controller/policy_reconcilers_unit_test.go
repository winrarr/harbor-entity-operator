package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

//nolint:gocyclo // The mock verifies all observable transitions through one reconciliation lifecycle.
func TestRetentionPolicyReconcileCreateUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.RetentionPolicy
	var created, updated, deleted int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2.0/retentions":
			var policy harborclient.RetentionPolicy
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				t.Errorf("decode retention create: %v", err)
			}
			created++
			policy.ID = 17
			remote = &policy
			w.Header().Set("Location", "/api/v2.0/retentions/17")
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2.0/retentions/17":
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v2.0/retentions/17":
			var policy harborclient.RetentionPolicy
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				t.Errorf("decode retention update: %v", err)
			}
			updated++
			policy.ID = 17
			remote = &policy
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2.0/retentions/17":
			deleted++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected retention request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	policy := &harborv1alpha1.RetentionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "retention", Namespace: testNamespace},
		Spec: harborv1alpha1.RetentionPolicySpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			Rules: []harborv1alpha1.RetentionRule{{
				Action:       "retain",
				Template:     "latestPushedK",
				Params:       map[string]apiextensionsv1.JSON{"latestPushedK": {Raw: []byte(`{"value":3}`)}},
				TagSelectors: []harborv1alpha1.RetentionSelector{{Kind: "doublestar", Decoration: "matches", Pattern: "**"}},
			}},
			Trigger: &harborv1alpha1.RetentionTrigger{Kind: "Schedule", Settings: map[string]apiextensionsv1.JSON{"cron": {Raw: []byte(`"0 0 * * *"`)}}},
			Scope:   &harborv1alpha1.RetentionScope{Level: "project", Ref: 42},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, policy)
	reconciler := &RetentionPolicyReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: policy.Namespace, Name: policy.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create retention policy: %v", err)
	}
	var stored harborv1alpha1.RetentionPolicy
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get retention policy after create: %v", err)
	}
	if created != 1 || stored.Status.HarborRetentionID != 17 || remote == nil || remote.Algorithm != "or" || remote.Scope.Ref != 42 {
		t.Fatalf("retention create count/status/remote = %d/%d/%#v", created, stored.Status.HarborRetentionID, remote)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if len(remote.Rules) != 1 || remote.Rules[0].Priority != 1 || remote.Rules[0].Params.(map[string]any)["latestPushedK"] != float64(3) {
		t.Fatalf("retention rule conversion = %#v", remote.Rules)
	}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op retention reconcile: %v", err)
	}
	if created != 1 || updated != 0 {
		t.Fatalf("no-op retention request counts: creates=%d updates=%d", created, updated)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload retention policy: %v", err)
	}
	stored.Spec.Rules[0].Template = "latestPushedN"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update retention policy spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update retention policy: %v", err)
	}
	if updated != 1 || remote.Rules[0].Template != "latestPushedN" {
		t.Fatalf("updated retention request/remote = %d/%#v", updated, remote)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload retention before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete retention policy: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize retention policy deletion: %v", err)
	}
	if deleted != 1 || remote != nil {
		t.Fatalf("retention deletion count/remote = %d/%#v", deleted, remote)
	}
}

//nolint:gocyclo // The mock verifies all observable transitions through one reconciliation lifecycle.
func TestImmutableTagRuleReconcileCreateUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.ImmutableRule
	var created, updated, deleted int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2.0/projects/42/immutabletagrules" && r.Method == http.MethodPost:
			var rule harborclient.ImmutableRule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
				t.Errorf("decode immutable rule create: %v", err)
			}
			created++
			rule.ID = 9
			remote = &rule
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/api/v2.0/projects/42/immutabletagrules" && r.Method == http.MethodGet:
			if remote == nil {
				writeJSON(t, w, []harborclient.ImmutableRule{})
				return
			}
			writeJSON(t, w, []harborclient.ImmutableRule{*remote})
		case r.URL.Path == "/api/v2.0/projects/42/immutabletagrules/9" && r.Method == http.MethodPut:
			var rule harborclient.ImmutableRule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
				t.Errorf("decode immutable rule update: %v", err)
			}
			updated++
			rule.ID = 9
			remote = &rule
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v2.0/projects/42/immutabletagrules/9" && r.Method == http.MethodDelete:
			deleted++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected immutable rule request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	rule := &harborv1alpha1.ImmutableTagRule{
		ObjectMeta: metav1.ObjectMeta{Name: "release", Namespace: testNamespace},
		Spec: harborv1alpha1.ImmutableTagRuleSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			ProjectRef:     &harborv1alpha1.ProjectReference{Name: project.Name},
			Action:         "immutable",
			Template:       "repoMatches",
			TagSelectors:   []harborv1alpha1.ImmutableSelector{{Kind: "doublestar", Decoration: "matches", Pattern: "release-*"}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, project, rule)
	reconciler := &ImmutableTagRuleReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: rule.Namespace, Name: rule.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create immutable tag rule: %v", err)
	}
	var stored harborv1alpha1.ImmutableTagRule
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get immutable rule after create: %v", err)
	}
	if created != 1 || stored.Status.HarborImmutableRuleID != 9 || remote == nil || remote.Priority != rule.Spec.Priority {
		t.Fatalf("immutable rule create count/status/remote = %d/%d/%#v", created, stored.Status.HarborImmutableRuleID, remote)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op immutable rule reconcile: %v", err)
	}
	if created != 1 || updated != 0 {
		t.Fatalf("no-op immutable rule request counts: creates=%d updates=%d", created, updated)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload immutable rule: %v", err)
	}
	stored.Spec.Disabled = true
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update immutable rule spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update immutable rule: %v", err)
	}
	if updated != 1 || !remote.Disabled {
		t.Fatalf("updated immutable rule count/remote = %d/%#v", updated, remote)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload immutable rule before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete immutable rule: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize immutable rule deletion: %v", err)
	}
	if deleted != 1 || remote != nil {
		t.Fatalf("immutable rule deletion count/remote = %d/%#v", deleted, remote)
	}
}

//nolint:gocyclo // The mock verifies all observable transitions through one reconciliation lifecycle.
func TestReplicationPolicyReconcileCreateUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.ReplicationPolicy
	var created, updated, deleted int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2.0/replication/policies" && r.Method == http.MethodPost:
			var policy harborclient.ReplicationPolicy
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				t.Errorf("decode replication policy create: %v", err)
			}
			created++
			policy.ID = 31
			remote = &policy
			w.Header().Set("Location", "/api/v2.0/replication/policies/31")
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/api/v2.0/replication/policies/31" && r.Method == http.MethodGet:
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.URL.Path == "/api/v2.0/replication/policies/31" && r.Method == http.MethodPut:
			var policy harborclient.ReplicationPolicy
			if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
				t.Errorf("decode replication policy update: %v", err)
			}
			updated++
			policy.ID = 31
			remote = &policy
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v2.0/replication/policies/31" && r.Method == http.MethodDelete:
			deleted++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected replication policy request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	source := &harborv1alpha1.Registry{ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: testNamespace}, Status: harborv1alpha1.RegistryStatus{HarborRegistryID: 11}}
	destination := &harborv1alpha1.Registry{ObjectMeta: metav1.ObjectMeta{Name: "destination", Namespace: testNamespace}, Status: harborv1alpha1.RegistryStatus{HarborRegistryID: 22}}
	policy := &harborv1alpha1.ReplicationPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mirror", Namespace: testNamespace},
		Spec: harborv1alpha1.ReplicationPolicySpec{
			HarborSpecBase:         harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			SourceRegistryRef:      &harborv1alpha1.RegistryReference{Name: source.Name},
			DestinationRegistryRef: &harborv1alpha1.RegistryReference{Name: destination.Name},
			Description:            "mirror releases",
			ReplicateDeletion:      boolPointer(true),
			Trigger:                &harborv1alpha1.ReplicationTriggerSpec{Type: "manual"},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, source, destination, policy)
	reconciler := &ReplicationPolicyReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: policy.Namespace, Name: policy.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create replication policy: %v", err)
	}
	var stored harborv1alpha1.ReplicationPolicy
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get replication policy after create: %v", err)
	}
	if created != 1 || stored.Status.HarborReplicationPolicyID != 31 || remote == nil || remote.SrcRegistry.ID != 11 || remote.DestRegistry.ID != 22 || !*remote.ReplicateDeletion {
		t.Fatalf("replication policy create count/status/remote = %d/%d/%#v", created, stored.Status.HarborReplicationPolicyID, remote)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op replication policy reconcile: %v", err)
	}
	if created != 1 || updated != 0 {
		t.Fatalf("no-op replication request counts: creates=%d updates=%d", created, updated)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload replication policy: %v", err)
	}
	stored.Spec.Description = "updated mirror description"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update replication policy spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update replication policy: %v", err)
	}
	if updated != 1 || remote.Description != "updated mirror description" {
		t.Fatalf("updated replication request/remote = %d/%#v", updated, remote)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload replication policy before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete replication policy: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize replication policy deletion: %v", err)
	}
	if deleted != 1 || remote != nil {
		t.Fatalf("replication policy deletion count/remote = %d/%#v", deleted, remote)
	}
}

func boolPointer(value bool) *bool { return &value }
