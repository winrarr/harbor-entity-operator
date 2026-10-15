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

//nolint:gocyclo // The mock verifies all observable transitions through one reconciliation lifecycle.
func TestProjectReconcileCreatesUpdatesAndDeletesHarborProject(t *testing.T) {
	ctx := context.Background()
	var remote *harborclient.Project
	var created, updated, deleted int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2.0/projects" && r.Method == http.MethodPost:
			var desired harborclient.CreateProjectRequest
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode project create: %v", err)
			}
			created++
			remote = projectFromCreateRequest(42, desired)
			w.Header().Set("Location", "/api/v2.0/projects/42")
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/api/v2.0/projects/42" && r.Method == http.MethodGet:
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.URL.Path == "/api/v2.0/projects/42" && r.Method == http.MethodPut:
			var desired harborclient.CreateProjectRequest
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode project update: %v", err)
			}
			updated++
			remote = projectFromCreateRequest(42, desired)
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v2.0/projects/42" && r.Method == http.MethodDelete:
			deleted++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected project request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	severity := "high"
	project := &harborv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace},
		Spec: harborv1alpha1.ProjectSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			Public:         true,
			Owner:          "admin",
			Metadata:       &harborv1alpha1.ProjectMetadata{Severity: &severity},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, project)
	reconciler := &ProjectReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: project.Namespace, Name: project.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create project: %v", err)
	}
	var stored harborv1alpha1.Project
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get project after create: %v", err)
	}
	if created != 1 || stored.Status.HarborProjectID != 42 || remote == nil || remote.Name != "demo" || remote.Metadata.Public != "true" || remote.Metadata.Severity == nil || *remote.Metadata.Severity != severity {
		t.Fatalf("project create count/status/remote = %d/%d/%#v", created, stored.Status.HarborProjectID, remote)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op project reconcile: %v", err)
	}
	if created != 1 || updated != 0 {
		t.Fatalf("no-op project request counts: creates=%d updates=%d", created, updated)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	stored.Spec.Public = false
	critical := "critical"
	stored.Spec.Metadata.Severity = &critical
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update project spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update project: %v", err)
	}
	if updated != 1 || remote.Metadata.Public != "false" || remote.Metadata.Severity == nil || *remote.Metadata.Severity != critical {
		t.Fatalf("project update count/remote = %d/%#v", updated, remote)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload project before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete project CR: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize project deletion: %v", err)
	}
	if deleted != 1 || remote != nil {
		t.Fatalf("project delete count/remote = %d/%#v", deleted, remote)
	}
}

func projectFromCreateRequest(id int, request harborclient.CreateProjectRequest) *harborclient.Project {
	return &harborclient.Project{
		ProjectID: id,
		Name:      request.ProjectName,
		OwnerName: request.Owner,
		Metadata:  request.Metadata,
		RegistryID: func() int {
			if request.RegistryID == nil {
				return 0
			}
			return *request.RegistryID
		}(),
	}
}

//nolint:gocyclo // The mock verifies all observable transitions through one reconciliation lifecycle.
func TestMemberReconcileCreatesUpdatesAndRemovesUserMembership(t *testing.T) {
	ctx := context.Background()
	var remote []harborclient.ProjectMember
	var created, updated, deleted int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2.0/projects/42/members" && r.Method == http.MethodGet {
			writeJSON(t, w, remote)
			return
		}
		if r.URL.Path == "/api/v2.0/projects/42/members" && r.Method == http.MethodPost {
			var desired harborclient.CreateMemberRequest
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode project member create: %v", err)
			}
			created++
			remote = []harborclient.ProjectMember{{ID: 77, ProjectID: 42, EntityType: "u", EntityName: desired.MemberUser.Username, RoleID: desired.RoleID}}
			w.Header().Set("Location", "/api/v2.0/projects/42/members/77")
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.URL.Path == "/api/v2.0/projects/42/members/77" && r.Method == http.MethodPut {
			var body struct {
				RoleID int `json:"role_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode project member role update: %v", err)
			}
			updated++
			remote[0].RoleID = body.RoleID
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/v2.0/projects/42/members/77" && r.Method == http.MethodDelete {
			deleted++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
			return
		}
		t.Errorf("unexpected project member request %s %s", r.Method, r.URL)
		http.NotFound(w, r)
	}))
	defer server.Close()

	connection, secret := clusterHarborTestObjects(server.URL)
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	user := &harborv1alpha1.User{ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: testNamespace}}
	member := &harborv1alpha1.Member{
		ObjectMeta: metav1.ObjectMeta{Name: "alice-developer", Namespace: testNamespace},
		Spec: harborv1alpha1.MemberSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			ProjectRef:     harborv1alpha1.ProjectReference{Name: project.Name},
			Role:           "developer",
			MemberUser:     &harborv1alpha1.MemberUser{UserRef: harborv1alpha1.UserReference{Name: user.Name}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, secret, project, user, member)
	reconciler := &MemberReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: member.Namespace, Name: member.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create project member: %v", err)
	}
	var stored harborv1alpha1.Member
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get member after create: %v", err)
	}
	if created != 1 || stored.Status.HarborProjectID != 42 || stored.Status.HarborMemberID != 77 || len(remote) != 1 || remote[0].EntityName != "alice" || remote[0].RoleID != 2 {
		t.Fatalf("member create count/status/remote = %d/%#v/%#v", created, stored.Status, remote)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op project member reconcile: %v", err)
	}
	if created != 1 || updated != 0 {
		t.Fatalf("no-op member request counts: creates=%d updates=%d", created, updated)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload member: %v", err)
	}
	stored.Spec.Role = "maintainer"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update member spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update project member role: %v", err)
	}
	if updated != 1 || remote[0].RoleID != 4 {
		t.Fatalf("member role update count/remote = %d/%#v", updated, remote)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload member before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete member CR: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize member deletion: %v", err)
	}
	if deleted != 1 || len(remote) != 0 {
		t.Fatalf("member delete count/remote = %d/%#v", deleted, remote)
	}
}
