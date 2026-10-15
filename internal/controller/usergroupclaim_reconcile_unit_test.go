package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

func TestUserGroupClaimRegistersGroupAndProtectsActiveMemberships(t *testing.T) {
	ctx := context.Background()
	var remote []harborclient.UserGroup
	var creates, deletes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2.0/usergroups" && r.Method == http.MethodGet:
			writeJSON(t, w, remote)
		case r.URL.Path == "/api/v2.0/usergroups" && r.Method == http.MethodPost:
			var desired harborclient.UserGroup
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode UserGroup create: %v", err)
			}
			creates++
			desired.ID = 8
			remote = []harborclient.UserGroup{desired}
			w.Header().Set("Location", "/api/v2.0/usergroups/8")
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/api/v2.0/usergroups/8" && r.Method == http.MethodDelete:
			deletes++
			t.Errorf("deleting a UserGroupClaim must not delete the shared Harbor group")
			http.Error(w, "unexpected delete", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected UserGroup request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	claim := &harborv1alpha1.UserGroupClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "developers", Namespace: testNamespace},
		Spec: harborv1alpha1.UserGroupClaimSpec{
			HarborClaimSpecBase: harborv1alpha1.HarborClaimSpecBase{HarborConnectionRef: clusterHarborRef()},
			GroupName:           "team:developers",
			GroupType:           3,
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, claim)
	reconciler := &UserGroupClaimReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: claim.Namespace, Name: claim.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("register external group claim: %v", err)
	}
	var stored harborv1alpha1.UserGroupClaim
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get claim after registration: %v", err)
	}
	if creates != 1 || stored.Status.HarborGroupID != 8 || len(remote) != 1 || remote[0].GroupName != claim.Spec.GroupName || remote[0].GroupType != 3 {
		t.Fatalf("claim registration calls/status/group = %d/%#v/%#v", creates, stored.Status, remote)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile already registered group claim: %v", err)
	}
	if creates != 1 {
		t.Fatalf("existing matching UserGroup was recreated %d times", creates-1)
	}

	member := &harborv1alpha1.Member{
		ObjectMeta: metav1.ObjectMeta{Name: "developers-in-demo", Namespace: testNamespace},
		Spec: harborv1alpha1.MemberSpec{
			ProjectRef:  harborv1alpha1.ProjectReference{Name: "demo"},
			MemberGroup: &harborv1alpha1.MemberGroup{GroupClaimRef: harborv1alpha1.UserGroupClaimReference{Name: claim.Name}},
		},
	}
	if err := kubeClient.Create(ctx, member); err != nil {
		t.Fatalf("create Member reference: %v", err)
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload claim before blocked deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete referenced claim: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err == nil {
		t.Fatal("claim deletion succeeded while a Member still referenced it")
	}
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get claim after blocked deletion: %v", err)
	}
	if stored.DeletionTimestamp.IsZero() || len(stored.Finalizers) == 0 || deletes != 0 {
		t.Fatalf("blocked deletion state: timestamp=%v finalizers=%v Harbor deletions=%d", stored.DeletionTimestamp, stored.Finalizers, deletes)
	}

	if err := kubeClient.Delete(ctx, member); err != nil {
		t.Fatalf("delete Member reference: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finish claim deletion after Member removal: %v", err)
	}
	if deletes != 0 {
		t.Fatalf("claim deletion removed the non-owned Harbor group %d times", deletes)
	}
}
