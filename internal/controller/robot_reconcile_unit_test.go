package controller

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	harborv1alpha1 "github.com/winrarr/harbor-entity-operator/api/v1alpha1"
	"github.com/winrarr/harbor-entity-operator/internal/harborclient"
)

//nolint:gocyclo // The mock verifies credentials, generated Secrets, drift, and finalization in one lifecycle.
func TestRobotReconcileCreatesUpdatesRotatedSecretsAndFinalizes(t *testing.T) {
	ctx := context.Background()
	futureExpiry := int(time.Now().Add(24 * time.Hour).Unix())
	var remote *harborclient.Robot
	var creates, updates, rotations, deletions int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2.0/robots" && r.Method == http.MethodPost:
			var desired harborclient.RobotCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode robot create: %v", err)
			}
			creates++
			remote = &harborclient.Robot{
				ID: 21, Name: "robot$release", Description: desired.Description, Level: desired.Level,
				Disable: desired.Disable != nil && *desired.Disable, Duration: desired.Duration,
				Permissions: desired.Permissions, ExpiresAt: futureExpiry,
			}
			writeJSON(t, w, harborclient.RobotCreated{ID: remote.ID, Name: remote.Name, Secret: "initial-token", ExpiresAt: remote.ExpiresAt})
		case r.URL.Path == "/api/v2.0/robots/21" && r.Method == http.MethodGet:
			if remote == nil {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, remote)
		case r.URL.Path == "/api/v2.0/robots/21" && r.Method == http.MethodPut:
			var desired harborclient.Robot
			if err := json.NewDecoder(r.Body).Decode(&desired); err != nil {
				t.Errorf("decode robot update: %v", err)
			}
			updates++
			remote.Description = desired.Description
			remote.Disable = desired.Disable
			remote.Duration = desired.Duration
			remote.Permissions = desired.Permissions
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v2.0/robots/21" && r.Method == http.MethodPatch:
			rotations++
			remote.ExpiresAt = futureExpiry
			writeJSON(t, w, harborclient.RobotSec{Secret: "rotated-token"})
		case r.URL.Path == "/api/v2.0/robots/21" && r.Method == http.MethodDelete:
			deletions++
			remote = nil
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected robot request %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	connection, connectionSecret := clusterHarborTestObjects(server.URL)
	project := &harborv1alpha1.Project{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: testNamespace}, Status: harborv1alpha1.ProjectStatus{HarborProjectID: 42}}
	robot := &harborv1alpha1.Robot{
		ObjectMeta: metav1.ObjectMeta{Name: "release", Namespace: testNamespace},
		Spec: harborv1alpha1.RobotSpec{
			HarborSpecBase: harborv1alpha1.HarborSpecBase{HarborConnectionRef: clusterHarborRef(), DeletionPolicy: harborv1alpha1.DeletionPolicyDelete},
			Level:          "project",
			Description:    "release automation",
			Duration:       -1,
			SecretRef:      &harborv1alpha1.SecretReference{Name: "release-credentials", Key: "token"},
			ImagePullSecretRef: &harborv1alpha1.SecretObjectReference{
				Name: "release-pull-secret",
			},
			Permissions: []harborv1alpha1.RobotPermission{{
				Kind:       "project",
				ProjectRef: &harborv1alpha1.ProjectReference{Name: project.Name},
				Access:     []harborv1alpha1.RobotAccess{{Resource: harborv1alpha1.RobotResourceRepository, Action: harborv1alpha1.RobotActionPush}},
			}},
		},
	}
	kubeClient := newAdvancedUnitClient(t, connection, connectionSecret, project, robot)
	reconciler := &RobotReconciler{Client: kubeClient}
	req := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: robot.Namespace, Name: robot.Name}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("create robot: %v", err)
	}
	var stored harborv1alpha1.Robot
	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("get robot after create: %v", err)
	}
	if creates != 1 || stored.Status.HarborRobotID != 21 || stored.Status.Username != "robot$release" || stored.Status.LastRotatedAt == nil {
		t.Fatalf("robot create calls/status = %d/%#v", creates, stored.Status)
	}
	assertReadyCondition(t, stored.Status.Conditions)
	if remote == nil || len(remote.Permissions) != 1 || remote.Permissions[0].Namespace != "demo" {
		t.Fatalf("robot permission payload = %#v", remote)
	}

	var tokenSecret corev1.Secret
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: robot.Namespace, Name: "release-credentials"}, &tokenSecret); err != nil {
		t.Fatalf("get robot token Secret: %v", err)
	}
	if string(tokenSecret.Data["token"]) != "initial-token" || string(tokenSecret.Data[robotUsernameSecretKey]) != "robot$release" || !secretOwnedBy(&tokenSecret, &stored, "Robot") {
		t.Fatalf("robot token Secret data/owner = %#v / %#v", tokenSecret.Data, tokenSecret.OwnerReferences)
	}
	var pullSecret corev1.Secret
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: robot.Namespace, Name: "release-pull-secret"}, &pullSecret); err != nil {
		t.Fatalf("get robot image pull Secret: %v", err)
	}
	var dockerCfg dockerConfig
	if err := json.Unmarshal(pullSecret.Data[corev1.DockerConfigJsonKey], &dockerCfg); err != nil {
		t.Fatalf("parse image pull Secret: %v", err)
	}
	registryAuth := dockerCfg.Auths[server.Listener.Addr().String()]
	if registryAuth.Username != "robot$release" || registryAuth.Password != "initial-token" || registryAuth.Auth != base64.StdEncoding.EncodeToString([]byte("robot$release:initial-token")) {
		t.Fatalf("image pull credentials = %#v", registryAuth)
	}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("no-op robot reconcile: %v", err)
	}
	if creates != 1 || updates != 0 || rotations != 0 {
		t.Fatalf("no-op robot API calls: creates=%d updates=%d rotations=%d", creates, updates, rotations)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload robot before update: %v", err)
	}
	stored.Spec.Description = "updated release automation"
	if err := kubeClient.Update(ctx, &stored); err != nil {
		t.Fatalf("update robot spec: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("update robot: %v", err)
	}
	if updates != 1 || remote.Description != "updated release automation" {
		t.Fatalf("robot update calls/remote description = %d/%q", updates, remote.Description)
	}

	remote.ExpiresAt = int(time.Now().Add(-time.Hour).Unix())
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("rotate expired robot credentials: %v", err)
	}
	if rotations != 1 {
		t.Fatalf("expired robot secret refreshes = %d, want 1", rotations)
	}
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: robot.Namespace, Name: "release-credentials"}, &tokenSecret); err != nil {
		t.Fatalf("get rotated robot token Secret: %v", err)
	}
	if string(tokenSecret.Data["token"]) != "rotated-token" {
		t.Fatalf("rotated robot token = %q", tokenSecret.Data["token"])
	}
	if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: robot.Namespace, Name: "release-pull-secret"}, &pullSecret); err != nil {
		t.Fatalf("get rotated image pull Secret: %v", err)
	}
	if err := json.Unmarshal(pullSecret.Data[corev1.DockerConfigJsonKey], &dockerCfg); err != nil {
		t.Fatalf("parse rotated image pull Secret: %v", err)
	}
	if got := dockerCfg.Auths[server.Listener.Addr().String()].Password; got != "rotated-token" {
		t.Fatalf("rotated image pull token = %q", got)
	}

	if err := kubeClient.Get(ctx, req.NamespacedName, &stored); err != nil {
		t.Fatalf("reload robot before deletion: %v", err)
	}
	if err := kubeClient.Delete(ctx, &stored); err != nil {
		t.Fatalf("delete robot CR: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatalf("finalize robot deletion: %v", err)
	}
	if deletions != 1 || remote != nil {
		t.Fatalf("robot deletion count/remote = %d/%#v", deletions, remote)
	}
}
