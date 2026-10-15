package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

//nolint:gocyclo // The mock verifies all observable transitions through one reconciliation lifecycle.
func TestHarborRoleReconcileCreateUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.HarborRole
	var creates, updates, deletes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2.0/roles" && r.Method == http.MethodGet:
			if remote == nil {
				writeJSON(t, w, []harborclient.HarborRole{})
				return
			}
			writeJSON(t, w, []harborclient.HarborRole{*remote})
		case r.URL.Path == "/api/v2.0/roles" && r.Method == http.MethodPost:
			var desired harborclient.HarborRole
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode Harbor role create: %v", err)
			}
			creates++
			desired.ID = 70
			remote = &desired
			writeJSON(t, w, map[string]int{"id": 70})
		case r.URL.Path == "/api/v2.0/roles/70" && r.Method == http.MethodGet:
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.URL.Path == "/api/v2.0/roles/70" && r.Method == http.MethodPut:
			var desired harborclient.HarborRole
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode Harbor role update: %v", err)
			}
			updates++
			desired.ID = 70
			remote = &desired
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v2.0/roles/70" && r.Method == http.MethodDelete:
			deletes++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected Harbor role request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	role := &harborv1alpha1.HarborRole{
		ObjectMeta: metav1.ObjectMeta{Name: "release-auditor"},
		Spec: harborv1alpha1.HarborRoleSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			Description:    "read release artifacts",
			Permissions: []harborv1alpha1.HarborRolePermission{{
				Kind: "project", Namespace: "*",
				Access: []harborv1alpha1.HarborRoleAccess{{Resource: "artifact", Action: "read", Effect: "allow"}},
			}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, role)
	reconciler := NewHarborRoleReconciler(kubeClient, nil, OperatorOptions{})
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: role.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create Harbor role: %v", err)
	}
	var stored harborv1alpha1.HarborRole
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get Harbor role after create: %v", err)
	}
	if creates != 1 || stored.Status.HarborRoleID != 70 || remote == nil || remote.Description != role.Spec.Description || len(remote.Permissions) != 1 {
		t.Fatalf("Harbor role create count/status/remote = %d/%#v/%#v", creates, stored.Status, remote)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op Harbor role reconcile: %v", err)
	}
	if creates != 1 || updates != 0 {
		t.Fatalf("no-op Harbor role request counts: creates=%d updates=%d", creates, updates)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload Harbor role: %v", err)
	}
	stored.Spec.Description = "read release artifacts and metadata"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update Harbor role spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update Harbor role: %v", err)
	}
	if updates != 1 || remote.Description != stored.Spec.Description {
		t.Fatalf("Harbor role update count/remote = %d/%#v", updates, remote)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload Harbor role before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete Harbor role CR: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize Harbor role deletion: %v", err)
	}
	if deletes != 1 || remote != nil {
		t.Fatalf("Harbor role delete count/remote = %d/%#v", deletes, remote)
	}
}

func TestHarborRoleReconcileRejectsBuiltInAdoption(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2.0/roles" {
			t.Errorf("unexpected Harbor role request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
			return
		}
		writeJSON(t, w, []harborclient.HarborRole{{ID: 1, Name: "system-admin", IsBuiltin: true}})
	}))
	defer server.Close()
	connection, secret := clusterHarborTestObjects(server.URL)
	role := &harborv1alpha1.HarborRole{
		ObjectMeta: metav1.ObjectMeta{Name: "system-admin"},
		Spec: harborv1alpha1.HarborRoleSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef()},
			CreationPolicy: harborv1alpha1.CreationPolicyCreateOrAdopt,
			Permissions:    []harborv1alpha1.HarborRolePermission{{Kind: "project", Access: []harborv1alpha1.HarborRoleAccess{{Resource: "artifact", Action: "read", Effect: "allow"}}}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, role)
	reconciler := NewHarborRoleReconciler(kubeClient, nil, OperatorOptions{})
	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: role.Name}})
	if err == nil || !strings.Contains(err.Error(), "built-in Harbor harborrole") {
		t.Fatalf("built-in role adoption error = %v", err)
	}
	var stored harborv1alpha1.HarborRole
	if getErr := kubeClient.Get(ctx, types.NamespacedName{Name: role.Name}, &stored); getErr != nil {
		t.Fatalf("get built-in role failure status: %v", getErr)
	}
	condition := meta.FindStatusCondition(stored.Status.Conditions, ConditionReady)
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("built-in role Ready condition = %#v", condition)
	}
}
